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
