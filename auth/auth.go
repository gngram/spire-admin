package auth

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const usersFile = "users.json"

type User struct {
	Username string `json:"username"`
	Password string `json:"password"` // Persist hash to file
	IsAdmin  bool   `json:"is_admin"`
}

type Claims struct {
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
	jwt.RegisteredClaims
}

type Manager struct {
	mu            sync.RWMutex
	users         map[string]*User
	jwtSigningKey []byte
	SessionTimeout time.Duration
	lastActivity   sync.Map // token string -> time.Time
}

func NewManager(signingKey string, timeout time.Duration) *Manager {
	m := &Manager{
		users:          make(map[string]*User),
		jwtSigningKey:  []byte(signingKey),
		SessionTimeout: timeout,
	}

	if err := m.loadUsers(); err != nil {
		adminUser := os.Getenv("ADMIN_USERNAME")
		if adminUser == "" {
			adminUser = "admin"
		}
		adminPass := os.Getenv("ADMIN_PASSWORD")
		if adminPass == "" {
			adminPass = "admin123"
		}

		// Create default admin if persistence fails/doesn't exist
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(adminPass), bcrypt.DefaultCost)
		if err != nil {
			// This should practically never happen with DefaultCost
			panic("failed to generate default admin password hash: " + err.Error())
		}
		m.users[adminUser] = &User{
			Username: adminUser,
			Password: string(hashedPassword),
			IsAdmin:  true,
		}
		m.saveUsers()
	}

	return m
}

func (m *Manager) saveUsers() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	data, err := json.MarshalIndent(m.users, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(usersFile, data, 0600)
}

func (m *Manager) loadUsers() error {
	data, err := os.ReadFile(usersFile)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return json.Unmarshal(data, &m.users)
}

func (m *Manager) HandleLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	m.mu.RLock()
	user, exists := m.users[req.Username]
	m.mu.RUnlock()

	if !exists || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	m.issueToken(c, user)
}

func (m *Manager) HandleCreateUser(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		IsAdmin  bool   `json:"is_admin"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	m.mu.Lock()
	if _, exists := m.users[req.Username]; exists {
		m.mu.Unlock()
		c.JSON(http.StatusConflict, gin.H{"error": "User exists"})
		return
	}
	m.mu.Unlock()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process password"})
		return
	}

	m.mu.Lock()
	m.users[req.Username] = &User{Username: req.Username, Password: string(hashedPassword), IsAdmin: req.IsAdmin}
	m.mu.Unlock()

	m.saveUsers()
	c.JSON(http.StatusCreated, gin.H{"status": "account created"})
}

func (m *Manager) HandleChangePassword(c *gin.Context) {
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	usernameRaw, _ := c.Get("username")
	username := usernameRaw.(string)

	m.mu.Lock()
	user, exists := m.users[username]
	if !exists {
		m.mu.Unlock()
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)) != nil {
		m.mu.Unlock()
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Incorrect current password"})
		return
	}

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	user.Password = string(hashedPassword)
	m.mu.Unlock()

	if err := m.saveUsers(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save user data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "password updated"})
}

func (m *Manager) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return m.jwtSigningKey, nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		if lastSeen, ok := m.lastActivity.Load(tokenStr); ok {
			if time.Since(lastSeen.(time.Time)) > m.SessionTimeout {
				m.lastActivity.Delete(tokenStr)
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Session expired due to inactivity"})
				return
			}
		}
		m.lastActivity.Store(tokenStr, time.Now())

		c.Set("username", claims.Username)
		c.Set("is_admin", claims.IsAdmin)
		c.Next()
	}
}

func (m *Manager) AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, exists := c.Get("is_admin")
		if !exists || !isAdmin.(bool) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin required"})
			return
		}
		c.Next()
	}
}

// issueToken is an internal helper to generate the app's JWT
func (m *Manager) issueToken(c *gin.Context, user *User) {
	claims := &Claims{
		Username: user.Username,
		IsAdmin:  user.IsAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(12 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(m.jwtSigningKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	m.lastActivity.Store(tokenString, time.Now())

	c.JSON(http.StatusOK, gin.H{
		"token":    tokenString,
		"username": user.Username,
		"is_admin": user.IsAdmin,
	})
}
