# CloudBox 文件搜索与预览功能实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 CloudBox 后端添加文件搜索和图片元数据预览功能。

**Architecture:** 文件搜索使用 SQLite 递归 CTE 实现子文件夹搜索，相关性排序使用 SQL CASE 表达式。预览服务作为独立模块，合并缩略图生成和元数据提取，使用数据库乐观锁控制并发。

**Tech Stack:** Go 1.21+, Gin, GORM, SQLite, go-exif/v3

---

## File Structure

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/model/physical_file.go` | Modify | 新增 Width, Height, MetadataJSON 字段 |
| `internal/repository/file.go` | Modify | 新增 Search 方法 |
| `internal/repository/physical_file.go` | Modify | 新增 UpdateMetadata, UpdateImageInfo 方法 |
| `internal/service/file.go` | Modify | 新增 SearchFiles 方法 |
| `internal/service/preview.go` | Create | 新建预览服务 |
| `internal/service/upload.go` | Modify | 上传完成后调用预览服务 |
| `internal/handler/file.go` | Modify | 新增 SearchFiles handler |
| `internal/handler/preview.go` | Create | 新建预览 handler |
| `cmd/server/main.go` | Modify | 注册新路由、初始化 PreviewService |
| `go.mod` | Modify | 添加 EXIF 解析库 |

---

## Task 1: 数据库模型修改

**Files:**
- Modify: `internal/model/physical_file.go`

- [ ] **Step 1: 添加元数据字段**

在 `internal/model/physical_file.go` 的 `PhysicalFile` 结构体中添加字段：

```go
package model

import "time"

type PhysicalFile struct {
	ID            int64     `gorm:"primaryKey" json:"id"`
	StoragePath   string    `gorm:"uniqueIndex;size:500;not null" json:"storagePath"`
	MD5           string    `gorm:"uniqueIndex;size:32;not null" json:"md5"`
	Size          int64     `gorm:"not null" json:"size"`
	MimeType      string    `gorm:"size:100" json:"mimeType"`
	RefCount      int       `gorm:"default:1" json:"refCount"`
	ThumbnailPath string    `gorm:"size:500" json:"thumbnailPath"`
	Width         int       `gorm:"default:0" json:"width"`           // 图片宽度
	Height        int       `gorm:"default:0" json:"height"`          // 图片高度
	MetadataJSON  string    `gorm:"type:text" json:"metadataJson"`    // EXIF 元数据 JSON
	CreatedAt     time.Time `json:"createdAt"`
}

