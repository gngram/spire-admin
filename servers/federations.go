package servers

import (
	"context"
	"fmt"

	"github.com/gngram/spidar/logger"
	trustdomainv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/trustdomain/v1"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
)

// FederatedServer represents a server with which this SPIRE server federates.
type FederatedServer struct {
	Address     string
	TrustDomain string
}

// ListFederatedServers lists all servers with which this SPIRE server federates.
func (s *SpireServer) ListFederatedServers(ctx context.Context, pull bool) ([]FederatedServer, error) {
	if !pull {
		s.mu.RLock()
		if s.FederatedServers != nil {
			defer s.mu.RUnlock()
			return s.FederatedServers, nil
		}
		s.mu.RUnlock()
	}

	rels, err := s.ListFederationRelationships(ctx)
	if err != nil {
		logger.Error("Failed to list federation relationships", err)
		return nil, err
	}

	var servers []FederatedServer
	for _, rel := range rels {
		servers = append(servers, FederatedServer{
			Address:     rel.BundleEndpointUrl,
			TrustDomain: rel.TrustDomain,
		})
	}

	s.mu.Lock()
	s.FederatedServers = servers
	s.mu.Unlock()

	return servers, nil
}

// CreateFederationRelationship creates a dynamic federation relationship with a foreign trust domain.
func (s *SpireServer) CreateFederationRelationship(ctx context.Context, rel *types.FederationRelationship) (*types.FederationRelationship, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := trustdomainv1.NewTrustDomainClient(s.conn)
	resp, err := client.BatchCreateFederationRelationship(ctx, &trustdomainv1.BatchCreateFederationRelationshipRequest{
		FederationRelationships: []*types.FederationRelationship{rel},
	})
	if err != nil {
		logger.Error("Failed to batch create federation relationship", err)
		return nil, err
	}
	if len(resp.Results) > 0 {
		if resp.Results[0].Status.Code != 0 {
			err = fmt.Errorf("failed to create federation relationship: %s", resp.Results[0].Status.Message)
			logger.Error("Create federation relationship error status", err)
			return nil, err
		}
		return resp.Results[0].FederationRelationship, nil
	}
	err = fmt.Errorf("unexpected empty response from server")
	logger.Error("Create federation relationship empty response", err)
	return nil, err
}

// DeleteFederationRelationship deletes a dynamic federation relationship.
func (s *SpireServer) DeleteFederationRelationship(ctx context.Context, trustDomain string) error {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return err
	}

	client := trustdomainv1.NewTrustDomainClient(s.conn)
	resp, err := client.BatchDeleteFederationRelationship(ctx, &trustdomainv1.BatchDeleteFederationRelationshipRequest{
		TrustDomains: []string{trustDomain},
	})
	if err != nil {
		logger.Error("Failed to batch delete federation relationship", err)
		return err
	}
	if len(resp.Results) > 0 && resp.Results[0].Status.Code != 0 {
		err = fmt.Errorf("failed to delete federation relationship: %s", resp.Results[0].Status.Message)
		logger.Error("Delete federation relationship error status", err)
		return err
	}
	return nil
}

// ListFederationRelationships lists all dynamic federation relationships.
func (s *SpireServer) ListFederationRelationships(ctx context.Context) ([]*types.FederationRelationship, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := trustdomainv1.NewTrustDomainClient(s.conn)
	var allRels []*types.FederationRelationship
	var pageToken string

	for {
		resp, err := client.ListFederationRelationships(ctx, &trustdomainv1.ListFederationRelationshipsRequest{
			PageToken: pageToken,
		})
		if err != nil {
			logger.Error("Failed to list federation relationships", err)
			return nil, err
		}
		allRels = append(allRels, resp.FederationRelationships...)
		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return allRels, nil
}

// RefreshFederationBundle refreshes the bundle from the specified federated trust domain.
func (s *SpireServer) RefreshFederationBundle(ctx context.Context, trustDomain string) error {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return err
	}

	client := trustdomainv1.NewTrustDomainClient(s.conn)
	_, err := client.RefreshBundle(ctx, &trustdomainv1.RefreshBundleRequest{
		TrustDomain: trustDomain,
	})
	if err != nil {
		logger.Error("Failed to refresh federation bundle", err)
	}
	return err
}

// GetFederationRelationship shows a dynamic federation relationship.
func (s *SpireServer) GetFederationRelationship(ctx context.Context, trustDomain string) (*types.FederationRelationship, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := trustdomainv1.NewTrustDomainClient(s.conn)
	resp, err := client.GetFederationRelationship(ctx, &trustdomainv1.GetFederationRelationshipRequest{
		TrustDomain: trustDomain,
	})
	if err != nil {
		logger.Error("Failed to get federation relationship", err)
	}
	return resp, err
}

// UpdateFederationRelationship updates a dynamic federation relationship with a foreign trust domain.
func (s *SpireServer) UpdateFederationRelationship(ctx context.Context, rel *types.FederationRelationship, mask *types.FederationRelationshipMask) (*types.FederationRelationship, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := trustdomainv1.NewTrustDomainClient(s.conn)
	resp, err := client.BatchUpdateFederationRelationship(ctx, &trustdomainv1.BatchUpdateFederationRelationshipRequest{
		FederationRelationships: []*types.FederationRelationship{rel},
		InputMask:               mask,
	})
	if err != nil {
		logger.Error("Failed to batch update federation relationship", err)
		return nil, err
	}
	if len(resp.Results) > 0 {
		if resp.Results[0].Status.Code != 0 {
			err = fmt.Errorf("failed to update federation relationship: %s", resp.Results[0].Status.Message)
			logger.Error("Update federation relationship error status", err)
			return nil, err
		}
		return resp.Results[0].FederationRelationship, nil
	}
	err = fmt.Errorf("unexpected empty response from server")
	logger.Error("Update federation relationship empty response", err)
	return nil, err
}
