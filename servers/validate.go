package servers

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ValidateService provides validation helpers for SPIRE server configurations and bundles.
type ValidateService struct {
	ctx *ServerContext
}

// NewValidateService creates a new ValidateService bound to the provided ServerContext.
func NewValidateService(ctx *ServerContext) *ValidateService {
	return &ValidateService{ctx: ctx}
}

// ValidateConfig validates SPIRE server configuration data.
func (v *ValidateService) ValidateConfig(ctx context.Context, configData []byte) error {
	return status.Errorf(codes.Unimplemented, "ValidateConfig is not yet implemented")
}

// ValidateBundle validates SPIRE trust bundle data.
func (v *ValidateService) ValidateBundle(ctx context.Context, bundleData []byte) error {
	return status.Errorf(codes.Unimplemented, "ValidateBundle is not yet implemented")
}
