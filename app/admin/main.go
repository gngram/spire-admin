package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/server/entry/v1"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/server/node/v1"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"github.com/spiffe/spire/pkg/agent"
	"github.com/spiffe/spire/pkg/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Paths - In NixOS, these would point to your generated files
	configPath := "server.conf"
	serverSocket := "/run/spire/sockets/admin.sock"
	agentSocket := "/run/spire/sockets/agent.sock"

	// 1. START SERVER FROM CONFIG FILE
	// We use the SPIRE internal loader to ensure HCL is parsed correctly
	// Note: You may need to manually populate server.Config if pkg/server/config is internal
	sConf := &server.Config{
		Log:             log.Default(),
		AdminSocketPath: serverSocket,
		TrustDomain:     "example.org", // Should match your file
		DataDir:         "./data",
		BindAddress:     &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8081},
	}

	s, err := server.New(sConf)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	go s.Run(ctx)
	time.Sleep(2 * time.Second)

	// 2. SECURELY AUTHENTICATE AGENT (Derive Join Token)
	token, err := createJoinToken(ctx, serverSocket)
	if err != nil {
		log.Fatalf("Failed to create join token: %v", err)
	}

	// 3. START AGENT
	a, err := agent.New(&agent.Config{
		Log:           log.Default(),
		ServerAddress: "127.0.0.1",
		ServerPort:    8081,
		SocketPath:    agentSocket,
		TrustDomain:   sConf.TrustDomain,
		JoinToken:     token,
	})
	if err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}
	go a.Run(ctx)

	// 4. REGISTER WORKLOAD
	err = registerWorkload(ctx, serverSocket, sConf.TrustDomain, token)
	if err != nil {
		log.Fatalf("Failed to register workload: %v", err)
	}

	// 5. RUN WORKLOAD
	runWorkload(ctx, agentSocket)
}

func createJoinToken(ctx context.Context, socket string) (string, error) {
	conn, err := grpc.DialContext(ctx, "unix://"+socket, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", err
	}
	defer conn.Close()

	client := node.NewNodeClient(conn)
	resp, err := client.BatchCreateJoinToken(ctx, &node.BatchCreateJoinTokenRequest{
		Tokens: []*node.JoinToken{{Ttl: 600}},
	})
	if err != nil || len(resp.Results) == 0 {
		return "", fmt.Errorf("token creation failed: %v", err)
	}
	return resp.Results[0].Value, nil
}

func registerWorkload(ctx context.Context, socket, domain, token string) error {
	conn, err := grpc.DialContext(ctx, "unix://"+socket, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entry.NewEntryClient(conn)
	// Parent ID for join_token is specific:
	parentID := fmt.Sprintf("spiffe://%s/spire/agent/join_token/%s", domain, token)

	_, err = client.BatchCreateEntry(ctx, &entry.BatchCreateEntryRequest{
		Entries: []*types.Entry{
			{
				SpiffeId: &types.SPIFFEID{TrustDomain: domain, Path: "/my-workload"},
				ParentId: &types.SPIFFEID{TrustDomain: domain, Path: "/spire/agent/join_token/" + token},
				Selectors: []*types.Selector{
					{Type: "unix", Value: "uid:1000"}, // matches the runner's UID
				},
			},
		},
	})
	return err
}

func runWorkload(ctx context.Context, socket string) {
	source, err := workloadapi.NewX509Source(ctx, workloadapi.WithAddress("unix://"+socket))
	if err != nil {
		log.Fatalf("Workload error: %v", err)
	}
	defer source.Close()

	svid, err := source.GetX509SVID()
	if err != nil {
		log.Fatalf("SVID error: %v", err)
	}
	fmt.Printf("Workload Identifed: %s\n", svid.ID)
}
