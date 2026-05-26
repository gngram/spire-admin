package servers

import (
	"context"
	"fmt"

	"github.com/gngram/spidar/logger"
	localauthorityv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/localauthority/v1"
)

// ActivateLocalX509Authority activates a prepared X.509 authority for use.
// It causes it to be used for all X.509 signing operations serviced by this server going forward.
func (s *SpireServer) ActivateLocalX509Authority(ctx context.Context, authorityID string) (*localauthorityv1.AuthorityState, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := localauthorityv1.NewLocalAuthorityClient(s.conn)
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

// PrepareLocalX509Authority prepares a new X.509 authority for use by generating
// a new key and injecting the resulting CA certificate into the bundle.
func (s *SpireServer) PrepareLocalX509Authority(ctx context.Context) (*localauthorityv1.AuthorityState, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := localauthorityv1.NewLocalAuthorityClient(s.conn)
	resp, err := client.PrepareX509Authority(ctx, &localauthorityv1.PrepareX509AuthorityRequest{})
	if err != nil {
		err = fmt.Errorf("failed to prepare X.509 authority: %w", err)
		logger.Error("Prepare local X509 authority error", err)
		return nil, err
	}
	return resp.PreparedAuthority, nil
}

// RevokeLocalX509Authority revokes the previously active X.509 authority by removing
// it from the bundle and propagating this update throughout the cluster.
func (s *SpireServer) RevokeLocalX509Authority(ctx context.Context, authorityID string) (*localauthorityv1.AuthorityState, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := localauthorityv1.NewLocalAuthorityClient(s.conn)
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

// ShowLocalX509Authorities shows the local X.509 authorities (active, prepared, and old).
func (s *SpireServer) ShowLocalX509Authorities(ctx context.Context) (*localauthorityv1.GetX509AuthorityStateResponse, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := localauthorityv1.NewLocalAuthorityClient(s.conn)
	resp, err := client.GetX509AuthorityState(ctx, &localauthorityv1.GetX509AuthorityStateRequest{})
	if err != nil {
		err = fmt.Errorf("failed to get X.509 authority state: %w", err)
		logger.Error("Get X509 authority state error", err)
		return nil, err
	}
	return resp, nil
}

// TaintLocalX509Authority marks the previously active X.509 authority as being tainted.
func (s *SpireServer) TaintLocalX509Authority(ctx context.Context, authorityID string) (*localauthorityv1.AuthorityState, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := localauthorityv1.NewLocalAuthorityClient(s.conn)
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

// RevokeUpstreamX509Authority revokes the previously active X.509 upstream authority.
func (s *SpireServer) RevokeUpstreamX509Authority(ctx context.Context, subjectKeyID string) (string, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return "", err
	}

	client := localauthorityv1.NewLocalAuthorityClient(s.conn)
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
func (s *SpireServer) TaintUpstreamX509Authority(ctx context.Context, subjectKeyID string) (string, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return "", err
	}

	client := localauthorityv1.NewLocalAuthorityClient(s.conn)
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
