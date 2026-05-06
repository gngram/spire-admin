package spidar_config

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"

	agentruntime "github.com/spiffe/spire/cmd/spire-agent/cli/run"
	serverruntime "github.com/spiffe/spire/cmd/spire-server/cli/run"
	"github.com/spiffe/spire/pkg/agent"
	"github.com/spiffe/spire/pkg/common/config"
	"github.com/spiffe/spire/pkg/common/telemetry/server"
	"github.com/spiffe/spire/pkg/server"
)

//go:embed system-config.hcl
var systemConfig embed.FS

//go:embed agent.hcl
var agentConfig embed.FS

type WorkloadConfig struct {
	WorkloadManager WorkloadManager `hcl:"workloads,block"`
	Remaining       hcl.Body        `hcl:",remain"`
}

type WorkloadManager struct {
	Servers []SpireServer `hcl:"spire_server,block"`
}

type SpireServer struct {
	TrustDomain string     `hcl:"name,label"`
	Entries     []Workload `hcl:"workload,block"`
}

type Workload struct {
	Name      string   `hcl:"name,label"`
	SpiffeID  string   `hcl:"spiffe_id"`
	ParentID  string   `hcl:"parent_id"`
	Selectors []string `hcl:"selectors"`
}

func LoadWorkloadConfig(filename string) (*WorkloadConfig, error) {
	source, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("could not read file %s: %w", filename, err)
	}

	parser := hclparse.NewParser()

	file, diags := parser.ParseHCL(source, filename)
	if diags.HasErrors() {
		return nil, diags
	}

	var cfg RootConfig
	diags = gohcl.DecodeBody(file.Body, nil, &cfg)
	if diags.HasErrors() {
		return nil, diags
	}

	return &cfg, nil
}

func ParseFile(path string) (*server.Config, error) {
	c := &server.Config{}

	byteData, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			msg := "could not determine CWD; config file not found at %s: use -config"
			return nil, fmt.Errorf("config file not found at %s", path)
		}

		msg := "could not find config file %s: please use the -config flag"
		return nil, fmt.Errorf("config file not found at %s", absPath)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to read configuration at %q: %w", path, err)
	}
	data := string(byteData)

	// If envTemplate flag is passed, substitute $VARIABLES in configuration file
	if expandEnv {
		data = config.ExpandEnv(data)
	}

	if err := hcl.Decode(&c, data); err != nil {
		return nil, fmt.Errorf("unable to decode configuration at %q: %w", path, err)
	}

	return c, nil
}

func LoadConfig(hclPath string, allowUnknownConfig bool) (*server.Config, WorkloadConfig, error) {
	fileInput, err := serverruntime.ParseFile(hclPath, true)
	if err != nil {
		return nil, err
	}

	serverConfig, err := serverruntime.NewServerConfig(fileInput, nil, allowUnknownConfig)
	if err != nil {
		return nil, nil, err
	}

	workloadConfig, err := LoadWorkloadConfig(hclPath)
	if err != nil {
		return serverConfig, nil, err
	}

	return serviceConfig, workloadConfig, nil
}

func LoadAgentConfig() (*agent.Config, error) {
	fileInput, err := agentruntime.ParseFile(hclPath, true)
	if err != nil {
		return nil, err
	}

	return agentruntime.NewAgentConfig(fileInput, nil, allowUnknownConfig)
}
