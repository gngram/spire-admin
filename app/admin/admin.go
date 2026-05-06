package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	agentv1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/agent/v1"
	bundlev1 "github.com/spiffe/spire-api-sdk/proto/spire/api/server/bundle/v1"
	"github.com/spiffe/spire-api-sdk/proto/spire/api/types"
	"github.com/spiffe/spire/pkg/agent"
	"github.com/spiffe/spire/pkg/common/catalog"
	"github.com/spiffe/spire/pkg/server"
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
	cfg         *server.Config
	agentSocket string
}

func NewAdmin(cfg *server.Config) *Admin {
	return &Admin{
		cfg: cfg,
	}
}

func (a *Admin) Start(ctx context.Context) error {
	return a.setupAdmin(ctx)
}

func (a *Admin) setupAdmin(ctx context.Context) error {
	serverLogger := &taggedLogger{log.WithField("component", "spidar-admin")}
	agentLogger := &taggedLogger{log.WithField("component", "spidar-agent")}
	a.cfg.Log = serverLogger

	s := server.New(*a.cfg)
	log.Infof("Admin server Created")

	valCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	pluginNotes, err := s.ValidateConfig(valCtx)
	if err != nil {
		log.Errorf("Could not validate configuration file: %v", err)
		return fmt.Errorf("Could not validate configuration file: %w", err)
	}
	if len(pluginNotes) != 0 {
		log.Errorf("SPIRE server configuration file is invalid.\nValidation errors:\n")
		for plugin, notes := range pluginNotes {
			if len(notes) == 0 {
				continue
			}

			log.Errorf("\t%s:\n", plugin)

			for _, note := range notes {
				log.Errorf("\t\t%s\n", note)
			}
		}
		return fmt.Errorf("SPIRE server configuration file is not valid.")
	}

	log.Infof("Server configuration file is valid.")

	go s.Run(ctx)

	/* check server is up */
	for {
		health := s.CheckHealth()
		if health.Live && health.Ready {
			log.Infof("Server Health is Live and Ready!")
			break
		}
		log.Infof("Server Health is Live: %v, Ready: %v, waiting ...", health.Live, health.Ready)
		time.Sleep(2 * time.Second)
	}

	bundle, err := a.FetchBundleWithRetry(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch bundle: %w", err)
	}

	token, err := a.createJoinToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to create join token: %v", err)
	}
	log.Infof("Join token: %s", token)

	bundlePath := a.cfg.DataDir + "/bundle.pem"
	if err := writeBundlePEM(bundle, bundlePath); err != nil {
		return fmt.Errorf("failed to write bundle: %w", err)
	}
	// Create a separate data directory for the agent to avoid file conflicts with the server.
	agentDataDir := a.cfg.DataDir + "/agent"
	if err := os.MkdirAll(agentDataDir, 0755); err != nil {
		return fmt.Errorf("failed to create agent data directory: %w", err)
	}

	ag := agent.New(&agent.Config{
		Log:           agentLogger,
		ServerAddress: fmt.Sprintf("%s:%d", a.cfg.BindAddress.IP, a.cfg.BindAddress.Port),
		BindAddress:   &net.UnixAddr{Net: "unix", Name: a.cfg.DataDir + "/agent.Socket"},
		TrustDomain:   a.cfg.TrustDomain,
		DataDir:       agentDataDir,
		JoinToken:     token,
		PluginConfigs: catalog.PluginConfigs{
			{
				Type: "KeyManager",
				Name: "disk",
				DataSource: catalog.FixedData(fmt.Sprintf(`
					directory = "%s/agent_keys"
				`, a.cfg.DataDir)),
			},
			{
				Type: "NodeAttestor",
				Name: "join_token",
			},
			{
				Type: "WorkloadAttestor",
				Name: "unix",
			},
		},
	})
	go ag.Run(ctx)

	log.Infof("Agent is running")
	time.Sleep(10 * time.Second)

	err = a.registerWorkload(ctx, token)
	if err != nil {
		return fmt.Errorf("failed to register workload: %v", err)
	}

	a.runWorkload(ctx)

	return nil
}

func writeBundlePEM(bundle *types.Bundle, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, auth := range bundle.X509Authorities {
		// auth.Asn1 is DER-encoded — parse then re-encode as PEM
		cert, err := x509.ParseCertificate(auth.Asn1)
		if err != nil {
			return fmt.Errorf("invalid X.509 authority: %w", err)
		}
		if err := pem.Encode(f, &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert.Raw,
		}); err != nil {
			return fmt.Errorf("PEM encode failed: %w", err)
		}
	}
	return nil
}

func (a *Admin) FetchBundle(ctx context.Context) (*types.Bundle, error) {
	// Connect to the server's local admin socket (no auth required)
	conn, err := grpc.NewClient(
		"unix://"+a.cfg.BindLocalAddress.String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server socket: %w", err)
	}
	defer conn.Close()

	client := bundlev1.NewBundleClient(conn)
	bundle, err := client.GetBundle(ctx, &bundlev1.GetBundleRequest{})
	if err != nil {
		return nil, fmt.Errorf("GetBundle failed: %w", err)
	}
	return bundle, nil
}

func (a *Admin) FetchBundleWithRetry(ctx context.Context) (*types.Bundle, error) {
	var bundle *types.Bundle
	var err error

	for i := 0; i < 10; i++ {
		bundle, err = a.FetchBundle(ctx)
		if err == nil {
			return bundle, nil
		}
		log.Warnf("Bundle not ready yet (%v), retrying...", err)
		time.Sleep(500 * time.Millisecond)
	}
	return nil, fmt.Errorf("bundle fetch failed after retries: %w", err)
}

func (a *Admin) createJoinToken(ctx context.Context) (string, error) {
	conn, err := grpc.NewClient("unix://"+a.cfg.BindLocalAddress.String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
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
	/*
		conn, err := grpc.DialContext(ctx, "unix://"+a.serverSocket, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return err
		}
		defer conn.Close()

		client := entry.NewEntryClient(conn)

		uidSelector := &types.Selector{Type: "unix", Value: fmt.Sprintf("uid:%d", os.Getuid())}
		trustDomain := a.cfg.TrustDomain
		parentPath := fmt.Sprintf("/spire/agent/join_token/%s", token)

		_, err = client.BatchCreateEntry(ctx, &entry.BatchCreateEntryRequest{
			Entries: []*types.Entry{
				{
					SpiffeId:  &types.SPIFFEID{TrustDomain: trustDomain, Path: "/workload-server"},
					ParentId:  &types.SPIFFEID{TrustDomain: trustDomain, Path: parentPath},
					Selectors: []*types.Selector{uidSelector},
				},
				{
					SpiffeId:  &types.SPIFFEID{TrustDomain: trustDomain, Path: "/workload-client"},
					ParentId:  &types.SPIFFEID{TrustDomain: trustDomain, Path: parentPath},
					Selectors: []*types.Selector{uidSelector},
				},
			},
		})

		return err
	*/
	return nil
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
