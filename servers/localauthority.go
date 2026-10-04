package servers

import (
	"context"
	"fmt"

	"github.com/gngram/spire_admin/logger"
	localauthorityv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/localauthority/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// LocalAuthorityService manages local SPIRE CA signing authorities (active, prepared, and old).
type LocalAuthorityService struct {
	ctx *ServerContext
}

// NewLocalAuthorityService creates a new LocalAuthorityService bound to the provided ServerContext.
func NewLocalAuthorityService(ctx *ServerContext) *LocalAuthorityService {
	return &LocalAuthorityService{ctx: ctx}
}

// ShowLocalX509Authorities shows the local X.509 authorities (active, prepared, and old).
func (l *LocalAuthorityService) ShowLocalX509Authorities(ctx context.Context) (*localauthorityv1.GetX509AuthorityStateResponse, error) {
	if err := l.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := localauthorityv1.NewLocalAuthorityClient(l.ctx.conn)
	resp, err := client.GetX509AuthorityState(ctx, &localauthorityv1.GetX509AuthorityStateRequest{})
	if err != nil {
		err = fmt.Errorf("failed to get X.509 authority state: %w", err)
		logger.Error("Get X509 authority state error", err)
		return nil, err
	}
	return resp, nil
}

// PrepareLocalX509Authority prepares a new X.509 authority for use by generating
// a new key and injecting the resulting CA certificate into the bundle.
func (l *LocalAuthorityService) PrepareLocalX509Authority(ctx context.Context) (*localauthorityv1.AuthorityState, error) {
	if err := l.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := localauthorityv1.NewLocalAuthorityClient(l.ctx.conn)
	resp, err := client.PrepareX509Authority(ctx, &localauthorityv1.PrepareX509AuthorityRequest{})
	if err != nil {
		err = fmt.Errorf("failed to prepare X.509 authority: %w", err)
		logger.Error("Prepare local X509 authority error", err)
		return nil, err
	}
	return resp.PreparedAuthority, nil
}

// ActivateLocalX509Authority activates a prepared X.509 authority for use.
// It causes it to be used for all X.509 signing operations serviced by this server going forward.
func (l *LocalAuthorityService) ActivateLocalX509Authority(ctx context.Context, authorityID string) (*localauthorityv1.AuthorityState, error) {
	if err := l.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := localauthorityv1.NewLocalAuthorityClient(l.ctx.conn)
	resp, err := client.ActivateX509Authority(ctx, &localauthorityv1.ActivateX509AuthorityRequest{
		AuthorityId: authorityID,
	})
	if err != nil {
		err = fmt.Errorf("failed to activate X.509 authority: %w", err)
		logger.Error("Activate local X509 authority error", err)
		return nil, err
	}
	return resp.ActivatedAuthority, nil
}

// TaintLocalX509Authority marks the previously active X.509 authority as being tainted.
func (l *LocalAuthorityService) TaintLocalX509Authority(ctx context.Context, authorityID string) (*localauthorityv1.AuthorityState, error) {
	if err := l.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := localauthorityv1.NewLocalAuthorityClient(l.ctx.conn)
	resp, err := client.TaintX509Authority(ctx, &localauthorityv1.TaintX509AuthorityRequest{
		AuthorityId: authorityID,
	})
	if err != nil {
		err = fmt.Errorf("failed to taint X.509 authority: %w", err)
		logger.Error("Taint local X509 authority error", err)
		return nil, err
	}
	return resp.TaintedAuthority, nil
}

// RevokeLocalX509Authority revokes the previously active X.509 authority by removing
// it from the bundle and propagating this update throughout the cluster.
func (l *LocalAuthorityService) RevokeLocalX509Authority(ctx context.Context, authorityID string) (*localauthorityv1.AuthorityState, error) {
	if err := l.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := localauthorityv1.NewLocalAuthorityClient(l.ctx.conn)
	resp, err := client.RevokeX509Authority(ctx, &localauthorityv1.RevokeX509AuthorityRequest{
		AuthorityId: authorityID,
	})
	if err != nil {
		err = fmt.Errorf("failed to revoke X.509 authority: %w", err)
		logger.Error("Revoke local X509 authority error", err)
		return nil, err
	}
	return resp.RevokedAuthority, nil
}

// ShowLocalJWTAuthorities is a placeholder for querying local JWT authorities.
func (l *LocalAuthorityService) ShowLocalJWTAuthorities(ctx context.Context) (interface{}, error) {
	return nil, status.Errorf(codes.Unimplemented, "ShowLocalJWTAuthorities is not yet implemented")
}
