package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

// OIDCConfig holds the parameters required to initialize OpenID Connect authentication.
type OIDCConfig struct {
	Issuer       string
	TenantID     string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AdminRole    string
	AdminGroup   string
}

// OIDCManager handles OIDC provider discovery, token verification, and OAuth2 code exchange.
type OIDCManager struct {
	cfg      OIDCConfig
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauthCfg oauth2.Config
}

// IDTokenClaims captures standard and Microsoft Entra ID specific claims from the ID token.
type IDTokenClaims struct {
	Name              string   `json:"name"`
	PreferredUsername string   `json:"preferred_username"`
	Email             string   `json:"email"`
	Roles             []string `json:"roles"`
	Groups            []string `json:"groups"`
}

// NewOIDCManager creates and initializes an OIDCManager with discovery and verification.
func NewOIDCManager(ctx context.Context, cfg OIDCConfig) (*OIDCManager, error) {
	issuer := cfg.Issuer
	if issuer == "" && cfg.TenantID != "" {
		issuer = fmt.Sprintf("https://login.microsoftonline.com/%s/v2.0", cfg.TenantID)
	}
	if issuer == "" {
		return nil, fmt.Errorf("OIDC issuer or tenant ID is required")
	}

	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("failed to discover OIDC provider (%s): %w", issuer, err)
	}

	verifier := provider.Verifier(&oidc.Config{
		ClientID: cfg.ClientID,
	})

	oauthCfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	if cfg.AdminRole == "" {
		cfg.AdminRole = "admin"
	}
	if cfg.AdminGroup == "" {
		cfg.AdminGroup = "SPIRE-Admins"
	}

	return &OIDCManager{
		cfg:      cfg,
		provider: provider,
		verifier: verifier,
		oauthCfg: oauthCfg,
	}, nil
}

// IsAdmin determines whether the claims qualify the user as an Administrator.
func (o *OIDCManager) IsAdmin(claims *IDTokenClaims) bool {
	targetRole := strings.ToLower(o.cfg.AdminRole)
	for _, r := range claims.Roles {
		if strings.ToLower(r) == targetRole {
			return true
		}
	}

	targetGroup := strings.ToLower(o.cfg.AdminGroup)
	for _, g := range claims.Groups {
		if strings.ToLower(g) == targetGroup {
			return true
		}
	}
	return false
}

// HandleOIDCLogin generates a random state, sets a secure cookie, and redirects to the identity provider.
func (m *Manager) HandleOIDCLogin(c *gin.Context) {
	if m.OIDC == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "OIDC is not enabled"})
		return
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate random state"})
		return
	}
	state := base64.RawURLEncoding.EncodeToString(b)

	secure := c.Request.TLS != nil
	c.SetCookie("oauth_state", state, 300, "/", "", secure, true)

	authURL := m.OIDC.oauthCfg.AuthCodeURL(state)
	c.Redirect(http.StatusFound, authURL)
}

// HandleOIDCCallback processes the authorization code callback, verifies the ID token, and issues a session.
func (m *Manager) HandleOIDCCallback(c *gin.Context) {
	if m.OIDC == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "OIDC is not enabled"})
		return
	}

	cookieState, _ := c.Cookie("oauth_state")
	queryState := c.Query("state")
	if queryState == "" || cookieState == "" || queryState != cookieState {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired state parameter"})
		return
	}

	secure := c.Request.TLS != nil
	c.SetCookie("oauth_state", "", -1, "/", "", secure, true)

	if errCode := c.Query("error"); errCode != "" {
		errDesc := c.Query("error_description")
		c.JSON(http.StatusBadRequest, gin.H{"error": errCode, "description": errDesc})
		return
	}

	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing authorization code"})
		return
	}

	oauthToken, err := m.OIDC.oauthCfg.Exchange(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to exchange authorization code: " + err.Error()})
		return
	}

	rawIDToken, ok := oauthToken.Extra("id_token").(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Missing id_token in token response"})
		return
	}

	idToken, err := m.OIDC.verifier.Verify(c.Request.Context(), rawIDToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to verify ID token signature: " + err.Error()})
		return
	}

	var claims IDTokenClaims
	if err := idToken.Claims(&claims); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse ID token claims: " + err.Error()})
		return
	}

	username := claims.PreferredUsername
	if username == "" {
		username = claims.Email
	}
	if username == "" {
		username = claims.Name
	}
	if username == "" {
		username = idToken.Subject
	}

	isAdmin := m.OIDC.IsAdmin(&claims)

	sessionToken, err := m.CreateExternalSessionToken(username, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session token: " + err.Error()})
		return
	}

	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>Authenticating...</title></head>
<body>
<script>
    sessionStorage.setItem('spire_token', %q);
    sessionStorage.setItem('spire_admin', %t);
    window.location.replace('/app/dashboard.html');
</script>
<p>Authentication successful. Redirecting to dashboard...</p>
</body>
</html>`, sessionToken, isAdmin)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(htmlContent))
}
