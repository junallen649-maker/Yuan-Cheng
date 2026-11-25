package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"polaris-platform/internal/auth"
	"polaris-platform/internal/config"
	"polaris-platform/internal/handler"
	"polaris-platform/internal/model"

	"github.com/gin-gonic/gin"
)

//go:embed web_dist/*
var embeddedWeb embed.FS

func main() {
	cfg := config.Load()
	db, jwtSecret, err := model.InitDB(cfg.DBPath, cfg.AdminDefaultUser, cfg.AdminDefaultPass, cfg.JWTSecret)
	if err != nil {
		log.Fatalf("init db failed: %v", err)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery(), corsMiddleware())

	h := handler.New(db, cfg, jwtSecret)

	// Health check
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "service": "polaris-platform"})
	})

	// RustDesk / Polaris Client API
	api := r.Group("/api")
	{
		api.POST("/sysinfo", h.HandleSysinfo)
		api.POST("/sysinfo_ver", h.HandleSysinfoVer)
		api.POST("/heartbeat", h.HandleHeartbeat)
		api.POST("/audit/conn", h.HandleAuditConn)
		api.POST("/audit/outgoing", h.HandleAuditOutgoing)
		api.POST("/audit/file", h.HandleAuditFile)
		api.POST("/audit/alarm", h.HandleAuditAlarm)
		api.PUT("/audit", h.HandleUpdateAuditNote)

		api.GET("/login-options", h.HandleLoginOptions)
		api.POST("/login", h.HandleClientLogin)

		authedClient := api.Group("")
		authedClient.Use(auth.RequireUser(db, jwtSecret))
		{
			authedClient.POST("/currentUser", h.HandleCurrentUser)
			authedClient.POST("/logout", h.HandleClientLogout)
			authedClient.GET("/device-group/accessible", h.HandleAccessibleGroups)
			authedClient.GET("/users", h.HandleClientUsers)
			authedClient.GET("/peers", h.HandleClientPeers)
			authedClient.GET("/ab", h.HandleGetAB)
			authedClient.POST("/ab", h.HandleSaveAB)
			authedClient.POST("/ab/settings", h.HandleABSettings)
			authedClient.POST("/ab/personal", h.HandleABPersonal)
			authedClient.POST("/ab/shared/profiles", h.HandleABSharedProfiles)
		}

		// Web Admin Dashboard API
		admin := api.Group("/admin")
		{
			admin.POST("/login", h.HandleAdminLogin)

			protected := admin.Group("")
			protected.Use(auth.RequireUser(db, jwtSecret))
			{
				protected.GET("/me", h.HandleAdminMe)
				protected.POST("/change-password", h.HandleChangePassword)
				protected.GET("/overview", h.HandleOverview)
				protected.GET("/devices", h.HandleListDevices)
				protected.GET("/sessions", h.HandleLiveSessions)
				protected.GET("/groups", h.HandleListGroups)
				protected.GET("/audits", h.HandleListAudits)
				protected.GET("/settings", h.HandleGetSettings)

				// Write operations require admin or teacher (users/settings require admin)
				protected.PUT("/devices/:id", h.HandleUpdateDevice)
				protected.POST("/devices/batch-group", h.HandleBatchGroupDevices)
				protected.POST("/devices/:id/disconnect", h.HandleDisconnectDevice)

				adminOnly := protected.Group("")
				adminOnly.Use(auth.RequireAdmin())
				{
					adminOnly.DELETE("/devices/:id", h.HandleDeleteDevice)
					adminOnly.POST("/groups", h.HandleCreateGroup)
					adminOnly.PUT("/groups/:id", h.HandleUpdateGroup)
					adminOnly.DELETE("/groups/:id", h.HandleDeleteGroup)
					adminOnly.GET("/users", h.HandleListUsers)
					adminOnly.POST("/users", h.HandleCreateUser)
					adminOnly.PUT("/users/:id", h.HandleUpdateUser)
					adminOnly.DELETE("/users/:id", h.HandleDeleteUser)
					adminOnly.PUT("/settings", h.HandleUpdateSettings)
				}
			}
		}
	}

	// Serve embedded Vue 3 SPA
	subFS, err := fs.Sub(embeddedWeb, "web_dist")
	if err != nil {
		log.Fatalf("embedded web fs error: %v", err)
	}
	fileServer := http.FileServer(http.FS(subFS))
	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "API endpoint not found"})
			return
		}
		trimmed := strings.TrimPrefix(p, "/")
		if trimmed == "" {
			trimmed = "index.html"
		}
		if _, err := fs.Stat(subFS, trimmed); err == nil {
			fileServer.ServeHTTP(c.Writer, c.Request)
			return
		}
		// SPA fallback
		indexData, err := fs.ReadFile(subFS, "index.html")
		if err != nil {
			c.String(http.StatusNotFound, "Frontend not built")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
	})

	addr := ":" + cfg.Port
	log.Printf("Polaris School Platform listening on %s (DB: %s)", addr, cfg.DBPath)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
