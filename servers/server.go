package servers

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gngram/spire_admin/logger"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/spiffe/go-spiffe/v2/spiffegrpc/grpccredentials"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	bundlev1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/bundle/v1"
	localauthorityv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/localauthority/v1"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

// ServerHealthStatus represents the connection and operational state of a SPIRE server.
type ServerHealthStatus int

const (
	Connecting ServerHealthStatus = iota
	Online
	Offline
)

// ServerContext contains the shared state, connection handle, caches, and synchronization primitives
// for a single SPIRE server instance.
type ServerContext struct {
	mu           sync.RWMutex
	Nickname     string
	Address      string
	Port         string
	Domain       string
	AgentSocket  string
	HealthStatus ServerHealthStatus
	LastUpdated  time.Time

	ctx            context.Context
	cancel         context.CancelFunc
	OnHealthChange func(ServerHealthStatus)

	Agents           []Agent
	Entries          []Entry
	Bundles          []*types.Bundle
	FederatedServers []FederatedServer

	conn   *grpc.ClientConn
	source *workloadapi.X509Source

	server *SpireServer
}

// Connect establishes the gRPC client connection using mTLS (via agent socket) or insecure transport.
func (c *ServerContext) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		logger.Info("Already connected to server")
		return nil
	}
	logger.Info("Trying to connect to server")

	target := fmt.Sprintf("%s:%s", c.Address, c.Port)

	var dialCreds credentials.TransportCredentials

	if c.AgentSocket != "" {
		source, err := workloadapi.NewX509Source(
			ctx,
			workloadapi.WithClientOptions(
				workloadapi.WithAddr("unix://"+c.AgentSocket),
			),
		)
		if err != nil {
			err = fmt.Errorf("failed to create X509 source for agent: %s: %w", c.AgentSocket, err)
			logger.Error("X509 source creation failed", err)
			return err
		}

		c.source = source

		dialCreds = grpccredentials.MTLSClientCredentials(
			source,
			source,
			tlsconfig.AuthorizeAny(),
		)
	} else {
		logger.Info("Insecure connection")
		dialCreds = insecure.NewCredentials()
	}

	conn, err := grpc.NewClient(
		target,
		grpc.WithTransportCredentials(dialCreds),
	)
	if err != nil {
		if c.source != nil {
			c.source.Close()
			c.source = nil
		}

		logger.Error("failed to create gRPC client: %w", err)
		return err
	}

	c.conn = conn
	return nil
}

// Close gracefully terminates background contexts and closes gRPC connections and SVID sources.
func (c *ServerContext) Close() error {
	if c.cancel != nil {
		c.cancel()
	}

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	if c.source != nil {
		c.source.Close()
		c.source = nil
	}

	return nil
}

// SpireServer represents a connected SPIRE server and acts as a coordinator facade for category services.
type SpireServer struct {
	*ServerContext

	agent             *AgentService
	bundle            *BundleService
	entry             *EntryService
	federation        *FederationService
	healthcheck       *HealthCheckService
	localauthority    *LocalAuthorityService
	logger            *LoggerService
	token             *TokenService
	upstreamauthority *UpstreamAuthorityService
	validate          *ValidateService
	x509              *X509Service
}

// NewServerContext initializes a standalone ServerContext.
func NewServerContext(nickname, address, port, agentSocket string) *ServerContext {
	ctx, cancel := context.WithCancel(context.Background())
	return &ServerContext{
		Nickname:     nickname,
		Address:      address,
		Port:         port,
		AgentSocket:  agentSocket,
		HealthStatus: Connecting,
		LastUpdated:  time.Now(),
		Domain:       "Unknown",
		ctx:          ctx,
		cancel:       cancel,
	}
}

