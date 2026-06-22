package servers

import (
	"context"
	"fmt"
	"strings"

	"github.com/gngram/spire_admin/logger"
	entryv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/entry/v1"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
)

// Entry represents a workload/agent registered in the SPIRE server.
type Entry struct {
	ID        string
	SPIFFEID  string
	Selectors []string
	Parent    string
	Original  *types.Entry
}

// ListEntries lists all entries registered in the SPIRE server.
func (s *SpireServer) ListEntries(ctx context.Context, pull bool) ([]Entry, error) {
	if !pull {
		s.mu.RLock()
		if s.Entries != nil {
			defer s.mu.RUnlock()
			return s.Entries, nil
		}
		s.mu.RUnlock()
	}

	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := entryv1.NewEntryClient(s.conn)
	var allEntries []Entry
	var pageToken string

	for {
		resp, err := client.ListEntries(ctx, &entryv1.ListEntriesRequest{
			PageToken: pageToken,
		})
		if err != nil {
			logger.Error("Failed to list entries", err)
			return nil, err
		}

		for _, e := range resp.Entries {
			var selectors []string
			for _, sel := range e.Selectors {
				selectors = append(selectors, fmt.Sprintf("%s:%s", sel.Type, sel.Value))
			}

			s.mu.RLock()
			domain := s.Domain
			s.mu.RUnlock()

			spiffeID := "spiffe://" + domain + "/unknown"
			if e.SpiffeId != nil {
				spiffeID = fmt.Sprintf("spiffe://%s%s", e.SpiffeId.TrustDomain, e.SpiffeId.Path)
			}

			parent := ""
			if e.ParentId != nil {
				parent = fmt.Sprintf("spiffe://%s%s", e.ParentId.TrustDomain, e.ParentId.Path)
			}

			allEntries = append(allEntries, Entry{
				ID:        e.Id,
				SPIFFEID:  spiffeID,
				Selectors: selectors,
				Parent:    parent,
				Original:  e,
			})
		}

		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	s.mu.Lock()
	s.Entries = allEntries
	s.mu.Unlock()

	return allEntries, nil
}

// CountEntries returns the total number of registration entries in the SPIRE server.
func (s *SpireServer) CountEntries(ctx context.Context) (int32, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return 0, err
	}

	client := entryv1.NewEntryClient(s.conn)
	resp, err := client.CountEntries(ctx, &entryv1.CountEntriesRequest{})
	if err != nil {
		logger.Error("Failed to count entries", err)
		return 0, err
	}
	return resp.Count, nil
}

// CreateEntry creates a new registration entry.
func (s *SpireServer) CreateEntry(ctx context.Context, entry *types.Entry) (*types.Entry, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := entryv1.NewEntryClient(s.conn)
	resp, err := client.BatchCreateEntry(ctx, &entryv1.BatchCreateEntryRequest{
		Entries: []*types.Entry{entry},
	})
	if err != nil {
		logger.Error("Failed to batch create entry", err)
		return nil, err
	}
	if len(resp.Results) > 0 {
		if resp.Results[0].Status.Code != 0 {
			err = fmt.Errorf("failed to create entry: %s", resp.Results[0].Status.Message)
			logger.Error("Create entry error status", err)
			return nil, err
		}
		return resp.Results[0].Entry, nil
	}
	err = fmt.Errorf("unexpected empty response from server")
	logger.Error("Create entry empty response", err)
	return nil, err
}

// DeleteEntry deletes a registration entry by its ID.
func (s *SpireServer) DeleteEntry(ctx context.Context, entryID string) error {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return err
	}

	client := entryv1.NewEntryClient(s.conn)
	resp, err := client.BatchDeleteEntry(ctx, &entryv1.BatchDeleteEntryRequest{
		Ids: []string{entryID},
	})
	if err != nil {
		logger.Error("Failed to delete entry", err)
		return err
	}
	if len(resp.Results) > 0 && resp.Results[0].Status.Code != 0 {
		err = fmt.Errorf("failed to delete entry: %s", resp.Results[0].Status.Message)
		logger.Error("Delete entry error status", err)
		return err
	}
	return nil
}

// GetEntry displays a specific configured registration entry by its ID.
func (s *SpireServer) GetEntry(ctx context.Context, entryID string) (*types.Entry, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := entryv1.NewEntryClient(s.conn)
	resp, err := client.GetEntry(ctx, &entryv1.GetEntryRequest{Id: entryID})
	if err != nil {
		logger.Error("Failed to get entry", err)
	}
	return resp, err
}

// UpdateEntry updates an existing registration entry.
func (s *SpireServer) UpdateEntry(ctx context.Context, entry *types.Entry) (*types.Entry, error) {
	if err := s.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := entryv1.NewEntryClient(s.conn)
	resp, err := client.BatchUpdateEntry(ctx, &entryv1.BatchUpdateEntryRequest{
		Entries: []*types.Entry{entry},
	})
	if err != nil {
		logger.Error("Failed to update entry", err)
		return nil, err
	}
	if len(resp.Results) > 0 {
		if resp.Results[0].Status.Code != 0 {
			err = fmt.Errorf("failed to update entry: %s", resp.Results[0].Status.Message)
			logger.Error("Update entry error status", err)
			return nil, err
		}
		return resp.Results[0].Entry, nil
	}
	err = fmt.Errorf("unexpected empty response from server")
	logger.Error("Update entry empty response", err)
	return nil, err
}

// GetAgentsEntries returns all entries classified as agent entries.
func (s *SpireServer) GetAgentsEntries() []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var agents []Entry
	for _, e := range s.Entries {
		if e.Original != nil && e.Original.Downstream {
			continue
		}

		isAgent := false
		for _, a := range s.Agents {
			if e.Parent == a.SPIFFEID {
				isAgent = true
				break
			}
		}

		if isAgent || strings.Contains(e.Parent, "/spire/server") || strings.Contains(e.SPIFFEID, "/spire/agent/") {
			agents = append(agents, e)
		}
	}
	return agents
}

// GetWorkloadsEntries returns all entries classified as workload entries.
func (s *SpireServer) GetWorkloadsEntries() []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var workloads []Entry
	for _, e := range s.Entries {
		if e.Original != nil && e.Original.Downstream {
			continue
		}

		isAgent := false
		for _, a := range s.Agents {
			if e.Parent == a.SPIFFEID {
				isAgent = true
				break
			}
		}

		if isAgent || strings.Contains(e.Parent, "/spire/server") || strings.Contains(e.SPIFFEID, "/spire/agent/") {
			continue
		}

		workloads = append(workloads, e)
	}
	return workloads
}

// GetDownstreamsEntries returns all entries classified as downstream entries.
func (s *SpireServer) GetDownstreamsEntries() []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var downstreams []Entry
	for _, e := range s.Entries {
		if e.Original != nil && e.Original.Downstream {
			downstreams = append(downstreams, e)
		}
	}
	return downstreams
}
