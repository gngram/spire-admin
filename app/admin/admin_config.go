package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

type BindAddressConfig struct {
	IP   string `json:"ip"`
	Port int    `json:"port"`
}

type DataStoreConfig struct {
	Type             string `json:"type"`
	ConnectionString string `json:"connection_string"`
}

type AdminConfig struct {
	Socket  string                 `json:"agent_socket"`
	TrustDomain  string            `json:"trust_domain"`
	DataDir      string            `json:"data_dir"`
	RuntimeDir   string            `json:"runtime_dir"`
	BindAddress  BindAddressConfig `json:"bind_address"`
	DataStore    DataStoreConfig   `json:"data_store"`
}

func defaultConfig() *AdminConfig {
	return &AdminConfig{
		Socket:  "/run/spidar/spidar_admin.sock",
		TrustDomain:  "example.org",
		DataDir:      "/var/lib/spidar",
		RuntimeDir: "/run/spidar",
		BindAddress: BindAddressConfig{
			IP:   "127.0.0.1",
			Port: 8081,
		},
		DataStore: DataStoreConfig{
			Type:             "sqlite3",
			ConnectionString: "./data/datastore.sqlite3",
		},
	}
}

func LoadConfig(path string) (*AdminConfig, error) {
	cfg := defaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("config file %q not found, using defaults", path)
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	return cfg, nil
}