// NewSpireServer initializes a SpireServer, binds its category services, and asynchronously fetches its data.
func NewSpireServer(nickname, address, port, agentSocket string) (*SpireServer, error) {
	logger.Info("New Spire Server [%s]: %s:%s, socket:%s", nickname, address, port, agentSocket)
	if net.ParseIP(address) == nil {
		err := fmt.Errorf("invalid address: %s is not a valid IP address", address)
		logger.Error("Invalid IP address", err)
		return nil, err
	}
	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		err = fmt.Errorf("invalid port: %s", port)
		logger.Error("Invalid port", err)
		return nil, err
	}

	sCtx := NewServerContext(nickname, address, port, agentSocket)

	s := &SpireServer{
		ServerContext: sCtx,
	}
	sCtx.server = s

	// Initialize all category services
	s.initServices()

	// Load remote SPIRE data asynchronously so the UI doesn't freeze
	go func() {
		s.FetchInfo() // Initial fetch
		if err := s.RefreshCache(s.ctx); err != nil {
			logger.Error("RefreshCache failed", err)
		}
	}()

	return s, nil
}

func (s *SpireServer) initServices() {
	if s.ServerContext == nil {
		s.ServerContext = &ServerContext{}
	}
	s.agent = NewAgentService(s.ServerContext)
	s.bundle = NewBundleService(s.ServerContext)
	s.entry = NewEntryService(s.ServerContext)
	s.federation = NewFederationService(s.ServerContext)
	s.healthcheck = NewHealthCheckService(s.ServerContext)
	s.localauthority = NewLocalAuthorityService(s.ServerContext)
	s.logger = NewLoggerService(s.ServerContext)
	s.token = NewTokenService(s.ServerContext)
	s.upstreamauthority = NewUpstreamAuthorityService(s.ServerContext)
	s.validate = NewValidateService(s.ServerContext)
	s.x509 = NewX509Service(s.ServerContext)
}

// ════════════════════════════════════════════════════════
//  CATEGORY SERVICE ACCESSORS
// ════════════════════════════════════════════════════════

func (s *SpireServer) Agent() *AgentService {
	if s.agent == nil {
		s.agent = NewAgentService(s.ServerContext)
	}
	return s.agent
}

func (s *SpireServer) Bundle() *BundleService {
	if s.bundle == nil {
		s.bundle = NewBundleService(s.ServerContext)
	}
	return s.bundle
}

func (s *SpireServer) Entry() *EntryService {
	if s.entry == nil {
		s.entry = NewEntryService(s.ServerContext)
	}
	return s.entry
}

func (s *SpireServer) Federation() *FederationService {
	if s.federation == nil {
		s.federation = NewFederationService(s.ServerContext)
	}
	return s.federation
}

func (s *SpireServer) HealthCheck() *HealthCheckService {
	if s.healthcheck == nil {
		s.healthcheck = NewHealthCheckService(s.ServerContext)
	}
	return s.healthcheck
}

func (s *SpireServer) LocalAuthority() *LocalAuthorityService {
	if s.localauthority == nil {
		s.localauthority = NewLocalAuthorityService(s.ServerContext)
	}
	return s.localauthority
}

func (s *SpireServer) Logger() *LoggerService {
	if s.logger == nil {
		s.logger = NewLoggerService(s.ServerContext)
	}
	return s.logger
}

func (s *SpireServer) Token() *TokenService {
	if s.token == nil {
		s.token = NewTokenService(s.ServerContext)
	}
	return s.token
}

func (s *SpireServer) UpstreamAuthority() *UpstreamAuthorityService {
	if s.upstreamauthority == nil {
		s.upstreamauthority = NewUpstreamAuthorityService(s.ServerContext)
	}
	return s.upstreamauthority
}

func (s *SpireServer) Validate() *ValidateService {
	if s.validate == nil {
		s.validate = NewValidateService(s.ServerContext)
	}
	return s.validate
}

func (s *SpireServer) X509() *X509Service {
	if s.x509 == nil {
		s.x509 = NewX509Service(s.ServerContext)
	}
	return s.x509
}

// ════════════════════════════════════════════════════════
//  COMPATIBILITY DELEGATIONS (FRONTEND SAFETY)
// ════════════════════════════════════════════════════════

// Health & Refresh
func (s *SpireServer) FetchInfo() {
	s.HealthCheck().FetchInfo()
}

func (s *SpireServer) CheckHealth(ctx context.Context) (healthpb.HealthCheckResponse_ServingStatus, error) {
	return s.HealthCheck().CheckHealth(ctx)
}

func (s *SpireServer) GetCachedHealthStatus() ServerHealthStatus {
	return s.HealthCheck().GetCachedHealthStatus()
}

