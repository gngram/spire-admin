package servers

import (
	"context"
	"crypto/x509"
	"fmt"

	"github.com/gngram/spire_admin/logger"
	bundlev1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/bundle/v1"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
)

// AppendX509RootCA appends a new X.509 Root CA certificate to the trust bundle of a specific trust domain.
// If the trustDomain is empty, it defaults to the server's own trust domain.
func (s *SpireServer) AppendX509RootCA(ctx context.Context, trustDomain string, cert *x509.Certificate) error {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return err
	}

	if trustDomain == "" {
		s.mu.RLock()
		trustDomain = s.Domain
		s.mu.RUnlock()
	}

	client := bundlev1.NewBundleClient(s.conn)
	req := &bundlev1.AppendBundleRequest{
		X509Authorities: []*types.X509Certificate{
			{Asn1: cert.Raw}, // Provide the ASN.1 DER encoded bytes
		},
	}

	_, err := client.AppendBundle(ctx, req)
	if err != nil {
		logger.Error("Failed to append bundle", err)
	}
	return err
}

// GetBundle retrieves the trust bundle (and its trusted Root CAs) for a specific trust domain.
// If the trustDomain is empty, it defaults to the server's own trust domain.
func (s *SpireServer) GetBundle(ctx context.Context, trustDomain string) (*types.Bundle, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	s.mu.RLock()
	serverDomain := s.Domain
	s.mu.RUnlock()

	if trustDomain == "" {
		trustDomain = serverDomain
	}

	client := bundlev1.NewBundleClient(s.conn)
	if trustDomain != serverDomain {
		resp, err := client.GetFederatedBundle(ctx, &bundlev1.GetFederatedBundleRequest{
			TrustDomain: trustDomain,
		})
		if err != nil {
			logger.Error("Failed to get federated bundle", err)
		}
		return resp, err
	}
	resp, err := client.GetBundle(ctx, &bundlev1.GetBundleRequest{})
	if err != nil {
		logger.Error("Failed to get bundle", err)
	}
	return resp, err
}

// DeleteFederatedBundle deletes federated bundle data.
// The mode determines what happens if there are associated entries (e.g. RESTRICT, DELETE, DISSOCIATE).
func (s *SpireServer) DeleteFederatedBundle(ctx context.Context, trustDomain string, mode bundlev1.BatchDeleteFederatedBundleRequest_Mode) error {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return err
	}

	client := bundlev1.NewBundleClient(s.conn)
	resp, err := client.BatchDeleteFederatedBundle(ctx, &bundlev1.BatchDeleteFederatedBundleRequest{
		TrustDomains: []string{trustDomain},
		Mode:         mode,
	})
	if err != nil {
		logger.Error("Failed to batch delete federated bundle", err)
		return err
	}
	if len(resp.Results) > 0 && resp.Results[0].Status.Code != 0 {
		err = fmt.Errorf("failed to delete federated bundle: %s", resp.Results[0].Status.Message)
		logger.Error("Delete federated bundle error status", err)
		return err
	}
	return nil
}

// ListFederatedBundles lists federated bundle data.
func (s *SpireServer) ListFederatedBundles(ctx context.Context, pull bool) ([]*types.Bundle, error) {
	if !pull {
		s.mu.RLock()
		if s.Bundles != nil {
			defer s.mu.RUnlock()
			return s.Bundles, nil
		}
		s.mu.RUnlock()
	}

	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := bundlev1.NewBundleClient(s.conn)
	var allBundles []*types.Bundle
	var pageToken string

	for {
		resp, err := client.ListFederatedBundles(ctx, &bundlev1.ListFederatedBundlesRequest{
			PageToken: pageToken,
		})
		if err != nil {
			logger.Error("Failed to list federated bundles", err)
			return nil, err
		}
		allBundles = append(allBundles, resp.Bundles...)
		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	s.mu.Lock()
	s.Bundles = allBundles
	s.mu.Unlock()

	return allBundles, nil
}

// SetFederatedBundle creates or updates federated bundle data.
func (s *SpireServer) SetFederatedBundle(ctx context.Context, bundle *types.Bundle) (*types.Bundle, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := bundlev1.NewBundleClient(s.conn)
	resp, err := client.BatchSetFederatedBundle(ctx, &bundlev1.BatchSetFederatedBundleRequest{
		Bundle: []*types.Bundle{bundle},
	})
	if err != nil {
		logger.Error("Failed to batch set federated bundle", err)
		return nil, err
	}
	if len(resp.Results) > 0 {
		if resp.Results[0].Status.Code != 0 {
			err = fmt.Errorf("failed to set federated bundle: %s", resp.Results[0].Status.Message)
			logger.Error("Set federated bundle error status", err)
			return nil, err
		}
		return resp.Results[0].Bundle, nil
	}
	err = fmt.Errorf("unexpected empty response from server")
	logger.Error("Set federated bundle empty response", err)
	return nil, err
}
