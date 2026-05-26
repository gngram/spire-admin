package config
import (
	"os"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclwrite"
)


type ServerStatus string

type AppConfig struct {
	Servers  []ServerConfig `hcl:"server,block"`
}

type ServerConfig struct {
	Nickname    string `hcl:"name,label"`
	Address     string `hcl:"address,optional"`
	Port        string `hcl:"port,optional"`
	Aliases     []Alias `hcl:"alias,block"`
}

type Alias struct {
	SpiffeID string `hcl:"spiffe_id,label"`
	Name     string `hcl:"name,optional"`
}

type Server struct {
	Address     string
	Port        string
	Aliases     map[string]string
}

type Config struct {
	Servers map[string]Server
}

func NewConfig() *Config {
	return &Config{
		Servers: make(map[string]Server),
	}
}

func (cfg *Config) Load(path string) error {
	appCfg := &AppConfig{}
	parser := hclparse.NewParser()
	
	// If file doesn't exist, return empty config
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	file, diags := parser.ParseHCLFile(path)
	if diags.HasErrors() {
		return diags
	}

	gohcl.DecodeBody(file.Body, nil, appCfg)
	
	for _, sc := range appCfg.Servers {
		nickname := strings.TrimSpace(sc.Nickname)
		if nickname == "" {
			nickname = fmt.Sprintf("%s:%s", sc.Address, sc.Port)
		}
		aliases := make(map[string]string)
		for _, a := range sc.Aliases {
			aliases[a.SpiffeID] = a.Name
		}
		cfg.Servers[nickname] = Server{
			Address: sc.Address,
			Port:    sc.Port,
			Aliases: aliases,
		}
	}

	return nil
}

func (cfg *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	appCfg := &AppConfig{}
	for nickname, s := range cfg.Servers {
		sc := ServerConfig{
			Nickname: nickname,
			Address:  s.Address,
			Port:     s.Port,
		}
		for spiffeID, name := range s.Aliases {
			sc.Aliases = append(sc.Aliases, Alias{
				SpiffeID: spiffeID,
				Name:     name,
			})
		}
		appCfg.Servers = append(appCfg.Servers, sc)
	}
	f := hclwrite.NewEmptyFile()
	gohcl.EncodeIntoBody(appCfg, f.Body())
	return os.WriteFile(path, f.Bytes(), 0644)
}

func (cfg *Config) GetAlias(serverNickname, spiffeID string) string {
	if s, ok := cfg.Servers[serverNickname]; ok {
		if name, exists := s.Aliases[spiffeID]; exists && name != "" {
			return name
		}
	}
	return spiffeID
}

func (cfg *Config) SetAlias(serverNickname, spiffeID, name string) {
	if s, ok := cfg.Servers[serverNickname]; ok {
		if s.Aliases == nil {
			s.Aliases = make(map[string]string)
		}
		s.Aliases[spiffeID] = name
		cfg.Servers[serverNickname] = s
	}
}

func (cfg *Config) AddServer(s ServerConfig) error {
	nickname := strings.TrimSpace(s.Nickname)
	if nickname == "" {
		nickname = fmt.Sprintf("%s:%s", s.Address, s.Port)
	}
	if _, exists := cfg.Servers[nickname]; exists {
		return fmt.Errorf("server with nickname %q already exists", nickname)
	}
	aliases := make(map[string]string)
	for _, a := range s.Aliases {
		aliases[a.SpiffeID] = a.Name
	}
	cfg.Servers[nickname] = Server{
		Address: s.Address,
		Port:    s.Port,
		Aliases: aliases,
	}
	return nil
}

func (cfg *Config) DeleteServer(nickname string) {
	delete(cfg.Servers, nickname)
}