func (s *SpireServer) GetLastUpdated() time.Time {
	return s.HealthCheck().GetLastUpdated()
}

func (s *SpireServer) RefreshCache(ctx context.Context) error {
	return s.HealthCheck().RefreshCache(ctx)
}

// Agent
func (s *SpireServer) ListAgents(ctx context.Context, pull bool) ([]Agent, error) {
	return s.Agent().ListAgents(ctx, pull)
}

func (s *SpireServer) GetAgentInfo(ctx context.Context, spiffeID string) (Agent, error) {
	return s.Agent().GetAgentInfo(ctx, spiffeID)
}

func (s *SpireServer) BanAgent(ctx context.Context, spiffeID string) error {
	return s.Agent().BanAgent(ctx, spiffeID)
}

func (s *SpireServer) EvictAgent(ctx context.Context, spiffeID string) error {
	return s.Agent().EvictAgent(ctx, spiffeID)
}

func (s *SpireServer) PurgeExpiredAgents(ctx context.Context) error {
	return s.Agent().PurgeExpiredAgents(ctx)
}

// Token
func (s *SpireServer) CreateJoinToken(ctx context.Context, ttlSeconds int32) (*types.JoinToken, error) {
	return s.Token().CreateJoinToken(ctx, ttlSeconds)
}

// Bundle
func (s *SpireServer) AppendX509RootCA(ctx context.Context, trustDomain string, cert *x509.Certificate) error {
	return s.Bundle().AppendX509RootCA(ctx, trustDomain, cert)
}

func (s *SpireServer) GetBundle(ctx context.Context, trustDomain string) (*types.Bundle, error) {
	return s.Bundle().GetBundle(ctx, trustDomain)
}

func (s *SpireServer) DeleteFederatedBundle(ctx context.Context, trustDomain string, mode bundlev1.BatchDeleteFederatedBundleRequest_Mode) error {
	return s.Bundle().DeleteFederatedBundle(ctx, trustDomain, mode)
}

func (s *SpireServer) ListFederatedBundles(ctx context.Context, pull bool) ([]*types.Bundle, error) {
	return s.Bundle().ListFederatedBundles(ctx, pull)
}

func (s *SpireServer) SetFederatedBundle(ctx context.Context, bundle *types.Bundle) (*types.Bundle, error) {
	return s.Bundle().SetFederatedBundle(ctx, bundle)
}

// Entry
func (s *SpireServer) ListEntries(ctx context.Context, pull bool) ([]Entry, error) {
	return s.Entry().ListEntries(ctx, pull)
}

func (s *SpireServer) CountEntries(ctx context.Context) (int32, error) {
	return s.Entry().CountEntries(ctx)
}

func (s *SpireServer) CreateEntry(ctx context.Context, entry *types.Entry) (*types.Entry, error) {
	return s.Entry().CreateEntry(ctx, entry)
}

func (s *SpireServer) DeleteEntry(ctx context.Context, entryID string) error {
	return s.Entry().DeleteEntry(ctx, entryID)
}

func (s *SpireServer) GetEntry(ctx context.Context, entryID string) (*types.Entry, error) {
	return s.Entry().GetEntry(ctx, entryID)
}

func (s *SpireServer) UpdateEntry(ctx context.Context, entry *types.Entry) (*types.Entry, error) {
	return s.Entry().UpdateEntry(ctx, entry)
}

func (s *SpireServer) GetAgentsEntries() []Entry {
	return s.Entry().GetAgentsEntries()
}

func (s *SpireServer) GetWorkloadsEntries() []Entry {
	return s.Entry().GetWorkloadsEntries()
}

func (s *SpireServer) GetDownstreamsEntries() []Entry {
	return s.Entry().GetDownstreamsEntries()
}

// Federation
func (s *SpireServer) ListFederatedServers(ctx context.Context, pull bool) ([]FederatedServer, error) {
	return s.Federation().ListFederatedServers(ctx, pull)
}

func (s *SpireServer) CreateFederationRelationship(ctx context.Context, rel *types.FederationRelationship) (*types.FederationRelationship, error) {
	return s.Federation().CreateFederationRelationship(ctx, rel)
}

