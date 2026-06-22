package servers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gngram/spire_admin/logger"
	agentv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/agent/v1"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// Agent represents a SPIRE Agent connected to the server.
type Agent struct {
	SPIFFEID        string
	AttestationType string
	ExpirationTime  int64
	SerialNumber    string
	CanReattest     bool
	AgentVersion    string
	Banned          bool
}

// String returns the agent info in a formatted string.
func (a Agent) String() string {
	expiration := "Unknown"
	if a.ExpirationTime != 0 {
		expiration = time.Unix(a.ExpirationTime, 0).String()
	}
	return fmt.Sprintf("SPIFFE ID         : %s\n"+
		"Attestation type  : %s\n"+
		"Expiration time   : %s\n"+
		"Serial number     : %s\n"+
		"Can re-attest     : %t\n"+
		"Agent version     : %s\n"+
		"Banned            : %t",
		a.SPIFFEID, a.AttestationType, expiration, a.SerialNumber, a.CanReattest, a.AgentVersion, a.Banned)
}

// ListAgents lists all agents connected to the SPIRE server.
func (s *SpireServer) ListAgents(ctx context.Context, pull bool) ([]Agent, error) {
	logger.Info("List AGnets.........")
	if !pull {
		s.mu.RLock()
		if s.Agents != nil {
			defer s.mu.RUnlock()
			logger.Info("Returning cached agents: %v", s.Agents)
			return s.Agents, nil
		}
		s.mu.RUnlock()
	}
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connection Error", err)
		return nil, err
	}

	client := agentv1.NewAgentClient(s.conn)
	var allAgents []Agent
	var pageToken string

	for {
		resp, err := client.ListAgents(ctx, &agentv1.ListAgentsRequest{
			PageToken: pageToken,
		})
		if err != nil {
			logger.Error("failed to list agents", err)
			return nil, err
		}

		for _, a := range resp.Agents {
			spiffeID := "Unknown"
			if a.Id != nil {
				spiffeID = fmt.Sprintf("spiffe://%s%s", a.Id.TrustDomain, a.Id.Path)
			}
			allAgents = append(allAgents, Agent{
				SPIFFEID:        spiffeID,
				AttestationType: a.AttestationType,
				ExpirationTime:  a.X509SvidExpiresAt,
				SerialNumber:    a.X509SvidSerialNumber,
				CanReattest:     a.CanReattest,
				AgentVersion:    a.AgentVersion,
				Banned:          a.Banned,
			})
		}

		pageToken = resp.NextPageToken
		if pageToken == "" {
			logger.Info("Returning agents: %v", allAgents)
			break
		}
	}
	s.mu.Lock()
	s.Agents = allAgents
	s.mu.Unlock()
	return allAgents, nil
}

// CreateJoinToken generates a new join token to register a new agent to the server.
func (s *SpireServer) CreateJoinToken(ctx context.Context, ttlSeconds int32) (*types.JoinToken, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := agentv1.NewAgentClient(s.conn)
	resp, err := client.CreateJoinToken(ctx, &agentv1.CreateJoinTokenRequest{Ttl: ttlSeconds})
	if err != nil {
		logger.Error("Failed to create join token", err)
	}
	return resp, err
}

// BanAgent bans the agent with the given SPIFFE ID.
func (s *SpireServer) BanAgent(ctx context.Context, spiffeID string) error {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return err
	}

	id, err := ParseSPIFFEID(spiffeID)
	if err != nil {
		logger.Error("Parse SPIFFE ID error", err)
		return err
	}

	client := agentv1.NewAgentClient(s.conn)
	_, err = client.BanAgent(ctx, &agentv1.BanAgentRequest{Id: id})
	if err != nil {
		logger.Error("Failed to ban agent", err)
	}
	return err
}

// EvictAgent evicts (deletes) the agent with the given SPIFFE ID.
func (s *SpireServer) EvictAgent(ctx context.Context, spiffeID string) error {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return err
	}

	id, err := ParseSPIFFEID(spiffeID)
	if err != nil {
		logger.Error("Parse SPIFFE ID error", err)
		return err
	}

	client := agentv1.NewAgentClient(s.conn)
	_, err = client.DeleteAgent(ctx, &agentv1.DeleteAgentRequest{Id: id})
	if err != nil {
		logger.Error("Failed to evict agent", err)
	}
	return err
}

// GetAgentInfo returns the details of the agent with the given SPIFFE ID.
func (s *SpireServer) GetAgentInfo(ctx context.Context, spiffeID string) (Agent, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return Agent{}, err
	}

	id, err := ParseSPIFFEID(spiffeID)
	if err != nil {
		logger.Error("Parse SPIFFE ID error", err)
		return Agent{}, err
	}

	client := agentv1.NewAgentClient(s.conn)
	resp, err := client.GetAgent(ctx, &agentv1.GetAgentRequest{Id: id})
	if err != nil {
		logger.Error("Failed to get agent details", err)
		return Agent{}, err
	}

	agent := Agent{
		SPIFFEID:        spiffeID,
		AttestationType: resp.AttestationType,
		ExpirationTime:  resp.X509SvidExpiresAt,
		SerialNumber:    resp.X509SvidSerialNumber,
		CanReattest:     resp.CanReattest,
		AgentVersion:    resp.AgentVersion,
		Banned:          resp.Banned,
	}

	return agent, nil
}

// PurgeExpiredAgents evicts all agents that have expired.
func (s *SpireServer) PurgeExpiredAgents(ctx context.Context) error {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return err
	}

	client := agentv1.NewAgentClient(s.conn)
	var errs []string
	var pageToken string

	for {
		req := &agentv1.ListAgentsRequest{
			Filter: &agentv1.ListAgentsRequest_Filter{
				ByCanReattest: wrapperspb.Bool(true),
			},
			OutputMask: &types.AgentMask{X509SvidExpiresAt: true},
			PageToken:  pageToken,
		}

		logger.Info("Purging expired agents...")
		resp, err := client.ListAgents(ctx, req)
		if err != nil {
			logger.Error("Failed to list agents", err)
			return fmt.Errorf("failed to list agents: %w", err)
		}

		for _, agent := range resp.Agents {
			if agent.Id == nil {
				continue
			}

			expirationTime := time.Unix(agent.X509SvidExpiresAt, 0)
			if time.Since(expirationTime) > 0 {
				if _, err := client.DeleteAgent(ctx, &agentv1.DeleteAgentRequest{Id: agent.Id}); err != nil {
					spiffeID := fmt.Sprintf("spiffe://%s%s", agent.Id.TrustDomain, agent.Id.Path)
					errs = append(errs, fmt.Sprintf("failed to delete agent %s: %v", spiffeID, err))
				}
			}
		}

		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	if len(errs) > 0 {
		logger.Error("Encountered errors while purging agents", fmt.Errorf("%s", strings.Join(errs, "; ")))
		return fmt.Errorf("encountered errors while purging agents: %s", strings.Join(errs, "; "))
	}

	return nil
}
