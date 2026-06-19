package main

import (
	"flag"
	"os"
	"time"

	"github.com/gngram/spire_admin/logger"
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
	certFile := flag.String("cert", "/tmp/spire-certs/cert.pem", "Path to the TLS certificate file")
	keyFile := flag.String("key", "/tmp/spire-certs/key.pem", "Path to the TLS private key file")
	port := flag.Int("port", 8443, "Port to listen on")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	sessionTimeout := flag.Duration("session-timeout", 30*time.Minute, "Inactivity timeout for sessions")
	flag.Parse()

	logger.SetLevelString(*logLevel)

	if !fileExists(*parentSocket) {
		logger.Error("Parent socket (%s) not found", *parentSocket)
		flag.Usage()
		return
	}

	if !fileExists(*certFile) {
		logger.Error("Certificate file (%s) not found", *certFile)
		flag.Usage()
		return
	}

	if !fileExists(*keyFile) {
		logger.Error("Key file (%s) not found", *keyFile)
		flag.Usage()
		return
	}
	run(*parentSocket, *sessionTimeout, *certFile, *keyFile, *port)
}
