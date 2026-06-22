package main

import (
	"context"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gngram/spire_admin/auth"
	"github.com/gngram/spire_admin/logger"
	"github.com/gngram/spire_admin/servers"
	bundlev1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/bundle/v1"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
)

var (
	webLogBuffer string
	webLogMu     sync.Mutex
)

type webLogWriter struct{}

func (w *webLogWriter) Write(p []byte) (n int, err error) {
	webLogMu.Lock()
	webLogBuffer += string(p)
	if len(webLogBuffer) > 50000 { // Keep the buffer size manageable
		webLogBuffer = webLogBuffer[len(webLogBuffer)-50000:]
	}
	webLogMu.Unlock()
	return len(p), nil
}

type WebApp struct {
	mu           sync.RWMutex
	servers      map[int]*servers.SpireServer
	Auth         *auth.Manager
	nextServerID int
	socket       string
}

func NewWebApp(parentSocket string, sessionTimeout time.Duration) *WebApp {
	return &WebApp{
		servers:      make(map[int]*servers.SpireServer),
		nextServerID: 1,
		socket:       parentSocket,
		Auth:         auth.NewManager("SECURE_RANDOM_KEY", sessionTimeout),
	}
}

func run(
	parentSocket string,
	sessionTimeout time.Duration,
	certFile string,
	keyFile string,
	port int) {

	log.SetOutput(io.MultiWriter(os.Stdout, &webLogWriter{}))

	app := NewWebApp(parentSocket, sessionTimeout)
	r := gin.Default()
	r.SetTrustedProxies(nil)

	portStr := strconv.Itoa(port)
	r.Static("/app", "./web")
	r.StaticFile("/favicon.ico", "./web/favicon.ico")

	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/app/login.html")
	})

	api := r.Group("/api")
	{
		api.POST("/login", app.Auth.HandleLogin)

		authGroup := api.Group("/")
		authGroup.Use(app.Auth.AuthMiddleware())
		{
			authGroup.GET("/logs", app.handleGetLogs)

			authGroup.GET("/servers", app.handleGetServers)
			authGroup.POST("/servers", app.handleAddServer)
			authGroup.DELETE("/servers/:id", app.handleDeleteServer)
			authGroup.POST("/servers/:id/refresh", app.handleRefreshServer)

			// Agents
			authGroup.GET("/servers/:id/agents", app.handleGetAgents)
			authGroup.GET("/servers/:id/agents/info", app.handleGetAgentInfo)
			authGroup.POST("/servers/:id/agents/evict", app.handleEvictAgent)
			authGroup.POST("/servers/:id/agents/ban", app.handleBanAgent)
			authGroup.POST("/servers/:id/agents/purge-expired", app.handlePurgeExpiredAgents)

			// Entries
			authGroup.GET("/servers/:id/entries", app.handleGetEntries)
			authGroup.GET("/servers/:id/entries/workloads", app.handleGetWorkloadEntries)
			authGroup.GET("/servers/:id/entries/agents", app.handleGetAgentEntries)
			authGroup.GET("/servers/:id/entries/downstreams", app.handleGetDownstreamEntries)
			authGroup.GET("/servers/:id/entries/:entry_id/info", app.handleGetEntryInfo)
			authGroup.POST("/servers/:id/entries", app.handleCreateEntry)
			authGroup.PUT("/servers/:id/entries/:entry_id", app.handleUpdateEntry)
			authGroup.DELETE("/servers/:id/entries/:entry_id", app.handleDeleteEntry)

			// Trust Bundles
			authGroup.GET("/servers/:id/bundles", app.handleListBundles)
			authGroup.GET("/servers/:id/bundles/info", app.handleGetBundleInfo)
			authGroup.POST("/servers/:id/bundles", app.handleSetBundle)
			authGroup.DELETE("/servers/:id/bundles", app.handleDeleteBundle)

			// Federations
			authGroup.GET("/servers/:id/federations", app.handleListFederations)
			authGroup.GET("/servers/:id/federations/info", app.handleGetFederationInfo)
			authGroup.POST("/servers/:id/federations", app.handleCreateFederation)
			authGroup.PUT("/servers/:id/federations", app.handleUpdateFederation)
			authGroup.POST("/servers/:id/federations/refresh", app.handleRefreshFederation)
			authGroup.DELETE("/servers/:id/federations", app.handleDeleteFederation)
			authGroup.POST("/servers/:id/federations/internal", app.handleFederateInternalServer)

			// Local Authority
			authGroup.GET("/servers/:id/local-authority", app.handleGetLocalAuthority)
			authGroup.POST("/servers/:id/local-authority/rotate", app.handleRotateLocalAuthority)
			authGroup.POST("/servers/:id/local-authority/activate", app.handleActivateLocalAuthority)
			authGroup.POST("/servers/:id/local-authority/delete", app.handleDeleteLocalAuthority)

			// Upstream Authority
			authGroup.POST("/servers/:id/upstream-authority/taint", app.handleTaintUpstreamAuthority)
			authGroup.POST("/servers/:id/upstream-authority/revoke", app.handleRevokeUpstreamAuthority)

			authGroup.POST("/change-password", app.Auth.HandleChangePassword)
			authGroup.POST("/logout", app.Auth.HandleLogout)
			authGroup.POST("/users", app.Auth.AdminMiddleware(), app.Auth.HandleCreateUser)
		}
	}

	if certFile != "" && keyFile != "" {
		r.RunTLS(":"+portStr, certFile, keyFile)
	} else {
		r.Run(":" + portStr)
	}
}

