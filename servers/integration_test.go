package servers

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestIntegration_SpireServerCaches(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 1. Get the absolute path to the setup script
	scriptPath, err := filepath.Abs("../test/setup_spire_env.sh")
	if err != nil {
		t.Fatalf("failed to resolve script path: %v", err)
	}

	// 2. Execute the bash script asynchronously
	cmd := exec.Command("bash", scriptPath)
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start setup script: %v", err)
	}

	// 3. Ensure we clean up the processes when the test finishes
	t.Cleanup(func() {
		if cmd.Process != nil {
			// Send Interrupt so the bash script's trap can clean up child processes gracefully
			cmd.Process.Signal(os.Interrupt)
			time.Sleep(2 * time.Second)
			cmd.Process.Kill()
			cmd.Wait()
		}
	})

	// 4. Wait for the SPIRE Agent socket to become available
	// The script places it in the 'test/data' directory.
	agentSocket := filepath.Join(filepath.Dir(scriptPath), "data", "agent1.sock")
	socketReady := false
	for i := 0; i < 30; i++ { // wait up to 30 seconds
		if _, err := os.Stat(agentSocket); err == nil {
			socketReady = true
			break
		}
		time.Sleep(1 * time.Second)
	}

	if !socketReady {
		t.Fatalf("timed out waiting for agent socket at %s", agentSocket)
	}

	// Give the servers/agents an extra 5 seconds to fully sync identities and federations
	time.Sleep(5 * time.Second)

	// 5. Initialize the SpireServer pointing to Server 1 (127.0.0.1:8081)
	// By passing agentSocket, we authenticate using the workload API!
	server, err := NewSpireServer("TestServer1", "127.0.0.1", "8081", agentSocket, 0)
	if err != nil {
		t.Fatalf("failed to create SpireServer instance: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 6. Test the APIs and verify the cache counts
	if err := server.RefreshCache(ctx); err != nil {
		t.Fatalf("RefreshCache failed: %v", err)
	}

	// Server 1 should have 2 agents registered to it
	if len(server.Agents) != 2 {
		t.Errorf("Expected 2 agents, got %d", len(server.Agents))
	}

	// Server 1 should have 3 entries: 2 standard workloads + 1 spire_admin-admin workload
	if len(server.Workloads) != 3 {
		t.Errorf("Expected 3 workloads, got %d", len(server.Workloads))
	}

	// Server 1 should have 1 dynamic federation configured with Server 2 (domain2.test)
	if len(server.FederatedServers) != 1 {
		t.Errorf("Expected 1 federation, got %d", len(server.FederatedServers))
	} else if server.FederatedServers[0].TrustDomain != "domain2.test" {
		t.Errorf("Expected federation with domain2.test, got %s", server.FederatedServers[0].TrustDomain)
	}

	t.Log("✅ Integration test passed! Data caches are working perfectly.")
}
