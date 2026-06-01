// cmd/server/main.go
package main

import (
	"cloudbox/internal/config"
	"cloudbox/internal/handler"
	"cloudbox/internal/middleware"
	"cloudbox/internal/repository"
	"cloudbox/internal/service"
	"cloudbox/internal/util/storage"
	"cloudbox/static"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "cloudbox/docs" // Swagger docs
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	db := repository.InitDB(cfg)

	// Initialize storage
	storageManager := storage.InitStorage(cfg)
	storage.StartTempCleanup(cfg.Upload.TempExpire)

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	fileRepo := repository.NewFileRepository(db)
	physicalRepo := repository.NewPhysicalFileRepository(db)
	clipboardRepo := repository.NewClipboardRepository(db)
	shareRepo := repository.NewShareRepository(db)

	// Initialize services
	cryptoAdapter := service.NewCryptoAdapter()

	// Initialize audit service
	auditRepo := repository.NewAuditRepository(db)
	auditService := service.NewAuditService(auditRepo)
	startAuditCleanup(auditRepo, cfg.Audit.RetentionDays)

	authService := service.NewAuthService(userRepo, cryptoAdapter, cryptoAdapter, cfg.Auth.Registration, cfg.Auth.ApprovalRequired, cfg.Admin.Password, auditService)
	previewService := service.NewPreviewService(physicalRepo, fileRepo, storageManager)
	fileService := service.NewFileService(fileRepo, physicalRepo, storageManager, auditService)
	uploadService := service.NewUploadService(fileRepo, physicalRepo, userRepo, storageManager, previewService, cfg.Upload.ChunkSize, cfg.Disk.DefaultQuota, auditService)
	clipboardService := service.NewClipboardService(clipboardRepo, auditService)
	shareService := service.NewShareService(shareRepo, fileRepo, physicalRepo, storageManager, fileService, cryptoAdapter, auditService)

	// Initialize handlers
	authHandler := handler.NewAuthHandler(authService)
	fileHandler := handler.NewFileHandler(fileService)
	previewHandler := handler.NewPreviewHandler(previewService, fileService)
	trashHandler := handler.NewTrashHandler(fileService)
	uploadHandler := handler.NewUploadHandler(uploadService)
	clipboardHandler := handler.NewClipboardHandler(clipboardService)

	// Initialize admin service and handler
	adminService := service.NewAdminService(userRepo, fileRepo, physicalRepo, clipboardRepo, fileService, cryptoAdapter, auditService, auditRepo)
	adminHandler := handler.NewAdminHandler(adminService)
	shareHandler := handler.NewShareHandler(shareService, fileService)
	storageHandler := handler.NewStorageHandler(physicalRepo, userRepo)

	// Setup Gin
	r := gin.Default()

	// CORS middleware for cross-origin requests
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Device-Name")
		c.Header("Access-Control-Expose-Headers", "Content-Disposition, Content-Length")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Request logging middleware
	r.Use(middleware.Logger())

	// Health check (no auth required)
	r.GET("/health", handler.HealthCheck)

	// API routes
	api := r.Group("/api")

	// Public share routes (no JWT required)
	sGroup := r.Group("/api/s")
	{
		sGroup.GET("/:token", shareHandler.GetShareInfo)
		sGroup.POST("/:token/verify", shareHandler.VerifyShare)
		sGroup.GET("/:token/download", shareHandler.DownloadByShare)
	}

	// Rate limiting for auth routes (100 requests per minute per IP)
	auth := api.Group("/auth")
	auth.Use(middleware.RateLimit(100, time.Minute))
	{
		auth.POST("/login", authHandler.Login)
		auth.POST("/register", authHandler.Register)
	}

	// Protected routes
	protected := api.Group("")
	protected.Use(handler.JWTMiddleware())
	{
		// Auth
		protected.POST("/auth/password", authHandler.ChangePassword)
		protected.POST("/auth/logout", authHandler.Logout)
		protected.GET("/auth/profile", authHandler.GetProfile)

		// Files
		protected.GET("/files", fileHandler.ListFiles)
		protected.GET("/files/search", fileHandler.SearchFiles)       // Must be before /files/:id
		protected.GET("/files/download", fileHandler.BatchDownload)    // Must be before /:id routes
		protected.GET("/files/lookup", fileHandler.LookupFile)
		protected.GET("/files/:id", fileHandler.GetFile)
		protected.GET("/files/:id/download", fileHandler.DownloadFile)
		protected.GET("/files/:id/thumbnail", fileHandler.GetThumbnail)
		protected.GET("/files/:id/metadata", previewHandler.GetMetadata)
		protected.PUT("/files/:id", fileHandler.RenameFile)
		protected.DELETE("/files/:id", fileHandler.DeleteFile)
		protected.PATCH("/files/move", fileHandler.MoveFiles)

		// Folders
		protected.POST("/folders", fileHandler.CreateFolder)
		protected.GET("/folders/:id/download", fileHandler.DownloadFolder)

		// Trash
		protected.GET("/trash", trashHandler.ListTrash)
		protected.POST("/trash/:id/restore", trashHandler.RestoreFile)
		protected.DELETE("/trash/:id", trashHandler.PermanentDelete)
		protected.DELETE("/trash", trashHandler.EmptyTrash)

		// Clipboard
		protected.GET("/clipboard", clipboardHandler.List)
		protected.POST("/clipboard", clipboardHandler.Create)
		protected.PATCH("/clipboard/:id/pin", clipboardHandler.UpdatePin)
		protected.DELETE("/clipboard/:id", clipboardHandler.Delete)
		protected.DELETE("/clipboard", clipboardHandler.Clear)

		// Upload
		protected.POST("/upload/init", uploadHandler.InitUpload)
		protected.PUT("/upload/:uploadID/chunk/:index", uploadHandler.UploadChunk)
		protected.GET("/upload/:uploadID/progress", uploadHandler.GetProgress)
		protected.POST("/upload/:uploadID/complete", uploadHandler.CompleteUpload)
		protected.DELETE("/upload/:uploadID", uploadHandler.CancelUpload)

		// Shares
		protected.POST("/shares", shareHandler.CreateShare)
		protected.GET("/shares", shareHandler.ListFileShares)
		protected.GET("/shares/mine", shareHandler.ListMyShares)
		protected.DELETE("/shares/:id", shareHandler.RevokeShare)

		// Storage
		protected.GET("/storage/usage", storageHandler.GetUsage)
		}

	// Admin routes
	admin := api.Group("/admin")
	admin.Use(handler.JWTMiddleware(), handler.AdminMiddleware())
	{
		admin.GET("/users", adminHandler.ListUsers)
		admin.POST("/users", adminHandler.CreateUser)
		admin.PUT("/users/:id", adminHandler.UpdateUser)
		admin.PUT("/users/:id/password", adminHandler.ResetPassword)
		admin.PUT("/users/:id/quota", adminHandler.UpdateUserQuota)
		admin.DELETE("/users/:id", adminHandler.DeleteUser)
		admin.GET("/audit-logs", adminHandler.ListAuditLogs)
	}

	// WebDAV routes — register all WebDAV methods explicitly
	// (r.Any only covers standard HTTP methods, not PROPFIND/MKCOL/MOVE/COPY/LOCK/UNLOCK)
	davHandler := handler.NewWebDAVHandler(fileService, uploadService, auditService, storageManager)
	davAuth := handler.BasicAuthMiddleware(authService, auditService)
	davServe := func(c *gin.Context) { davHandler.ServeHTTP(c.Writer, c.Request) }
	webdavMethods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS",
		"PROPFIND", "MKCOL", "MOVE", "COPY", "LOCK", "UNLOCK"}
	for _, method := range webdavMethods {
		r.Handle(method, "/dav/*path", davAuth, davServe)
	}

	// Serve static files (must be after API routes)
	r.Static("/assets", "./static/assets")

	// Swagger documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.NoRoute(func(c *gin.Context) {
		data, err := static.StaticFiles.ReadFile("index.html")
		if err != nil {
			c.String(404, "Frontend not found. Run 'make build-frontend' first.")
			return
		}
		c.Data(200, "text/html; charset=utf-8", data)
	})

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{Addr: addr, Handler: r}
	go func() {
		log.Printf("Server starting at http://%s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	auditService.Shutdown()
	log.Println("Server exited")
}

func startAuditCleanup(auditRepo *repository.AuditRepository, retentionDays int) {
	if retentionDays <= 0 {
		return
	}

	go func() {
		time.Sleep(2 * time.Minute)
		cleanupAuditLogs(auditRepo, retentionDays)

		for {
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
			time.Sleep(next.Sub(now))
			cleanupAuditLogs(auditRepo, retentionDays)
		}
	}()
}

func cleanupAuditLogs(auditRepo *repository.AuditRepository, retentionDays int) {
	before := time.Now().AddDate(0, 0, -retentionDays)
	affected, err := auditRepo.DeleteBefore(context.Background(), before)
	if err != nil {
		log.Printf("[audit-cleanup] error: %v", err)
		return
	}
	if affected > 0 {
		log.Printf("[audit-cleanup] removed %d audit log(s) older than %d days", affected, retentionDays)
	}
}
