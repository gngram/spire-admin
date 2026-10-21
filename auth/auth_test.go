package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestAuthSessionManagement(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create manager with a 10s session timeout
	m := NewManager("test-secret-key", 10*time.Second)

	r := gin.New()
	r.POST("/login", m.HandleLogin)
	r.POST("/logout", m.AuthMiddleware(), m.HandleLogout)

	protected := r.Group("/")
	protected.Use(m.AuthMiddleware())
	protected.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Helper to login and get the token
	login := func(username, password string) string {
		body, _ := json.Marshal(map[string]string{
			"username": username,
			"password": password,
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/login", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("failed to login: %d", w.Code)
		}

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		return resp["token"].(string)
	}

	// Helper to make a protected GET request
	callProtected := func(token string) int {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		r.ServeHTTP(w, req)
		return w.Code
	}

	// Helper to logout
	callLogout := func(token string) int {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/logout", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		r.ServeHTTP(w, req)
		return w.Code
	}

	// 1. Log in and verify we can access the protected endpoint
	token1 := login("admin", "admin123")
	if status := callProtected(token1); status != http.StatusOK {
		t.Errorf("expected 200 OK for valid token, got %d", status)
	}

	// 2. Perform a second login (new connection request) for the same user
	token2 := login("admin", "admin123")

	// The old session (token1) must be terminated/invalidated now
	if status := callProtected(token1); status != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for old token after new login, got %d", status)
	}

	// The new session (token2) must be active
	if status := callProtected(token2); status != http.StatusOK {
		t.Errorf("expected 200 OK for new token, got %d", status)
	}

	// 3. Log out/disconnect the active session (token2)
	if status := callLogout(token2); status != http.StatusOK {
		t.Errorf("expected 200 OK for logout, got %d", status)
	}

	// The session (token2) must be terminated now
	if status := callProtected(token2); status != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for token after logout, got %d", status)
	}
}

func TestExternalSessionTokenAndProviders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	m := NewManager("test-secret-key", 10*time.Second)

	r := gin.New()
	r.GET("/api/auth/providers", m.HandleAuthProviders)

	protected := r.Group("/")
	protected.Use(m.AuthMiddleware())
	protected.GET("/user", func(c *gin.Context) {
		username, _ := c.Get("username")
		isAdmin, _ := c.Get("is_admin")
		c.JSON(http.StatusOK, gin.H{"username": username, "is_admin": isAdmin})
	})
	protected.GET("/admin-only", m.AdminMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "admin_granted"})
	})

	// 1. Providers endpoint when OIDC is nil
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/auth/providers", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for providers, got %d", w.Code)
	}
	var provResp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &provResp)
	if provResp["oidc_enabled"] != false || provResp["local_enabled"] != true {
		t.Errorf("unexpected providers response: %v", provResp)
	}

	// 2. Issue external session token for an admin user
	adminToken, err := m.CreateExternalSessionToken("ad_admin@corp.com", true)
	if err != nil {
		t.Fatalf("failed to create external admin token: %v", err)
	}

	wAdmin := httptest.NewRecorder()
	reqAdmin, _ := http.NewRequest("GET", "/admin-only", nil)
	reqAdmin.Header.Set("Authorization", "Bearer "+adminToken)
	r.ServeHTTP(wAdmin, reqAdmin)
	if wAdmin.Code != http.StatusOK {
		t.Errorf("expected 200 OK for external admin token on admin-only endpoint, got %d", wAdmin.Code)
	}

	// 3. Issue external session token for an operator (non-admin)
	opToken, err := m.CreateExternalSessionToken("ad_user@corp.com", false)
	if err != nil {
		t.Fatalf("failed to create external operator token: %v", err)
	}

	wOp := httptest.NewRecorder()
	reqOp, _ := http.NewRequest("GET", "/admin-only", nil)
	reqOp.Header.Set("Authorization", "Bearer "+opToken)
	r.ServeHTTP(wOp, reqOp)
	if wOp.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for non-admin on admin-only endpoint, got %d", wOp.Code)
	}
}

func TestOIDCManagerIsAdmin(t *testing.T) {
	mgr := &OIDCManager{
		cfg: OIDCConfig{
			AdminRole:  "admin",
			AdminGroup: "spire-admins",
		},
	}

	// Case 1: Role match
	claims1 := &IDTokenClaims{
		Roles: []string{"User", "Admin"},
	}
	if !mgr.IsAdmin(claims1) {
		t.Errorf("expected IsAdmin to be true for role match")
	}

	// Case 2: Group match
	claims2 := &IDTokenClaims{
		Groups: []string{"Engineers", "SPIRE-Admins"},
	}
	if !mgr.IsAdmin(claims2) {
		t.Errorf("expected IsAdmin to be true for group match")
	}

	// Case 3: Neither match
	claims3 := &IDTokenClaims{
		Roles:  []string{"Viewer"},
		Groups: []string{"General"},
	}
	if mgr.IsAdmin(claims3) {
		t.Errorf("expected IsAdmin to be false when neither role nor group match")
	}
}