func (a *WebApp) handleGetLogs(c *gin.Context) {
	webLogMu.Lock()
	logs := webLogBuffer
	webLogMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

func (a *WebApp) handleAddServer(c *gin.Context) {
	var req struct {
		Name    string `json:"name"`
		Address string `json:"address"`
		Port    string `json:"port"`
	}
	logger.Info("handleAddServer calling..")
	c.ShouldBindJSON(&req)
	srv, err := servers.NewSpireServer(req.Name, req.Address, req.Port, a.socket)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.mu.Lock()
	id := a.nextServerID
	a.servers[id] = srv
	a.nextServerID++
	a.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"id": id, "status": "added"})
}

func (a *WebApp) handleGetServers(c *gin.Context) {
	logger.Info("handleGetServer calling..")
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := []map[string]interface{}{}
	for id, s := range a.servers {
		logger.Info("Server %s Status is: %s\n", s.Nickname, servers.StatusString(s.GetCachedHealthStatus()))
		result = append(result, map[string]interface{}{
			"id":           id,
			"name":         s.Nickname,
			"address":      s.Address,
			"domain":       s.Domain,
			"status":       servers.StatusString(s.GetCachedHealthStatus()),
			"last_updated": s.GetLastUpdated().Unix(),
		})
	}
	c.JSON(http.StatusOK, result)
}

func (a *WebApp) handleDeleteServer(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.mu.Lock()
	srv, exists := a.servers[id]
	if !exists {
		a.mu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}

	// Close connections before removing from the map
	srv.Close()
	delete(a.servers, id)
	a.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (a *WebApp) handleRefreshServer(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	go srv.FetchInfo()
	c.JSON(http.StatusOK, gin.H{"status": "refreshing"})
}

func (a *WebApp) handleGetAgents(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	agents, err := srv.ListAgents(ctx, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, agents)
}

func (a *WebApp) handleGetAgentInfo(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	spiffeID := c.Query("spiffe_id")
	if spiffeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "spiffe_id parameter is required"})
		return
	}
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	agentInfo, err := srv.GetAgentInfo(ctx, spiffeID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"info": agentInfo})
}

func (a *WebApp) handleEvictAgent(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		SpiffeID string `json:"spiffe_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.EvictAgent(ctx, req.SpiffeID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "evicted"})
}

func (a *WebApp) handleBanAgent(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		SpiffeID string `json:"spiffe_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.BanAgent(ctx, req.SpiffeID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "banned"})
}

func (a *WebApp) handlePurgeExpiredAgents(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.PurgeExpiredAgents(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "purged"})
}

func (a *WebApp) handleGetEntries(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	entries, err := srv.ListEntries(ctx, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entries)
}

func (a *WebApp) handleGetWorkloadEntries(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := srv.ListEntries(ctx, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, srv.GetWorkloadsEntries())
}

func (a *WebApp) handleGetAgentEntries(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := srv.ListEntries(ctx, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, srv.GetAgentsEntries())
}

func (a *WebApp) handleGetDownstreamEntries(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := srv.ListEntries(ctx, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, srv.GetDownstreamsEntries())
}

func (a *WebApp) handleGetEntryInfo(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	entryID := c.Param("entry_id")
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	entry, err := srv.GetEntry(ctx, entryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entry)
}

