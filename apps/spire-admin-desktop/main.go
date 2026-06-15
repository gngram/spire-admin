package main

import (
	"flag"
	"os"

	"github.com/gngram/spire_admin/logger"
	"github.com/gngram/spire_admin/ui"
)

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func main() {
	parentSocket := flag.String("socket", "/tmp/spire-admin/agent.sock", "Path to the parent agent socket")
	width := flag.Int("width", 1280, "Initial window width")
	height := flag.Int("height", 720, "Initial window height")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	flag.Parse()

	logger.SetLevelString(*logLevel)

	if !fileExists(*parentSocket) {
		logger.Error("Parent socket (%s) not found", *parentSocket)
		flag.Usage()
		return
	}

	ui.OpenDashboard(*parentSocket, uint16(*width), uint16(*height))
}
