package servers

import (
	"context"
	"fmt"
	"time"

	"github.com/gngram/spire_admin/logger"
	bundlev1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/bundle/v1"
	"google.golang.org/grpc/codes"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// HealthCheckService manages SPIRE server health and background discovery.
type HealthCheckService struct {
	ctx *ServerContext
}

// NewHealthCheckService creates a new HealthCheckService bound to the provided ServerContext.
func NewHealthCheckService(ctx *ServerContext) *HealthCheckService {
	return &HealthCheckService{ctx: ctx}
}

// CheckHealth actively queries the server's gRPC health check endpoint.
func (h *HealthCheckService) CheckHealth(ctx context.Context) (healthpb.HealthCheckResponse_ServingStatus, error) {
	if err := h.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return healthpb.HealthCheckResponse_UNKNOWN, err
	}

	client := healthpb.NewHealthClient(h.ctx.conn)
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
func (h *HealthCheckService) GetCachedHealthStatus() ServerHealthStatus {
	h.ctx.mu.RLock()
	defer h.ctx.mu.RUnlock()
	return h.ctx.HealthStatus
}

// GetLastUpdated returns the last known update time in a thread-safe manner.
func (h *HealthCheckService) GetLastUpdated() time.Time {
	h.ctx.mu.RLock()
	defer h.ctx.mu.RUnlock()
	return h.ctx.LastUpdated
}

// FetchInfo queries the server for its trust domain and evaluates health status.
func (h *HealthCheckService) FetchInfo() {
	ctx, cancel := context.WithTimeout(h.ctx.ctx, 10*time.Second)
	defer cancel()

	if err := h.ctx.Connect(ctx); err != nil {
		logger.Error("Error connecting to server", err)
		h.ctx.mu.Lock()
		oldStatus := h.ctx.HealthStatus
		h.ctx.HealthStatus = Offline
		h.ctx.LastUpdated = time.Now()
		cb := h.ctx.OnHealthChange
		h.ctx.mu.Unlock()

		select {
		case <-h.ctx.ctx.Done():
			return
		default:
			if cb != nil && oldStatus != Offline {
				cb(Offline)
			}
		}
		return
	}

	bundleClient := bundlev1.NewBundleClient(h.ctx.conn)
	if bundle, err := bundleClient.GetBundle(ctx, &bundlev1.GetBundleRequest{}); err == nil && bundle != nil {
		h.ctx.mu.Lock()
		if h.ctx.Domain == "Unknown" || h.ctx.Domain == "" {
			h.ctx.Domain = bundle.TrustDomain
		}
		h.ctx.mu.Unlock()
	}

	logger.Info("Domain: %s", h.ctx.Domain)

	logger.Info("Checking health..")
	client := healthpb.NewHealthClient(h.ctx.conn)
	_, err := client.Check(ctx, &healthpb.HealthCheckRequest{})

	h.ctx.mu.Lock()
	oldStatus := h.ctx.HealthStatus

	if err != nil && status.Code(err) != codes.Unimplemented {
		logger.Error("Error checking health", err)
		h.ctx.HealthStatus = Offline
	} else {
		h.ctx.HealthStatus = Online
	}

	h.ctx.LastUpdated = time.Now()
	newStatus := h.ctx.HealthStatus
	cb := h.ctx.OnHealthChange
	h.ctx.mu.Unlock()

	select {
	case <-h.ctx.ctx.Done():
		return
	default:
		if cb != nil && oldStatus != newStatus {
			cb(newStatus)
		}
	}
}

// RefreshCache pulls the latest agents, workloads, bundles, and federations
// from the remote SPIRE server and updates the local cache.
func (h *HealthCheckService) RefreshCache(ctx context.Context) error {
	if h.ctx.server != nil {
		if _, err := h.ctx.server.ListAgents(ctx, true); err != nil {
			return fmt.Errorf("failed to refresh agents: %w", err)
		}

		if _, err := h.ctx.server.ListEntries(ctx, true); err != nil {
			return fmt.Errorf("failed to refresh entries: %w", err)
		}

		if _, err := h.ctx.server.ListFederatedBundles(ctx, true); err != nil {
			return fmt.Errorf("failed to refresh bundles: %w", err)
		}

		if _, err := h.ctx.server.ListFederatedServers(ctx, true); err != nil {
			return fmt.Errorf("failed to refresh federations: %w", err)
		}
	}

	return nil
}
