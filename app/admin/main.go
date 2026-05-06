package main

import (
	"context"
	"flag"
	"os"

	log "github.com/sirupsen/logrus"
)

func setupLogger(logLevel string) {
	level, err := log.ParseLevel(logLevel)
	if err != nil {
		log.Fatalf("Invalid log level: %v", err)
	}

	log.SetLevel(level)
	log.SetReportCaller(true) // Enable file/line printing
	log.SetOutput(os.Stdout)

	log.Debug("Checking internals...")
	log.Info("Service running")
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	configPath := flag.String("config", "./admin.hcl", "Path to config file")
	logLevel := flag.String("log-level", "info", "Log level (e.g. debug, info, warn, error)")
	flag.Parse()

	log.Printf("Starting admin server with log level: %s", *logLevel)
	setupLogger(*logLevel)

	cfg, err := LoadConfig(*configPath, true)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	admin := NewAdmin(cfg)
	if err := admin.Start(ctx); err != nil {
		log.Fatalf("Failed to start admin: %v", err)
	}
}
