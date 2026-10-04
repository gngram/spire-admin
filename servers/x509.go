package servers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gngram/spire_admin/logger"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// X509Service provides operations related to X.509 SVIDs and SPIFFE identities.
type X509Service struct {
	ctx *ServerContext
}

// NewX509Service creates a new X509Service bound to the provided ServerContext.
func NewX509Service(ctx *ServerContext) *X509Service {
	return &X509Service{ctx: ctx}
}

// ParseSPIFFEID parses a raw SPIFFE ID string into a types.SPIFFEID structure.
func (x *X509Service) ParseSPIFFEID(id string) (*types.SPIFFEID, error) {
	return ParseSPIFFEID(id)
}

// ParseSPIFFEID is a package-level helper that parses a raw SPIFFE ID string.
func ParseSPIFFEID(id string) (*types.SPIFFEID, error) {
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

// MintX509SVID is a placeholder for minting an X.509 SVID directly from the SPIRE server.
func (x *X509Service) MintX509SVID(ctx context.Context, spiffeID string, ttl time.Duration) ([]byte, error) {
	return nil, status.Errorf(codes.Unimplemented, "MintX509SVID is not yet implemented")
}

// ValidateX509SVID is a placeholder for validating an X.509 SVID against the trust bundle.
func (x *X509Service) ValidateX509SVID(ctx context.Context, svidBytes []byte) (bool, error) {
	return false, status.Errorf(codes.Unimplemented, "ValidateX509SVID is not yet implemented")
}
