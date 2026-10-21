package main

import (
	"flag"
	"os"
	"strconv"
	"time"

	"github.com/gngram/spire_admin/auth"
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

	// OIDC / Active Directory configuration flags with environment variable fallback
	oidcTenantID := flag.String("oidc-tenant-id", getEnv("OIDC_TENANT_ID", ""), "Microsoft Entra ID Tenant ID")
	oidcClientID := flag.String("oidc-client-id", getEnv("OIDC_CLIENT_ID", ""), "OIDC Client / Application ID")
	oidcClientSecret := flag.String("oidc-client-secret", getEnv("OIDC_CLIENT_SECRET", ""), "OIDC Client Secret")
	oidcRedirectURL := flag.String("oidc-redirect-url", getEnv("OIDC_REDIRECT_URL", ""), "OIDC Redirect URI (default: https://localhost:<port>/api/auth/callback)")
	oidcIssuer := flag.String("oidc-issuer", getEnv("OIDC_ISSUER", ""), "Custom OIDC Issuer URL (e.g. for AD FS or Dex)")
	oidcAdminRole := flag.String("oidc-admin-role", getEnv("OIDC_ADMIN_ROLE", "admin"), "OIDC role claim value granting admin privileges")
	oidcAdminGroup := flag.String("oidc-admin-group", getEnv("OIDC_ADMIN_GROUP", "SPIRE-Admins"), "OIDC group claim value granting admin privileges")

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

	redirectURL := *oidcRedirectURL
	if redirectURL == "" && (*oidcClientID != "" && (*oidcTenantID != "" || *oidcIssuer != "")) {
		redirectURL = "https://localhost:" + strconv.Itoa(*port) + "/api/auth/callback"
	}

	oidcCfg := auth.OIDCConfig{
		Issuer:       *oidcIssuer,
		TenantID:     *oidcTenantID,
		ClientID:     *oidcClientID,
		ClientSecret: *oidcClientSecret,
		RedirectURL:  redirectURL,
		AdminRole:    *oidcAdminRole,
		AdminGroup:   *oidcAdminGroup,
	}

	run(*parentSocket, *sessionTimeout, *certFile, *keyFile, *port, oidcCfg)
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
