package servers

import (
	"context"
	"crypto/x509"
	"fmt"

	"github.com/gngram/spire_admin/logger"
	bundlev1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/bundle/v1"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// BundleService manages trust bundles and federated bundle operations.
type BundleService struct {
	ctx *ServerContext
}

// NewBundleService creates a new BundleService bound to the provided ServerContext.
func NewBundleService(ctx *ServerContext) *BundleService {
	return &BundleService{ctx: ctx}
}

// AppendX509RootCA appends a new X.509 Root CA certificate to the trust bundle of a specific trust domain.
// If the trustDomain is empty, it defaults to the server's own trust domain.
func (b *BundleService) AppendX509RootCA(ctx context.Context, trustDomain string, cert *x509.Certificate) error {
	if err := b.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return err
	}

	if trustDomain == "" {
		b.ctx.mu.RLock()
		trustDomain = b.ctx.Domain
		b.ctx.mu.RUnlock()
	}

	client := bundlev1.NewBundleClient(b.ctx.conn)
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
func (b *BundleService) GetBundle(ctx context.Context, trustDomain string) (*types.Bundle, error) {
	if err := b.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	b.ctx.mu.RLock()
	serverDomain := b.ctx.Domain
	b.ctx.mu.RUnlock()

	if trustDomain == "" {
		trustDomain = serverDomain
	}

	client := bundlev1.NewBundleClient(b.ctx.conn)
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
func (b *BundleService) DeleteFederatedBundle(ctx context.Context, trustDomain string, mode bundlev1.BatchDeleteFederatedBundleRequest_Mode) error {
	if err := b.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return err
	}

	client := bundlev1.NewBundleClient(b.ctx.conn)
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
func (b *BundleService) ListFederatedBundles(ctx context.Context, pull bool) ([]*types.Bundle, error) {
	if !pull {
		b.ctx.mu.RLock()
		if b.ctx.Bundles != nil {
			defer b.ctx.mu.RUnlock()
			return b.ctx.Bundles, nil
		}
		b.ctx.mu.RUnlock()
	}

	if err := b.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := bundlev1.NewBundleClient(b.ctx.conn)
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

	b.ctx.mu.Lock()
	b.ctx.Bundles = allBundles
	b.ctx.mu.Unlock()

	return allBundles, nil
}

// SetFederatedBundle creates or updates federated bundle data.
func (b *BundleService) SetFederatedBundle(ctx context.Context, bundle *types.Bundle) (*types.Bundle, error) {
	if err := b.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := bundlev1.NewBundleClient(b.ctx.conn)
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

// CountBundles is a placeholder for counting trust bundles.
func (b *BundleService) CountBundles(ctx context.Context) (int32, error) {
	return 0, status.Errorf(codes.Unimplemented, "CountBundles is not yet implemented")
}
