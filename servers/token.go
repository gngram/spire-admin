package servers

import (
	"context"

	"github.com/gngram/spire_admin/logger"
	agentv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/agent/v1"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TokenService manages token generation and validation for SPIRE agents.
type TokenService struct {
	ctx *ServerContext
}

// NewTokenService creates a new TokenService bound to the provided ServerContext.
func NewTokenService(ctx *ServerContext) *TokenService {
	return &TokenService{ctx: ctx}
}

// CreateJoinToken generates a new join token to register a new agent to the server.
func (t *TokenService) CreateJoinToken(ctx context.Context, ttlSeconds int32) (*types.JoinToken, error) {
	if err := t.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := agentv1.NewAgentClient(t.ctx.conn)
	resp, err := client.CreateJoinToken(ctx, &agentv1.CreateJoinTokenRequest{Ttl: ttlSeconds})
	if err != nil {
		logger.Error("Failed to create join token", err)
	}
	return resp, err
}

// ValidateToken is a placeholder for validating join tokens.
func (t *TokenService) ValidateToken(ctx context.Context, token string) (bool, error) {
	return false, status.Errorf(codes.Unimplemented, "ValidateToken is not yet implemented")
}
