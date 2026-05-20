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
	"fmt"
	"log"
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

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	fileRepo := repository.NewFileRepository(db)
	physicalRepo := repository.NewPhysicalFileRepository(db)
	clipboardRepo := repository.NewClipboardRepository(db)

	// Initialize services
	authService := service.NewAuthService(userRepo)
	previewService := service.NewPreviewService(physicalRepo, fileRepo, storageManager)
	fileService := service.NewFileService(fileRepo, physicalRepo, storageManager)
	uploadService := service.NewUploadService(fileRepo, physicalRepo, storageManager, previewService)
	clipboardService := service.NewClipboardService(clipboardRepo)

	// Initialize handlers
	authHandler := handler.NewAuthHandler(authService)
	fileHandler := handler.NewFileHandler(fileService)
	previewHandler := handler.NewPreviewHandler(previewService, fileService)
	trashHandler := handler.NewTrashHandler(fileService)
	uploadHandler := handler.NewUploadHandler(uploadService)
	clipboardHandler := handler.NewClipboardHandler(clipboardService)

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

	// Rate limiting for auth routes (100 requests per minute per IP)
	auth := api.Group("/auth")
	auth.Use(middleware.RateLimit(100, time.Minute))
	{
		auth.POST("/login", authHandler.Login)
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
		protected.GET("/files/search", fileHandler.SearchFiles)  // Must be before /files/:id
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
	log.Printf("Server starting at http://%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
