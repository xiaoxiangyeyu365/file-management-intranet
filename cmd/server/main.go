// cmd/server/main.go
package main

import (
	"cloudbox/internal/config"
	"cloudbox/internal/handler"
	"cloudbox/internal/repository"
	"cloudbox/internal/service"
	"cloudbox/internal/util/storage"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
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

	// Initialize services
	authService := service.NewAuthService(userRepo)
	fileService := service.NewFileService(fileRepo, physicalRepo, storageManager)
	uploadService := service.NewUploadService(fileRepo, physicalRepo, storageManager)

	// Initialize handlers
	authHandler := handler.NewAuthHandler(authService)
	fileHandler := handler.NewFileHandler(fileService)
	trashHandler := handler.NewTrashHandler(fileService)
	uploadHandler := handler.NewUploadHandler(uploadService)

	// Setup Gin
	r := gin.Default()

	// API routes
	api := r.Group("/api")
	{
		// Auth routes (no JWT required)
		auth := api.Group("/auth")
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
			protected.GET("/files/lookup", fileHandler.LookupFile)
			protected.GET("/files/:id", fileHandler.GetFile)
			protected.PUT("/files/:id", fileHandler.RenameFile)
			protected.DELETE("/files/:id", fileHandler.DeleteFile)
			protected.PATCH("/files/move", fileHandler.MoveFiles)

			// Folders
			protected.POST("/folders", fileHandler.CreateFolder)

			// Trash
			protected.GET("/trash", trashHandler.ListTrash)
			protected.POST("/trash/:id/restore", trashHandler.RestoreFile)
			protected.DELETE("/trash/:id", trashHandler.PermanentDelete)

			// Upload
			protected.POST("/upload/init", uploadHandler.InitUpload)
			protected.PUT("/upload/:uploadID/chunk/:index", uploadHandler.UploadChunk)
			protected.GET("/upload/:uploadID/progress", uploadHandler.GetProgress)
			protected.POST("/upload/:uploadID/complete", uploadHandler.CompleteUpload)
			protected.DELETE("/upload/:uploadID", uploadHandler.CancelUpload)
		}
	}

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Server starting at http://%s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
