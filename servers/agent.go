package servers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gngram/spidar/logger"
	agentv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/agent/v1"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
)

// Agent represents a SPIRE Agent connected to the server.
type Agent struct {
	SPIFFEID string
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
			allAgents = append(allAgents, Agent{SPIFFEID: spiffeID})
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

	id, err := parseSPIFFEID(spiffeID)
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

	id, err := parseSPIFFEID(spiffeID)
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

// AgentDetails returns the details of the agent with the given SPIFFE ID.
func (s *SpireServer) AgentDetails(ctx context.Context, spiffeID string) (*types.Agent, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	id, err := parseSPIFFEID(spiffeID)
	if err != nil {
		logger.Error("Parse SPIFFE ID error", err)
		return nil, err
	}

	client := agentv1.NewAgentClient(s.conn)
	resp, err := client.GetAgent(ctx, &agentv1.GetAgentRequest{Id: id})
	if err != nil {
		logger.Error("Failed to get agent details", err)
	}
	return resp, err
}

// PurgeExpiredAgents evicts all agents that have expired.
func (s *SpireServer) PurgeExpiredAgents(ctx context.Context) error {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return err
	}

	client := agentv1.NewAgentClient(s.conn)
	now := strconv.FormatInt(time.Now().Unix(), 10)

	var errs []string
	var pageToken string

	for {
		req := &agentv1.ListAgentsRequest{
			Filter: &agentv1.ListAgentsRequest_Filter{
				ByExpiresBefore: now,
			},
			PageToken: pageToken,
		}

		resp, err := client.ListAgents(ctx, req)
		if err != nil {
			logger.Error("Failed to list expired agents", err)
			return fmt.Errorf("failed to list expired agents: %w", err)
		}

		for _, agent := range resp.Agents {
			_, err := client.DeleteAgent(ctx, &agentv1.DeleteAgentRequest{Id: agent.Id})
			if err != nil {
				if agent.Id != nil {
					spiffeID := fmt.Sprintf("spiffe://%s%s", agent.Id.TrustDomain, agent.Id.Path)
					errs = append(errs, fmt.Sprintf("failed to delete agent %s: %v", spiffeID, err))
				} else {
					errs = append(errs, fmt.Sprintf("failed to delete agent with missing ID: %v", err))
				}
			}
		}

		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	if len(errs) > 0 {
		logger.Error("Encountered errors while purging agents", fmt.Errorf(strings.Join(errs, "; ")))
		return fmt.Errorf("encountered errors while purging agents: %s", strings.Join(errs, "; "))
	}

	return nil
}
