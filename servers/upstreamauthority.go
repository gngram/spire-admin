package servers

import (
	"context"
	"fmt"

	"github.com/gngram/spire_admin/logger"
	localauthorityv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/localauthority/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UpstreamAuthorityService manages upstream CA authority relationships and revocations.
type UpstreamAuthorityService struct {
	ctx *ServerContext
}

// NewUpstreamAuthorityService creates a new UpstreamAuthorityService bound to the provided ServerContext.
func NewUpstreamAuthorityService(ctx *ServerContext) *UpstreamAuthorityService {
	return &UpstreamAuthorityService{ctx: ctx}
}

// RevokeUpstreamX509Authority revokes the previously active X.509 upstream authority.
func (u *UpstreamAuthorityService) RevokeUpstreamX509Authority(ctx context.Context, subjectKeyID string) (string, error) {
	if err := u.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return "", err
	}

	client := localauthorityv1.NewLocalAuthorityClient(u.ctx.conn)
	resp, err := client.RevokeX509UpstreamAuthority(ctx, &localauthorityv1.RevokeX509UpstreamAuthorityRequest{
		SubjectKeyId: subjectKeyID,
	})
	if err != nil {
		err = fmt.Errorf("failed to revoke upstream X.509 authority: %w", err)
		logger.Error("Revoke upstream X509 authority error", err)
		return "", err
	}
	return resp.UpstreamAuthoritySubjectKeyId, nil
}

// TaintUpstreamX509Authority marks the provided X.509 upstream authority as being tainted.
func (u *UpstreamAuthorityService) TaintUpstreamX509Authority(ctx context.Context, subjectKeyID string) (string, error) {
	if err := u.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return "", err
	}

	client := localauthorityv1.NewLocalAuthorityClient(u.ctx.conn)
	resp, err := client.TaintX509UpstreamAuthority(ctx, &localauthorityv1.TaintX509UpstreamAuthorityRequest{
		SubjectKeyId: subjectKeyID,
	})
	if err != nil {
		err = fmt.Errorf("failed to taint upstream X.509 authority: %w", err)
		logger.Error("Taint upstream X509 authority error", err)
		return "", err
	}
	return resp.UpstreamAuthoritySubjectKeyId, nil
}

// ShowUpstreamAuthority is a placeholder for retrieving upstream authority state.
func (u *UpstreamAuthorityService) ShowUpstreamAuthority(ctx context.Context) (interface{}, error) {
	return nil, status.Errorf(codes.Unimplemented, "ShowUpstreamAuthority is not yet implemented")
}