func (s *SpireServer) DeleteFederationRelationship(ctx context.Context, trustDomain string) error {
	return s.Federation().DeleteFederationRelationship(ctx, trustDomain)
}

func (s *SpireServer) ListFederationRelationships(ctx context.Context) ([]*types.FederationRelationship, error) {
	return s.Federation().ListFederationRelationships(ctx)
}

func (s *SpireServer) RefreshFederationBundle(ctx context.Context, trustDomain string) error {
	return s.Federation().RefreshFederationBundle(ctx, trustDomain)
}

func (s *SpireServer) GetFederationRelationship(ctx context.Context, trustDomain string) (*types.FederationRelationship, error) {
	return s.Federation().GetFederationRelationship(ctx, trustDomain)
}

func (s *SpireServer) UpdateFederationRelationship(ctx context.Context, rel *types.FederationRelationship, mask *types.FederationRelationshipMask) (*types.FederationRelationship, error) {
	return s.Federation().UpdateFederationRelationship(ctx, rel, mask)
}

// Local Authority
func (s *SpireServer) ShowLocalX509Authorities(ctx context.Context) (*localauthorityv1.GetX509AuthorityStateResponse, error) {
	return s.LocalAuthority().ShowLocalX509Authorities(ctx)
}

func (s *SpireServer) PrepareLocalX509Authority(ctx context.Context) (*localauthorityv1.AuthorityState, error) {
	return s.LocalAuthority().PrepareLocalX509Authority(ctx)
}

func (s *SpireServer) ActivateLocalX509Authority(ctx context.Context, authorityID string) (*localauthorityv1.AuthorityState, error) {
	return s.LocalAuthority().ActivateLocalX509Authority(ctx, authorityID)
}

func (s *SpireServer) TaintLocalX509Authority(ctx context.Context, authorityID string) (*localauthorityv1.AuthorityState, error) {
	return s.LocalAuthority().TaintLocalX509Authority(ctx, authorityID)
}

func (s *SpireServer) RevokeLocalX509Authority(ctx context.Context, authorityID string) (*localauthorityv1.AuthorityState, error) {
	return s.LocalAuthority().RevokeLocalX509Authority(ctx, authorityID)
}

// Upstream Authority
func (s *SpireServer) RevokeUpstreamX509Authority(ctx context.Context, subjectKeyID string) (string, error) {
	return s.UpstreamAuthority().RevokeUpstreamX509Authority(ctx, subjectKeyID)
}

func (s *SpireServer) TaintUpstreamX509Authority(ctx context.Context, subjectKeyID string) (string, error) {
	return s.UpstreamAuthority().TaintUpstreamX509Authority(ctx, subjectKeyID)
}

// ════════════════════════════════════════════════════════
//  HCL CONFIGURATION MANAGEMENT
// ════════════════════════════════════════════════════════

type AppConfig struct {
	Servers []ServerConfig `hcl:"server,block"`
}

type ServerConfig struct {
	Nickname string `hcl:"name,label"`
	Address  string `hcl:"address,optional"`
	Port     string `hcl:"port,optional"`
}

// SaveServersConfig saves the provided map of SpireServers to an HCL file.
func SaveServersConfig(path string, serverMap map[string]*SpireServer) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	appCfg := &AppConfig{}
	for nickname, s := range serverMap {
		s.mu.RLock()
		sc := ServerConfig{Nickname: nickname, Address: s.Address, Port: s.Port}
		s.mu.RUnlock()
		appCfg.Servers = append(appCfg.Servers, sc)
	}
	f := hclwrite.NewEmptyFile()
	gohcl.EncodeIntoBody(appCfg, f.Body())
	return os.WriteFile(path, f.Bytes(), 0o644)
}

// LoadServersConfig parses the list of ServerConfigs from an HCL file.
func LoadServersConfig(path string) (*AppConfig, error) {
	appCfg := &AppConfig{}
	parser := hclparse.NewParser()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	file, diags := parser.ParseHCLFile(path)
	if diags.HasErrors() {
		return nil, diags
	}
	gohcl.DecodeBody(file.Body, nil, appCfg)
	return appCfg, nil
}