func (PhysicalFile) TableName() string {
	return "physical_files"
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/model/physical_file.go
git commit -m "feat: add image metadata fields to PhysicalFile model"
```

---

## Task 2: Repository 层 - 搜索方法

**Files:**
- Modify: `internal/repository/file.go`

- [ ] **Step 1: 添加 Search 方法**

在 `internal/repository/file.go` 末尾添加：

```go
// Search searches files by keyword with optional folder scope and sorting
func (r *FileRepository) Search(ctx context.Context, userID int64, keyword string, folderID *int64, sort string) ([]model.File, error) {
	var files []model.File

	// Build base query
	if folderID == nil {
		// Global search
		query := r.db.WithContext(ctx).
			Where("owner_id = ? AND deleted_at IS NULL AND name LIKE ?", userID, "%"+keyword+"%")
		query = r.applySearchSort(query, sort, keyword)
		err := query.Find(&files).Error
		return files, err
	}

	// Recursive search in folder and subfolders
	query := `
		WITH RECURSIVE subfolders AS (
			SELECT id FROM files
			WHERE id = ? AND owner_id = ? AND is_folder = 1 AND deleted_at IS NULL
			UNION ALL
			SELECT f.id FROM files f
			JOIN subfolders s ON f.parent_id = s.id
			WHERE f.owner_id = ? AND f.is_folder = 1 AND f.deleted_at IS NULL
		)
		SELECT f.* FROM files f
		JOIN subfolders s ON f.parent_id = s.id
		WHERE f.owner_id = ?
		  AND f.deleted_at IS NULL
		  AND f.name LIKE ?
	`

	// Add ORDER BY clause
	orderBy := r.buildSearchOrderBy(sort, keyword)
	query += orderBy

	err := r.db.WithContext(ctx).Raw(query, *folderID, userID, userID, userID, "%"+keyword+"%").Scan(&files).Error
	return files, err
}

func (r *FileRepository) applySearchSort(query *gorm.DB, sort, keyword string) *gorm.DB {
	switch sort {
	case "time":
		return query.Order("updated_at DESC")
	case "name":
		return query.Order("name ASC")
	default: // relevance
		// Use raw SQL with escaped keyword for relevance sorting
		escapedKeyword := strings.ReplaceAll(keyword, "'", "''")
		return query.Order(fmt.Sprintf(
			"CASE WHEN name = '%s' THEN 0 WHEN name LIKE '%s%%' THEN 1 ELSE 2 END, name ASC",
			escapedKeyword, escapedKeyword))
	}
}

func (r *FileRepository) buildSearchOrderBy(sort, keyword string) string {
	switch sort {
	case "time":
		return " ORDER BY updated_at DESC"
	case "name":
		return " ORDER BY name ASC"
	default: // relevance - escape keyword to prevent SQL injection
		escapedKeyword := strings.ReplaceAll(keyword, "'", "''")
		return fmt.Sprintf(" ORDER BY CASE WHEN name = '%s' THEN 0 WHEN name LIKE '%s%%' THEN 1 ELSE 2 END, name ASC",
			escapedKeyword, escapedKeyword)
	}
}
```

- [ ] **Step 2: 添加 fmt import**

确保 import 包含 `"fmt"` 和 `"strings"`：

```go
import (
	"cloudbox/internal/model"
	"context"
	"database/sql"
	"fmt"
	"strings"

	"gorm.io/gorm"
)
```

- [ ] **Step 3: Commit**

```bash
git add internal/repository/file.go
git commit -m "feat: add Search method with recursive CTE for folder search"
```

---

## Task 3: Repository 层 - 元数据更新

**Files:**
- Modify: `internal/repository/physical_file.go`

- [ ] **Step 1: 添加元数据更新方法**

在 `internal/repository/physical_file.go` 末尾添加：

```go
// UpdateImageInfo updates width, height, and optionally metadata in one call
func (r *PhysicalFileRepository) UpdateImageInfo(ctx context.Context, id int64, width, height int, metadataJSON string) error {
	updates := map[string]interface{}{
		"width":  width,
		"height": height,
	}
	if metadataJSON != "" {
		updates["metadata_json"] = metadataJSON
	}
	return r.db.WithContext(ctx).
		Model(&model.PhysicalFile{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// UpdateMetadataWithOptimisticLock updates metadata only if it's currently NULL
// Returns true if updated, false if already set by another process
func (r *PhysicalFileRepository) UpdateMetadataWithOptimisticLock(ctx context.Context, id int64, metadataJSON string) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&model.PhysicalFile{}).
		Where("id = ? AND (metadata_json IS NULL OR metadata_json = '')", id).
		Update("metadata_json", metadataJSON)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/repository/physical_file.go
git commit -m "feat: add UpdateImageInfo and UpdateMetadataWithOptimisticLock methods"
```

---

## Task 4: Service 层 - 搜索服务

**Files:**
- Modify: `internal/service/file.go`

- [ ] **Step 1: 添加 SearchFiles 方法**

在 `internal/service/file.go` 末尾添加：

```go
// SearchFiles searches files by keyword with optional folder scope and sorting
func (s *FileService) SearchFiles(ctx context.Context, userID int64, keyword string, folderID *int64, sort string) ([]model.File, error) {
	if keyword == "" {
		return nil, errors.New("keyword is required")
	}

	// Validate sort parameter
	validSorts := map[string]bool{"relevance": true, "time": true, "name": true}
	if sort == "" {
		sort = "relevance"
	}
	if !validSorts[sort] {
		sort = "relevance"
	}

	return s.fileRepo.Search(ctx, userID, keyword, folderID, sort)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/service/file.go
git commit -m "feat: add SearchFiles service method"
```

---

## Task 5: Service 层 - 预览服务（核心）

**Files:**
- Create: `internal/service/preview.go`

- [ ] **Step 1: 创建预览服务文件**

创建 `internal/service/preview.go`：

```go
// internal/service/preview.go
package service

import (
	"cloudbox/internal/model"
	"cloudbox/internal/repository"
	"cloudbox/internal/util/storage"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dsoprea/go-exif/v3"
	exifcommon "github.com/dsoprea/go-exif/v3/common"
)

var (
	ErrNotImage       = errors.New("file is not an image")
	ErrMetadataFailed = errors.New("failed to extract metadata")
)

type ImageMetadata struct {
	Width  int            `json:"width"`
	Height int            `json:"height"`
	EXIF   map[string]any `json:"exif,omitempty"`
}

type PreviewService struct {
	physicalRepo *repository.PhysicalFileRepository
	fileRepo     *repository.FileRepository
	storage      *storage.StorageManager
}

func NewPreviewService(
	physicalRepo *repository.PhysicalFileRepository,
	fileRepo *repository.FileRepository,
	storage *storage.StorageManager,
) *PreviewService {
	return &PreviewService{
		physicalRepo: physicalRepo,
		fileRepo:     fileRepo,
		storage:      storage,
	}
}

// ProcessImage processes an image after upload: extract dimensions, generate thumbnail, extract EXIF
func (s *PreviewService) ProcessImage(ctx context.Context, physicalID int64) error {
	pf, err := s.physicalRepo.FindByID(ctx, physicalID)
	if err != nil {
		return fmt.Errorf("failed to find physical file: %w", err)
	}

	// Check if image type
	if !isImageType(pf.MimeType) {
		return nil // Not an image, skip processing
	}

	absPath := s.storage.ToAbsPath(pf.StoragePath)

	// 1. Get image dimensions
	width, height, err := s.getImageDimensions(absPath)
	if err != nil {
		log.Printf("warning: failed to get image dimensions for %d: %v", physicalID, err)
	}

	// 2. Extract EXIF metadata
	metadataJSON := ""
	if exifData, err := s.extractEXIF(absPath); err == nil {
		if jsonData, err := json.Marshal(exifData); err == nil {
			metadataJSON = string(jsonData)
		}
	}

	// 3. Generate thumbnail if needed
	thumbnailPath := s.storage.ThumbnailPath(pf.ID)
	if _, err := os.Stat(thumbnailPath); os.IsNotExist(err) {
		if err := s.generateThumbnail(pf, absPath, thumbnailPath, width, height); err != nil {
			log.Printf("warning: failed to generate thumbnail for %d: %v", physicalID, err)
		}
	}

	// 4. Update database with all info
	if width > 0 && height > 0 {
		if err := s.physicalRepo.UpdateImageInfo(ctx, physicalID, width, height, metadataJSON); err != nil {
			log.Printf("warning: failed to update image info for %d: %v", physicalID, err)
		}
	}

	return nil
}

// GetMetadata returns image metadata, extracting on-demand if needed
func (s *PreviewService) GetMetadata(ctx context.Context, userID, fileID int64) (*ImageMetadata, error) {
	// Get file and verify ownership
	file, err := s.fileRepo.FindByIDAndOwner(ctx, fileID, userID)
	if err != nil {
		return nil, ErrFileNotFound
	}

	if file.IsFolder || !file.PhysicalID.Valid {
		return nil, ErrNotImage
	}

	pf, err := s.physicalRepo.FindByID(ctx, file.PhysicalID.Int64)
	if err != nil {
		return nil, fmt.Errorf("failed to find physical file: %w", err)
	}

	if !isImageType(pf.MimeType) {
		return nil, ErrNotImage
	}

	metadata := &ImageMetadata{
		Width:  pf.Width,
		Height: pf.Height,
	}

	// If metadata exists, parse it
	if pf.MetadataJSON != "" {
		if err := json.Unmarshal([]byte(pf.MetadataJSON), &metadata.EXIF); err != nil {
			log.Printf("warning: failed to parse metadata JSON: %v", err)
		}
		return metadata, nil
	}

	// Extract on-demand
	absPath := s.storage.ToAbsPath(pf.StoragePath)

	// Get dimensions if not set
	if pf.Width == 0 || pf.Height == 0 {
		width, height, err := s.getImageDimensions(absPath)
		if err != nil {
			return nil, err
		}
		metadata.Width = width
		metadata.Height = height
	}

	// Extract EXIF
	exifData, err := s.extractEXIF(absPath)
	if err != nil {
		metadata.EXIF = exifData
	}

	// Try to save with optimistic lock
	if jsonData, err := json.Marshal(metadata.EXIF); err == nil {
		updated, err := s.physicalRepo.UpdateMetadataWithOptimisticLock(ctx, pf.ID, string(jsonData))
		if err != nil {
			log.Printf("warning: failed to update metadata: %v", err)
		}
		if !updated {
			// Another process updated it, fetch the latest
			pf, _ = s.physicalRepo.FindByID(ctx, pf.ID)
			if pf.MetadataJSON != "" {
				json.Unmarshal([]byte(pf.MetadataJSON), &metadata.EXIF)
			}
		}
	}

	return metadata, nil
}

func (s *PreviewService) getImageDimensions(path string) (int, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}
	return config.Width, config.Height, nil
}

func (s *PreviewService) extractEXIF(path string) (map[string]any, error) {
	data, err := exif.SearchFileAndExtractExif(path)
	if err != nil {
		return nil, err
	}

	im, err := exifcommon.NewIfdMappingWithStandard()
	if err != nil {
		return nil, err
	}

	ti := exif.NewTagIndex()
	_, index, err := exif.Collect(im, ti, data)
	if err != nil {
		return nil, err
	}

	result := make(map[string]any)

	// Walk through all tags
	exif.Walk(index, func(ifd *exif.Ifd, ite *exif.IfdTagEntry) error {
		tagName, err := ite.TagName()
		if err != nil {
			return nil
		}

		value, err := ifd.TagValue(ite)
		if err != nil {
			return nil
		}

		// Format specific tags
		switch v := value.(type) {
		case []byte:
			if len(v) < 100 {
				result[tagName] = fmt.Sprintf("%x", v)
			}
		case []interface{}:
			if len(v) <= 4 {
				result[tagName] = v
			}
		default:
			result[tagName] = v
		}
		return nil
	})

	return result, nil
}

func (s *PreviewService) generateThumbnail(pf *model.PhysicalFile, srcPath, thumbPath string, width, height int) error {
	const thumbnailSize = 200
	const maxLockAge = 30 * time.Second

	// Ensure directory exists
	if err := s.storage.EnsureParentDir(thumbPath); err != nil {
		return err
	}

	// Lock file for concurrency safety
	lockPath := thumbPath + ".lock"
	for i := 0; i < 10; i++ {
		f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			defer os.Remove(lockPath)
			defer f.Close()
			break
		}

		if info, statErr := os.Stat(lockPath); statErr == nil {
			if time.Since(info.ModTime()) > maxLockAge {
				os.Remove(lockPath)
				continue
			}
		}

		if _, err := os.Stat(thumbPath); err == nil {
			return nil // Already generated
		}

		if i < 9 {
			time.Sleep(100 * time.Millisecond)
		} else {
			return fmt.Errorf("failed to acquire lock")
		}
	}

	// Open source image
	file, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var img image.Image
	switch pf.MimeType {
	case "image/png":
		img, err = png.Decode(file)
	case "image/gif":
		img, err = gif.Decode(file)
	default:
		img, err = jpeg.Decode(file)
	}
	if err != nil {
		return err
	}

	// Resize
	resized := resizeImage(img, thumbnailSize)

	// Save thumbnail
	outFile, err := os.Create(thumbPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	if err := jpeg.Encode(outFile, resized, &jpeg.Options{Quality: 85}); err != nil {
		return err
	}

	// Update database
	return s.physicalRepo.UpdateThumbnail(context.Background(), pf.ID, thumbPath)
}

func resizeImage(img image.Image, maxSize int) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width <= maxSize && height <= maxSize {
		return img
	}

	var newWidth, newHeight int
	if width > height {
		newWidth = maxSize
		newHeight = height * maxSize / width
	} else {
		newHeight = maxSize
		newWidth = width * maxSize / height
	}

	if newWidth < 1 {
		newWidth = 1
	}
	if newHeight < 1 {
		newHeight = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			srcX := x * width / newWidth
			srcY := y * height / newHeight
			dst.Set(x, y, img.At(srcX+bounds.Min.X, srcY+bounds.Min.Y))
		}
	}
	return dst
}

func isImageType(mimeType string) bool {
	imageTypes := []string{"image/jpeg", "image/png", "image/gif"}
	for _, t := range imageTypes {
		if mimeType == t {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/service/preview.go
git commit -m "feat: create PreviewService with image processing and EXIF extraction"
```

---

## Task 6: 修改上传服务集成预览

**Files:**
- Modify: `internal/service/upload.go`

- [ ] **Step 1: 添加 PreviewService 依赖**

修改 `internal/service/upload.go`：

```go
type UploadService struct {
	fileRepo       *repository.FileRepository
	physicalRepo   *repository.PhysicalFileRepository
	storage        *storage.StorageManager
	chunkSize      int64
	previewService *PreviewService
}

func NewUploadService(
	fileRepo *repository.FileRepository,
	physicalRepo *repository.PhysicalFileRepository,
	storage *storage.StorageManager,
	previewService *PreviewService,
) *UploadService {
	cfg := config.Get()
	return &UploadService{
		fileRepo:       fileRepo,
		physicalRepo:   physicalRepo,
		storage:        storage,
		chunkSize:      cfg.Upload.ChunkSize,
		previewService: previewService,
	}
}
```

- [ ] **Step 2: 修改 CompleteUpload 调用预览服务**

找到 `CompleteUpload` 方法末尾的 `// Generate thumbnail asynchronously` 注释，替换为：

```go
	// Process image asynchronously (thumbnail + metadata)
	if s.previewService != nil {
		go s.previewService.ProcessImage(context.Background(), pf.ID)
	}
```

- [ ] **Step 3: 删除旧的 generateThumbnail 占位方法**

删除或替换 `generateThumbnail` 方法：

```go
// generateThumbnail is now handled by PreviewService
```

- [ ] **Step 4: Commit**

```bash
git add internal/service/upload.go
git commit -m "feat: integrate PreviewService into upload flow"
```

---

## Task 7: Handler 层 - 搜索接口

**Files:**
- Modify: `internal/handler/file.go`

- [ ] **Step 1: 添加 SearchFiles handler**

在 `internal/handler/file.go` 末尾添加：

```go
func (h *FileHandler) SearchFiles(c *gin.Context) {
	userID := GetUserID(c)

	keyword := c.Query("keyword")
	if keyword == "" {
		response.BadRequest(c, "keyword is required")
		return
	}

	var folderID *int64
	if folderIDStr := c.Query("folderId"); folderIDStr != "" {
		id, err := strconv.ParseInt(folderIDStr, 10, 64)
		if err != nil {
			response.BadRequest(c, "invalid folderId")
			return
		}
		folderID = &id
	}

	sort := c.Query("sort")

	files, err := h.fileService.SearchFiles(c.Request.Context(), userID, keyword, folderID, sort)
	if err != nil {
		response.InternalError(c, "failed to search files")
		return
	}

	response.Success(c, gin.H{
		"files": files,
	})
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/handler/file.go
git commit -m "feat: add SearchFiles handler"
```

---

## Task 8: Handler 层 - 预览接口

**Files:**
- Create: `internal/handler/preview.go`

- [ ] **Step 1: 创建预览 handler 文件**

创建 `internal/handler/preview.go`：

```go
// internal/handler/preview.go
package handler

import (
	"cloudbox/internal/service"
	"cloudbox/internal/util/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type PreviewHandler struct {
	previewService *service.PreviewService
	fileService    *service.FileService
}

func NewPreviewHandler(previewService *service.PreviewService, fileService *service.FileService) *PreviewHandler {
	return &PreviewHandler{
		previewService: previewService,
		fileService:    fileService,
	}
}

func (h *PreviewHandler) GetMetadata(c *gin.Context) {
	userID := GetUserID(c)
	fileID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid file id")
		return
	}

	metadata, err := h.previewService.GetMetadata(c.Request.Context(), userID, fileID)
	if err != nil {
		if err == service.ErrFileNotFound {
			response.NotFound(c, "file not found")
			return
		}
		if err == service.ErrNotImage {
			response.Error(c, 400, "file is not an image")
			return
		}
		response.InternalError(c, "failed to get metadata")
		return
	}

	response.Success(c, metadata)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/handler/preview.go
git commit -m "feat: add PreviewHandler with GetMetadata endpoint"
```

---

## Task 9: 添加 EXIF 依赖

**Files:**
- Modify: `go.mod`

- [ ] **Step 1: 添加 go-exif 依赖**

```bash
cd E:/fileManagementIntranet && go get github.com/dsoprea/go-exif/v3@latest
```

- [ ] **Step 2: 整理依赖**

```bash
go mod tidy
```

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "deps: add go-exif for EXIF metadata extraction"
```

---

## Task 10: 注册路由和初始化服务

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: 初始化 PreviewService**

在 `cmd/server/main.go` 的服务初始化部分修改：

```go
	// Initialize services
	authService := service.NewAuthService(userRepo)
	previewService := service.NewPreviewService(physicalRepo, fileRepo, storageManager)
	fileService := service.NewFileService(fileRepo, physicalRepo, storageManager)
	uploadService := service.NewUploadService(fileRepo, physicalRepo, storageManager, previewService)
```

- [ ] **Step 2: 初始化 PreviewHandler**

在 handler 初始化部分添加：

```go
	// Initialize handlers
	authHandler := handler.NewAuthHandler(authService)
	fileHandler := handler.NewFileHandler(fileService)
	trashHandler := handler.NewTrashHandler(fileService)
	uploadHandler := handler.NewUploadHandler(uploadService)
	previewHandler := handler.NewPreviewHandler(previewService, fileService)
```

- [ ] **Step 3: 注册新路由**

在 protected 路由组中添加：

```go
			// Files
			protected.GET("/files", fileHandler.ListFiles)
			protected.GET("/files/search", fileHandler.SearchFiles)
			protected.GET("/files/lookup", fileHandler.LookupFile)
			protected.GET("/files/:id", fileHandler.GetFile)
			protected.GET("/files/:id/download", fileHandler.DownloadFile)
			protected.GET("/files/:id/thumbnail", fileHandler.GetThumbnail)
			protected.GET("/files/:id/metadata", previewHandler.GetMetadata)
			protected.PUT("/files/:id", fileHandler.RenameFile)
			protected.DELETE("/files/:id", fileHandler.DeleteFile)
			protected.PATCH("/files/move", fileHandler.MoveFiles)
```

**注意：** `/files/search` 必须在 `/files/:id` 之前注册，否则 "search" 会被当作 id 解析。

- [ ] **Step 4: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: register search and preview routes, initialize PreviewService"
```

---

## Task 11: 构建和测试

**Files:**
- None

- [ ] **Step 1: 构建项目**

```bash
cd E:/fileManagementIntranet && go build -o cloudbox.exe ./cmd/server
```

- [ ] **Step 2: 启动服务器**

```bash
./cloudbox.exe
```

- [ ] **Step 3: 测试登录**

```bash
curl -X POST http://localhost:8080/api/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}'
```

- [ ] **Step 4: 测试搜索接口**

使用上一步获取的 token：

```bash
TOKEN="<token>"
curl "http://localhost:8080/api/files/search?keyword=test" -H "Authorization: Bearer $TOKEN"
```

- [ ] **Step 5: 测试元数据接口**

```bash
curl "http://localhost:8080/api/files/1/metadata" -H "Authorization: Bearer $TOKEN"
```

- [ ] **Step 6: 最终提交**

```bash
git add -A
git commit -m "feat: complete search and preview implementation"
```

---

## Self-Review Checklist

- [ ] 搜索支持全局和指定文件夹范围
- [ ] 搜索使用递归 CTE 而非应用层迭代
- [ ] 相关性排序使用 SQL CASE 表达式
- [ ] 预览服务合并缩略图生成和元数据提取
- [ ] 元数据更新使用数据库乐观锁
- [ ] 路由顺序正确（/files/search 在 /files/:id 之前）
- [ ] 无 placeholder 代码
