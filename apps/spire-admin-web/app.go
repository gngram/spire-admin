package main

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gngram/spire_admin/auth"
	"github.com/gngram/spire_admin/servers"
)

type WebApp struct {
	mu           sync.RWMutex
	servers      map[int]*servers.SpireServer
	Auth         *auth.Manager
	nextServerID int
	socket       string
}

func NewWebApp(parentSocket string, sessionTimeout time.Duration) *WebApp {
	return &WebApp{
		servers:      make(map[int]*servers.SpireServer),
		nextServerID: 1,
		socket:       parentSocket,
		Auth:         auth.NewManager("SECURE_RANDOM_KEY", sessionTimeout),
	}
}

func run(
	parentSocket string,
	sessionTimeout time.Duration,
	certFile string,
	keyFile string) {
	app := NewWebApp(parentSocket, sessionTimeout)
	r := gin.Default()

	r.Static("/app", "./web")

	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/app/login.html")
	})

	api := r.Group("/api")
	{
		api.POST("/login", app.Auth.HandleLogin)

		authGroup := api.Group("/")
		authGroup.Use(app.Auth.AuthMiddleware())
		{
			authGroup.GET("/servers", app.handleGetServers)
			authGroup.POST("/servers", app.handleAddServer)
			authGroup.GET("/servers/:id/agents", app.handleGetAgents)
			authGroup.GET("/servers/:id/entries", app.handleGetEntries)
			authGroup.POST("/change-password", app.Auth.HandleChangePassword)
			authGroup.POST("/users", app.Auth.AdminMiddleware(), app.Auth.HandleCreateUser)
		}
	}

	if certFile != "" && keyFile != "" {
		r.RunTLS(":8443", certFile, keyFile)
	} else {
		r.Run(":8080")
	}
}

func (a *WebApp) handleAddServer(c *gin.Context) {
	var req struct {
		Name    string `json:"name"`
		Address string `json:"address"`
		Port    string `json:"port"`
	}
	c.ShouldBindJSON(&req)
	srv, err := servers.NewSpireServer(req.Name, req.Address, req.Port, a.socket)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	a.mu.Lock()
	id := a.nextServerID
	a.servers[id] = srv
	a.nextServerID++
	a.mu.Unlock()
	c.JSON(http.StatusOK, gin.H{"id": id, "status": "added"})
}

func (a *WebApp) handleGetServers(c *gin.Context) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := []map[string]interface{}{}
	for id, s := range a.servers {
		result = append(result, map[string]interface{}{"id": id, "name": s.Nickname, "address": s.Address, "domain": s.Domain, "status": "online"})
	}
	c.JSON(http.StatusOK, result)
}

func (a *WebApp) handleGetAgents(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	agents, err := srv.ListAgents(ctx, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, agents)
}

func (a *WebApp) handleGetEntries(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	a.mu.RLock()
	srv, exists := a.servers[id]
	a.mu.RUnlock()
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	entries, err := srv.ListEntries(ctx, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, entries)
}
