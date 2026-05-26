package servers

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gngram/spidar/logger"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"github.com/spiffe/go-spiffe/v2/spiffegrpc/grpccredentials"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SpireServer represents a connected SPIRE server and its aggregated state.
type SpireServer struct {
	mu               sync.RWMutex
	Address          string
	Port             string
	Domain           string
	AgentSocket  string
	HealthStatus     ServerHealthStatus
	LastUpdated      time.Time

	ctx              context.Context
	cancel           context.CancelFunc
	OnHealthChange   func(ServerHealthStatus)

	Agents           []Agent
	Workloads        []Workload
	Bundles          []*types.Bundle
	FederatedServers []FederatedServer

	conn   *grpc.ClientConn
	source *workloadapi.X509Source
}

type ServerHealthStatus int

const (
	Connecting ServerHealthStatus = iota
	Online
	Offline
)


// NewSpireServer initializes a SpireServer and asynchronously fetches its data.
func NewSpireServer(address, port, agentSocket string) (*SpireServer, error) {
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

	s := &SpireServer{
		Address:         address,
		Port:            port,
		AgentSocket:     agentSocket,
		HealthStatus:    Connecting,
		LastUpdated:     time.Now(),
		Domain:          "Unknown",
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Load the remote SPIRE data asynchronously so the UI doesn't freeze
	go func() {
		s.fetchInfo() // Initial fetch
		if err := s.RefreshCache(s.ctx); err != nil {
			logger.Error("RefreshCache failed", err)
		}
	}()
	
	return s, nil
}

func (s *SpireServer) fetchInfo() {
	ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
	defer cancel()
   
	if err := s.Connect(ctx); err != nil {
		logger.Error("Error connecting to server", err)
		s.mu.Lock()
		oldStatus := s.HealthStatus
		s.HealthStatus = Offline
		s.LastUpdated = time.Now()
		cb := s.OnHealthChange
		s.mu.Unlock()
		
		select {
		case <-s.ctx.Done():
			return
		default:
			if cb != nil && oldStatus != Offline {
				cb(Offline)
			}
		}
		return
	}
	if s.source != nil {
		if svid, err := s.source.GetX509SVID(); err == nil {
			s.mu.Lock()
			if s.Domain == "Unknown" || s.Domain == "" {
				s.Domain = svid.ID.TrustDomain().Name()
			}
			s.mu.Unlock()
		}
	}

	logger.Info("Domain: %s", s.Domain)
	
	logger.Info("Checking health..")
	client := healthpb.NewHealthClient(s.conn)
	_, err := client.Check(ctx, &healthpb.HealthCheckRequest{})

	s.mu.Lock()
	oldStatus := s.HealthStatus
	
	if err != nil && status.Code(err) != codes.Unimplemented {
		logger.Error("Error checking health", err)
		s.HealthStatus = Offline
	} else {
		s.HealthStatus = Online
	}

	s.LastUpdated = time.Now()
	newStatus := s.HealthStatus
	cb := s.OnHealthChange
	s.mu.Unlock()
	
	select {
	case <-s.ctx.Done():
		return
	default:
		if cb != nil && oldStatus != newStatus {
			cb(newStatus)
		}
	}
}

func parseSPIFFEID(id string) (*types.SPIFFEID, error) {
	if !strings.HasPrefix(id, "spiffe://") {
		err := fmt.Errorf("invalid SPIFFE ID format")
		logger.Error("Parse SPIFFE ID error", err)
		return nil, err
	}
	trimmed := strings.TrimPrefix(id, "spiffe://")
	parts := strings.SplitN(trimmed, "/", 2)
	td := parts[0]
	path := ""
	if len(parts) > 1 {
		path = "/" + parts[1]
	}
	return &types.SPIFFEID{
		TrustDomain: td,
		Path:        path,
	}, nil
}


func (s *SpireServer) Connect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		logger.Info("Already connected to server")
		return nil
	}
	logger.Info("Trying to connect to server")

	target := fmt.Sprintf("%s:%s", s.Address, s.Port)

	var dialCreds credentials.TransportCredentials

	if s.AgentSocket != "" {
		source, err := workloadapi.NewX509Source(
			ctx,
			workloadapi.WithClientOptions(
				workloadapi.WithAddr("unix://"+s.AgentSocket),
			),
		)
		if err != nil {
			err = fmt.Errorf("failed to create X509 source for agent: %s: %w", s.AgentSocket, err)
			logger.Error("X509 source creation failed", err)
			return err
		}

		s.source = source

		dialCreds = grpccredentials.MTLSClientCredentials(
			source,
			source,
			tlsconfig.AuthorizeAny(),
		)
	} else {
		logger.Info("Insecure connection")
		dialCreds = insecure.NewCredentials()
	}

	conn, err := grpc.DialContext(
		ctx,
		target,
		grpc.WithTransportCredentials(dialCreds),
	)
	if err != nil {
		if s.source != nil {
			s.source.Close()
			s.source = nil
		}
		err = fmt.Errorf("failed to dial: %w", err)
		logger.Error("Failed to dial", err)
		return err
	}

	s.conn = conn
	logger.Info("Connected to server")

	return nil
}

func (s *SpireServer) Close() error {
	if s.cancel != nil {
		s.cancel()
	}

	if s.conn != nil {
		s.conn.Close()
	}

	if s.source != nil {
		s.source.Close()
	}

	return nil
}

// CheckHealth actively queries the server's gRPC health check endpoint.
func (s *SpireServer) CheckHealth(ctx context.Context) (healthpb.HealthCheckResponse_ServingStatus, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return healthpb.HealthCheckResponse_UNKNOWN, err
	}

	client := healthpb.NewHealthClient(s.conn)
	resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		if status.Code(err) == codes.Unimplemented {
			return healthpb.HealthCheckResponse_SERVING, nil
		}
		logger.Error("Health check failed", err)
		return healthpb.HealthCheckResponse_UNKNOWN, err
	}
	return resp.Status, nil
}

// GetCachedHealthStatus returns the last known health status from the background updater in a thread-safe manner.
func (s *SpireServer) GetCachedHealthStatus() ServerHealthStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.HealthStatus
}

// RefreshCache pulls the latest agents, workloads, bundles, and federations
// from the remote SPIRE server and updates the local cache.
func (s *SpireServer) RefreshCache(ctx context.Context) error {
	if _, err := s.ListAgents(ctx, true); err != nil {
		return fmt.Errorf("failed to refresh agents: %w", err)
	}

	if _, err := s.ListWorkloads(ctx, true); err != nil {
		return fmt.Errorf("failed to refresh workloads: %w", err)
	}

	if _, err := s.ListFederatedBundles(ctx, true); err != nil {
		return fmt.Errorf("failed to refresh bundles: %w", err)
	}

	if _, err := s.ListFederatedServers(ctx, true); err != nil {
		return fmt.Errorf("failed to refresh federations: %w", err)
	}


	return nil
}
