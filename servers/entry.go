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

// EntryService manages SPIRE workload and agent registration entries.
type EntryService struct {
	ctx *ServerContext
}

// NewEntryService creates a new EntryService bound to the provided ServerContext.
func NewEntryService(ctx *ServerContext) *EntryService {
	return &EntryService{ctx: ctx}
}

// ListEntries lists all entries registered in the SPIRE server.
func (e *EntryService) ListEntries(ctx context.Context, pull bool) ([]Entry, error) {
	if !pull {
		e.ctx.mu.RLock()
		if e.ctx.Entries != nil {
			defer e.ctx.mu.RUnlock()
			return e.ctx.Entries, nil
		}
		e.ctx.mu.RUnlock()
	}

	if err := e.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := entryv1.NewEntryClient(e.ctx.conn)
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

		for _, ent := range resp.Entries {
			var selectors []string
			for _, sel := range ent.Selectors {
				selectors = append(selectors, fmt.Sprintf("%s:%s", sel.Type, sel.Value))
			}

			e.ctx.mu.RLock()
			domain := e.ctx.Domain
			e.ctx.mu.RUnlock()

			spiffeID := "spiffe://" + domain + "/unknown"
			if ent.SpiffeId != nil {
				spiffeID = fmt.Sprintf("spiffe://%s%s", ent.SpiffeId.TrustDomain, ent.SpiffeId.Path)
			}

			parent := ""
			if ent.ParentId != nil {
				parent = fmt.Sprintf("spiffe://%s%s", ent.ParentId.TrustDomain, ent.ParentId.Path)
			}

			allEntries = append(allEntries, Entry{
				ID:        ent.Id,
				SPIFFEID:  spiffeID,
				Selectors: selectors,
				Parent:    parent,
				Original:  ent,
			})
		}

		pageToken = resp.NextPageToken
		if pageToken == "" {
			break
		}
	}

	e.ctx.mu.Lock()
	e.ctx.Entries = allEntries
	e.ctx.mu.Unlock()

	return allEntries, nil
}

// CountEntries returns the total number of registration entries in the SPIRE server.
func (e *EntryService) CountEntries(ctx context.Context) (int32, error) {
	if err := e.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return 0, err
	}

	client := entryv1.NewEntryClient(e.ctx.conn)
	resp, err := client.CountEntries(ctx, &entryv1.CountEntriesRequest{})
	if err != nil {
		logger.Error("Failed to count entries", err)
		return 0, err
	}
	return resp.Count, nil
}

// CreateEntry creates a new registration entry.
func (e *EntryService) CreateEntry(ctx context.Context, entry *types.Entry) (*types.Entry, error) {
	if err := e.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := entryv1.NewEntryClient(e.ctx.conn)
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
func (e *EntryService) DeleteEntry(ctx context.Context, entryID string) error {
	if err := e.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return err
	}

	client := entryv1.NewEntryClient(e.ctx.conn)
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
func (e *EntryService) GetEntry(ctx context.Context, entryID string) (*types.Entry, error) {
	if err := e.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := entryv1.NewEntryClient(e.ctx.conn)
	resp, err := client.GetEntry(ctx, &entryv1.GetEntryRequest{Id: entryID})
	if err != nil {
		logger.Error("Failed to get entry", err)
	}
	return resp, err
}

// UpdateEntry updates an existing registration entry.
func (e *EntryService) UpdateEntry(ctx context.Context, entry *types.Entry) (*types.Entry, error) {
	if err := e.ctx.Connect(ctx); err != nil {
		logger.Error("Connect error", err)
		return nil, err
	}

	client := entryv1.NewEntryClient(e.ctx.conn)
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

// GetAgentsEntries returns all entries classified as agent entries from cache.
func (e *EntryService) GetAgentsEntries() []Entry {
	e.ctx.mu.RLock()
	defer e.ctx.mu.RUnlock()

	var agents []Entry
	for _, ent := range e.ctx.Entries {
		if ent.Original != nil && ent.Original.Downstream {
			continue
		}

		isAgent := false
		for _, a := range e.ctx.Agents {
			if ent.Parent == a.SPIFFEID {
				isAgent = true
				break
			}
		}

		if isAgent || strings.Contains(ent.Parent, "/spire/server") || strings.Contains(ent.SPIFFEID, "/spire/agent/") {
			agents = append(agents, ent)
		}
	}
	return agents
}

// GetWorkloadsEntries returns all entries classified as workload entries from cache.
func (e *EntryService) GetWorkloadsEntries() []Entry {
	e.ctx.mu.RLock()
	defer e.ctx.mu.RUnlock()

	var workloads []Entry
	for _, ent := range e.ctx.Entries {
		if ent.Original != nil && ent.Original.Downstream {
			continue
		}

		isAgent := false
		for _, a := range e.ctx.Agents {
			if ent.Parent == a.SPIFFEID {
				isAgent = true
				break
			}
		}

		if isAgent || strings.Contains(ent.Parent, "/spire/server") || strings.Contains(ent.SPIFFEID, "/spire/agent/") {
			continue
		}

		workloads = append(workloads, ent)
	}
	return workloads
}

// GetDownstreamsEntries returns all entries classified as downstream entries from cache.
func (e *EntryService) GetDownstreamsEntries() []Entry {
	e.ctx.mu.RLock()
	defer e.ctx.mu.RUnlock()

	var downstreams []Entry
	for _, ent := range e.ctx.Entries {
		if ent.Original != nil && ent.Original.Downstream {
			downstreams = append(downstreams, ent)
		}
	}
	return downstreams
}