func (a *WebApp) handleCreateEntry(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		SpiffeID  string   `json:"spiffe_id"`
		ParentID  string   `json:"parent_id"`
		Selectors []string `json:"selectors"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	spiffeID, err := servers.ParseSPIFFEID(req.SpiffeID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid spiffe_id: " + err.Error()})
		return
	}
	parentID, err := servers.ParseSPIFFEID(req.ParentID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid parent_id: " + err.Error()})
		return
	}
	var selectors []*types.Selector
	for _, s := range req.Selectors {
		parts := strings.SplitN(strings.TrimSpace(s), ":", 2)
		if len(parts) == 2 {
			selectors = append(selectors, &types.Selector{
				Type:  strings.TrimSpace(parts[0]),
				Value: strings.TrimSpace(parts[1]),
			})
		}
	}
	entry := &types.Entry{
		SpiffeId:  spiffeID,
		ParentId:  parentID,
		Selectors: selectors,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	created, err := srv.CreateEntry(ctx, entry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, created)
}

func (a *WebApp) handleUpdateEntry(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	entryID := c.Param("entry_id")
	var req struct {
		DnsNames      []string `json:"dns_names"`
		Hint          string   `json:"hint"`
		Ttl           int32    `json:"ttl"`
		FederatesWith []string `json:"federates_with"`
		Downstream    bool     `json:"downstream"`
		Admin         bool     `json:"admin"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	entry, err := srv.GetEntry(ctx, entryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	entry.DnsNames = req.DnsNames
	entry.Hint = req.Hint
	entry.X509SvidTtl = req.Ttl
	entry.JwtSvidTtl = req.Ttl
	entry.FederatesWith = req.FederatesWith
	entry.Downstream = req.Downstream
	entry.Admin = req.Admin

	updated, err := srv.UpdateEntry(ctx, entry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (a *WebApp) handleDeleteEntry(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	entryID := c.Param("entry_id")
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.DeleteEntry(ctx, entryID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (a *WebApp) handleListBundles(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	bundles, err := srv.ListFederatedBundles(ctx, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, bundles)
}

func (a *WebApp) handleGetBundleInfo(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	trustDomain := c.Query("trust_domain")
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	bundle, err := srv.GetBundle(ctx, trustDomain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, bundle)
}

func (a *WebApp) handleSetBundle(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		TrustDomain string `json:"trust_domain"`
		PemContent  string `json:"pem_content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}

	var authorities []*types.X509Certificate
	rest := []byte(req.PemContent)
	for {
		block, remainder := pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			authorities = append(authorities, &types.X509Certificate{Asn1: block.Bytes})
		}
		rest = remainder
	}

	if len(authorities) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one valid X.509 certificate is required"})
		return
	}

	bundle := &types.Bundle{
		TrustDomain:     req.TrustDomain,
		X509Authorities: authorities,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	setBundle, err := srv.SetFederatedBundle(ctx, bundle)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, setBundle)
}

func (a *WebApp) handleDeleteBundle(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	trustDomain := c.Query("trust_domain")
	if trustDomain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trust_domain parameter is required"})
		return
	}
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := srv.DeleteFederatedBundle(ctx, trustDomain, bundlev1.BatchDeleteFederatedBundleRequest_RESTRICT)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (a *WebApp) handleListFederations(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	federations, err := srv.ListFederatedServers(ctx, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, federations)
}

func (a *WebApp) handleGetFederationInfo(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	trustDomain := c.Query("trust_domain")
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rel, err := srv.GetFederationRelationship(ctx, trustDomain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rel)
}

func (a *WebApp) handleCreateFederation(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		TrustDomain      string `json:"trust_domain"`
		EndpointURL      string `json:"endpoint_url"`
		ProfileType      string `json:"profile_type"`
		EndpointSpiffeID string `json:"endpoint_spiffe_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	rel := &types.FederationRelationship{
		TrustDomain:       req.TrustDomain,
		BundleEndpointUrl: req.EndpointURL,
	}
	if req.ProfileType == "spiffe" || req.ProfileType == "HTTPS SPIFFE" {
		rel.BundleEndpointProfile = &types.FederationRelationship_HttpsSpiffe{
			HttpsSpiffe: &types.HTTPSSPIFFEProfile{
				EndpointSpiffeId: req.EndpointSpiffeID,
			},
		}
	} else {
		rel.BundleEndpointProfile = &types.FederationRelationship_HttpsWeb{
			HttpsWeb: &types.HTTPSWebProfile{},
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	created, err := srv.CreateFederationRelationship(ctx, rel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, created)
}

func (a *WebApp) handleUpdateFederation(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		TrustDomain      string `json:"trust_domain"`
		EndpointURL      string `json:"endpoint_url"`
		ProfileType      string `json:"profile_type"`
		EndpointSpiffeID string `json:"endpoint_spiffe_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	rel := &types.FederationRelationship{
		TrustDomain:       req.TrustDomain,
		BundleEndpointUrl: req.EndpointURL,
	}
	if req.ProfileType == "spiffe" || req.ProfileType == "HTTPS SPIFFE" {
		rel.BundleEndpointProfile = &types.FederationRelationship_HttpsSpiffe{
			HttpsSpiffe: &types.HTTPSSPIFFEProfile{
				EndpointSpiffeId: req.EndpointSpiffeID,
			},
		}
	} else {
		rel.BundleEndpointProfile = &types.FederationRelationship_HttpsWeb{
			HttpsWeb: &types.HTTPSWebProfile{},
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	updated, err := srv.UpdateFederationRelationship(ctx, rel, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (a *WebApp) handleFederateInternalServer(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		TargetServerID int `json:"target_server_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.mu.RLock()
	srvX, existsX := a.servers[id]
	srvY, existsY := a.servers[req.TargetServerID]
	a.mu.RUnlock()

	if !existsX {
		c.JSON(http.StatusNotFound, gin.H{"error": "Current server not found"})
		return
	}
	if !existsY {
		c.JSON(http.StatusNotFound, gin.H{"error": "Target server not found"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// 1. Pull bundle from target server Y
	bundle, err := srvY.GetBundle(ctx, "")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to pull target bundle: " + err.Error()})
		return
	}

	// 2. Push bundle to current server X
	_, err = srvX.SetFederatedBundle(ctx, bundle)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to push bundle to current server: " + err.Error()})
		return
	}

	// 3. Create federation relationship on current server X pointing to target Y
	portInt, _ := strconv.Atoi(srvY.Port)
	bundlePort := portInt + 364
	endpointURL := fmt.Sprintf("https://%s:%d", srvY.Address, bundlePort)
	spiffeID := fmt.Sprintf("spiffe://%s/spire/server", srvY.Domain)

	rel := &types.FederationRelationship{
		TrustDomain:       srvY.Domain,
		BundleEndpointUrl: endpointURL,
		BundleEndpointProfile: &types.FederationRelationship_HttpsSpiffe{
			HttpsSpiffe: &types.HTTPSSPIFFEProfile{
				EndpointSpiffeId: spiffeID,
			},
		},
	}

	created, err := srvX.CreateFederationRelationship(ctx, rel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create federation relationship: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, created)
}

func (a *WebApp) handleRefreshFederation(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	trustDomain := c.Query("trust_domain")
	if trustDomain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trust_domain parameter is required"})
		return
	}
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := srv.RefreshFederationBundle(ctx, trustDomain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "refreshed"})
}

func (a *WebApp) handleDeleteFederation(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	trustDomain := c.Query("trust_domain")
	if trustDomain == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "trust_domain parameter is required"})
		return
	}
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := srv.DeleteFederationRelationship(ctx, trustDomain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (a *WebApp) handleGetLocalAuthority(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := srv.ShowLocalX509Authorities(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (a *WebApp) handleRotateLocalAuthority(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	resp, err := srv.PrepareLocalX509Authority(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	fmt.Printf("Prepared local authority: %v\n", resp)
	c.JSON(http.StatusOK, resp)
}

func (a *WebApp) handleActivateLocalAuthority(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		AuthorityID string `json:"authority_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	fmt.Printf("Activating local authority %s\n", req.AuthorityID)
	resp, err := srv.ActivateLocalX509Authority(ctx, req.AuthorityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (a *WebApp) handleDeleteLocalAuthority(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		AuthorityID string `json:"authority_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if _, err := srv.TaintLocalX509Authority(ctx, req.AuthorityID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	resp, err := srv.RevokeLocalX509Authority(ctx, req.AuthorityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (a *WebApp) handleTaintUpstreamAuthority(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		SubjectKeyID string `json:"subject_key_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := srv.TaintUpstreamX509Authority(ctx, req.SubjectKeyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"subject_key_id": resp})
}

func (a *WebApp) handleRevokeUpstreamAuthority(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		SubjectKeyID string `json:"subject_key_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Server not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := srv.RevokeUpstreamX509Authority(ctx, req.SubjectKeyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"subject_key_id": resp})
}
