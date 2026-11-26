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

	// Polaris Client API
	api := r.Group("/api")
	{
		api.POST("/sysinfo", h.HandleSysinfo)
		api.POST("/sysinfo_ver", h.HandleSysinfoVer)
		api.POST("/heartbeat", h.HandleHeartbeat)
		api.POST("/command/ack", h.HandleCommandAck)
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
				protected.GET("/schools", h.HandleListSchools)
				protected.GET("/devices", h.HandleListDevices)
				protected.GET("/sessions", h.HandleLiveSessions)
				protected.GET("/groups", h.HandleListGroups)
				protected.GET("/commands", h.HandleListCommands)
				protected.GET("/audits", h.HandleListAudits)
				protected.GET("/settings", h.HandleGetSettings)

				// Write operations scoped by user's school_id (super-admin can access all)
				protected.PUT("/devices/:id", h.HandleUpdateDevice)
				protected.POST("/devices/batch-group", h.HandleBatchGroupDevices)
				protected.POST("/devices/:id/disconnect", h.HandleDisconnectDevice)
				protected.POST("/devices/:id/command", h.HandleSendDeviceCommand)
				protected.POST("/devices/assign-ids", h.HandleAssignIDs)
				protected.POST("/devices/batch-password", h.HandleBatchPassword)
				protected.POST("/groups", h.HandleCreateGroup)
				protected.PUT("/groups/:id", h.HandleUpdateGroup)
				protected.DELETE("/groups/:id", h.HandleDeleteGroup)

				adminOnly := protected.Group("")
				adminOnly.Use(auth.RequireAdmin())
				{
					adminOnly.POST("/schools", h.HandleCreateSchool)
					adminOnly.PUT("/schools/:id", h.HandleUpdateSchool)
					adminOnly.DELETE("/schools/:id", h.HandleDeleteSchool)
					adminOnly.DELETE("/devices/:id", h.HandleDeleteDevice)
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
