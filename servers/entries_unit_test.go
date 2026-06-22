package servers

import (
	"testing"

	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
)

func TestGetEntriesFiltering(t *testing.T) {
	server := &SpireServer{
		Agents: []Agent{
			{SPIFFEID: "spiffe://example.org/spire/agent/agent1"},
			{SPIFFEID: "spiffe://example.org/spire/agent/agent2"},
		},
		Entries: []Entry{
			// Downstream entry
			{
				ID:       "downstream1",
				SPIFFEID: "spiffe://example.org/downstream",
				Parent:   "spiffe://example.org/spire/server",
				Original: &types.Entry{Downstream: true},
			},
			// Agent entry (matching cached agent SPIFFEID)
			{
				ID:       "agent-entry-1",
				SPIFFEID: "spiffe://example.org/workload-on-agent-1",
				Parent:   "spiffe://example.org/spire/agent/agent1",
				Original: &types.Entry{Downstream: false},
			},
			// Agent entry (parent containing "/spire/server")
			{
				ID:       "agent-entry-2",
				SPIFFEID: "spiffe://example.org/workload-on-server",
				Parent:   "spiffe://example.org/spire/server/some-parent",
				Original: &types.Entry{Downstream: false},
			},
			// Agent entry (SPIFFEID containing "/spire/agent/")
			{
				ID:       "agent-entry-3",
				SPIFFEID: "spiffe://example.org/spire/agent/agent3",
				Parent:   "spiffe://example.org/some-other-parent",
				Original: &types.Entry{Downstream: false},
			},
			// Workload entry (regular workload, no agent matches)
			{
				ID:       "workload-entry-1",
				SPIFFEID: "spiffe://example.org/workload1",
				Parent:   "spiffe://example.org/spire/agent/unregistered-agent",
				Original: &types.Entry{Downstream: false},
			},
		},
	}

	// 1. Verify Downstreams
	downstreams := server.GetDownstreamsEntries()
	if len(downstreams) != 1 {
		t.Fatalf("expected 1 downstream entry, got %d", len(downstreams))
	}
	if downstreams[0].ID != "downstream1" {
		t.Errorf("expected downstream entry ID to be 'downstream1', got '%s'", downstreams[0].ID)
	}

	// 2. Verify Agents
	agents := server.GetAgentsEntries()
	if len(agents) != 3 {
		t.Fatalf("expected 3 agent entries, got %d", len(agents))
	}
	expectedAgentIDs := map[string]bool{
		"agent-entry-1": true,
		"agent-entry-2": true,
		"agent-entry-3": true,
	}
	for _, a := range agents {
		if !expectedAgentIDs[a.ID] {
			t.Errorf("unexpected agent entry ID '%s'", a.ID)
		}
	}

	// 3. Verify Workloads
	workloads := server.GetWorkloadsEntries()
	if len(workloads) != 1 {
		t.Fatalf("expected 1 workload entry, got %d", len(workloads))
	}
	if workloads[0].ID != "workload-entry-1" {
		t.Errorf("expected workload entry ID to be 'workload-entry-1', got '%s'", workloads[0].ID)
	}
}
