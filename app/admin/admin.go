package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	entry "github.com/spiffe/spire-api-sdk/proto/spire/api/server/entry/v1"
	agentv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/agent/v1"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"github.com/spiffe/spire/pkg/agent"
	"github.com/spiffe/spire/pkg/server"
	"github.com/spiffe/spire/pkg/server/plugin/keymanager"
	"github.com/spiffe/spire/pkg/common/catalog"
	log "github.com/sirupsen/logrus"
)

import (
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type taggedLogger struct {
	*logrus.Entry
}

func (l *taggedLogger) GetLevel() logrus.Level {
	return l.Logger.GetLevel()
}

func (l *taggedLogger) SetLevel(level logrus.Level) {
	l.Logger.SetLevel(level)
}

type Admin struct {
	cfg          *AdminConfig
	serverSocket string
	agentSocket  string
}

func NewAdmin(cfg *AdminConfig) *Admin {
	return &Admin{
		cfg:          cfg,
		serverSocket: cfg.RuntimeDir + "/spidar_admin.sock",
		agentSocket:  cfg.RuntimeDir + "/spidar_agent.sock",
	}
}

func (a *Admin) Start(ctx context.Context) error {
	return a.setupAdmin(ctx)
}

func (a *Admin) setupAdmin(ctx context.Context) error {
	logger := logrus.New()

	serverLogger := &taggedLogger{logger.WithField("component", "spidar-admin")}
	agentLogger := &taggedLogger{logger.WithField("component", "spidar-agent")}

	sConf := server.Config{
    Log:             serverLogger,
    BindLocalAddress: &net.UnixAddr{Net: "unix", Name: a.serverSocket}, 
    TrustDomain:     spiffeid.RequireTrustDomainFromString(a.cfg.TrustDomain),
    DataDir:         a.cfg.DataDir,
    BindAddress:     &net.TCPAddr{IP: net.ParseIP(a.cfg.BindAddress.IP), Port: a.cfg.BindAddress.Port},
		CAKeyType:       keymanager.ECP256,
		
		PluginConfigs: catalog.PluginConfigs{
        // Required: DataStore — sqlite3 stored in your DataDir
        {
            Type: "DataStore",
            Name: "sql",
            DataSource: catalog.FixedData(`
                database_type = "sqlite3"
                connection_string = "` + a.cfg.DataDir + `/datastore.sqlite3"
            `),
        },
        // Required: KeyManager — in-memory (use "disk" for persistence across restarts)
        {
            Type: "KeyManager",
            Name: "memory",
            DataSource: catalog.FixedData(`
                key_type = "rsa-2048"
            `),
        },
        // Required: NodeAttestor matching the join_token agent attestation
        {
            Type: "NodeAttestor",
            Name: "join_token",
        },
    },
  }

	s := server.New(sConf)
  log.Infof("Admin server Created")
	go s.Run(ctx)
  log.Infof("Admin server started")
	time.Sleep(2 * time.Second)

	token, err := a.createJoinToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to create join token: %v", err)
	}

	ag := agent.New(&agent.Config{
		Log:            agentLogger,
		ServerAddress:  fmt.Sprintf("%s:%d", a.cfg.BindAddress.IP, a.cfg.BindAddress.Port),
		BindAddress:    &net.UnixAddr{Net: "unix", Name: a.agentSocket},
		TrustDomain:    spiffeid.RequireTrustDomainFromString(a.cfg.TrustDomain),
		DataDir:        a.cfg.DataDir, // The Agent persists its state (like bundles) using DataDir instead of a DataStore plugin
		JoinToken:      token,
		PluginConfigs: catalog.PluginConfigs{
        // Required: KeyManager for the agent's own key
        {
            Type: "KeyManager",
            Name: "memory",
            DataSource: catalog.FixedData(`
                key_type = "rsa-2048"
            `),
        },
        // Required: NodeAttestor — must match the server-side join_token attestor
        {
            Type: "NodeAttestor",
            Name: "join_token",
        },
        // Required: WorkloadAttestor — unix for uid-based selectors
        {
            Type: "WorkloadAttestor",
            Name: "unix",
        },
    },
	})
	go ag.Run(ctx)

	err = a.registerWorkload(ctx, token)
	if err != nil {
		return fmt.Errorf("failed to register workload: %v", err)
	}

	a.runWorkload(ctx)

	return nil
}

func (a *Admin) createJoinToken(ctx context.Context) (string, error) {
	conn, err := grpc.DialContext(ctx, "unix://"+a.serverSocket, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", err
	}
	defer conn.Close()

	client := agentv1.NewAgentClient(conn)
	resp, err := client.CreateJoinToken(ctx, &agentv1.CreateJoinTokenRequest{
		Ttl: 600,
	})
	if err != nil {
		return "", fmt.Errorf("token creation failed: %v", err)
	}
	return resp.Value, nil
}

func (a *Admin) registerWorkload(ctx context.Context, token string) error {
	conn, err := grpc.DialContext(ctx, "unix://"+a.serverSocket, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	client := entry.NewEntryClient(conn)

	uidSelector := &types.Selector{Type: "unix", Value: fmt.Sprintf("uid:%d", os.Getuid())}

	_, err = client.BatchCreateEntry(ctx, &entry.BatchCreateEntryRequest{
		Entries: []*types.Entry{
			{
				SpiffeId: &types.SPIFFEID{TrustDomain: a.cfg.TrustDomain, Path: "/workload-server"},
				ParentId: &types.SPIFFEID{TrustDomain: a.cfg.TrustDomain, Path: "/spire/agent/join_token/" + token},
				Selectors: []*types.Selector{uidSelector},
			},
			{
				SpiffeId: &types.SPIFFEID{TrustDomain: a.cfg.TrustDomain, Path: "/workload-client"},
				ParentId: &types.SPIFFEID{TrustDomain: a.cfg.TrustDomain, Path: "/spire/agent/join_token/" + token},
				Selectors: []*types.Selector{uidSelector},
			},
		},
	})
	return err
}

func (a *Admin) runWorkload(ctx context.Context) {
	source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(workloadapi.WithAddr("unix://"+a.agentSocket))) // ✅
	if err != nil {
		log.Fatalf("Workload error: %v", err)
	}
	defer source.Close()

	svid, err := source.GetX509SVID()
	if err != nil {
		log.Fatalf("SVID error: %v", err)
	}
	fmt.Printf("Workload Identified with Default SVID: %s\n", svid.ID)

	// 1. Setup an mTLS Server
	serverConfig := tlsconfig.MTLSServerConfig(source, source, tlsconfig.AuthorizeAny())
	listener, err := tls.Listen("tcp", "127.0.0.1:8443", serverConfig)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Server accept error: %v", err)
			return
		}
		defer conn.Close()

		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			log.Printf("Server read error: %v", err)
			return
		}
		fmt.Printf("Server received: %s\n", string(buf[:n]))
		conn.Write([]byte("Hello from the SPIFFE mTLS Server!"))
	}()

	// 2. Connect via an mTLS Client
	clientConfig := tlsconfig.MTLSClientConfig(source, source, tlsconfig.AuthorizeAny())
	conn, err := tls.Dial("tcp", "127.0.0.1:8443", clientConfig)
	if err != nil {
		log.Fatalf("Failed to dial server: %v", err)
	}
	defer conn.Close()

	conn.Write([]byte("Hello from the SPIFFE mTLS Client!"))

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Fatalf("Client read error: %v", err)
	}
	fmt.Printf("Client received: %s\n", string(buf[:n]))
}
