# CloudBox 下载接口补充实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 补全 CloudBox 后端缺失的 4 个下载相关接口：文件下载、缩略图获取、文件夹ZIP下载、清空回收站。

**Architecture:** Handler → Service → Repository 三层架构，复用现有模式。缩略图按需生成，使用文件锁保证并发安全。ZIP流式生成，不支持断点续传。

**Tech Stack:** Go 1.21+, Gin, GORM, archive/zip, image/draw

---

## File Structure

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/service/file.go` | Modify | 新增 DownloadFile, GetThumbnail, StreamFolderZip, EmptyTrash |
| `internal/handler/file.go` | Modify | 新增 DownloadFile, GetThumbnail, DownloadFolder |
| `internal/handler/trash.go` | Modify | 新增 EmptyTrash |
| `cmd/server/main.go` | Modify | 注册新路由 |

**Note:** `physical_file.go` 已有 `UpdateThumbnail` 方法，无需修改。

---

## Task 1: 文件下载 Service 层

**Files:**
- Modify: `internal/service/file.go`

- [ ] **Step 1: 添加 DownloadFile 方法**

在 `internal/service/file.go` 末尾添加：

```go
// DownloadFile returns file and physical file info for download
func (s *FileService) DownloadFile(ctx context.Context, userID, fileID int64) (*model.File, *model.PhysicalFile, error) {
	file, err := s.fileRepo.FindByIDAndOwner(ctx, fileID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrFileNotFound
		}
		return nil, nil, fmt.Errorf("failed to find file: %w", err)
	}

	if file.IsFolder {
		return nil, nil, errors.New("cannot download a folder")
	}

	if !file.PhysicalID.Valid {
		return nil, nil, errors.New("file has no physical content")
	}

	pf, err := s.physicalRepo.FindByID(ctx, file.PhysicalID.Int64)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find physical file: %w", err)
	}

	return file, pf, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/service/file.go
git commit -m "feat: add DownloadFile service method"
```

---

## Task 2: 文件下载 Handler 层

**Files:**
- Modify: `internal/handler/file.go`

- [ ] **Step 1: 添加必要的 import**

在 `internal/handler/file.go` 的 import 中添加 `"net/url"`：

```go
import (
	"cloudbox/internal/service"
	"cloudbox/internal/util/response"
	"fmt"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
)
```

- [ ] **Step 2: 添加 DownloadFile handler**

在文件末尾添加：

```go
func (h *FileHandler) DownloadFile(c *gin.Context) {
	userID := GetUserID(c)
	fileID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	file, pf, err := h.fileService.DownloadFile(c.Request.Context(), userID, fileID)
	if err != nil {
		if err == service.ErrFileNotFound {
			response.NotFound(c, "file not found")
			return
		}
		response.Error(c, 400, err.Error())
		return
	}

	// RFC 5987 encoding for Chinese filename
	encodedName := url.PathEscape(file.Name)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", encodedName))
	c.Header("Content-Type", pf.MimeType)
	c.Header("Content-Length", fmt.Sprintf("%d", pf.Size))

	// Get absolute path and serve file
	absPath := h.fileService.GetStorage().ToAbsPath(pf.StoragePath)
	c.File(absPath)
}
```

- [ ] **Step 3: 在 FileService 添加 GetStorage 方法**

在 `internal/service/file.go` 的 FileService 结构体添加：

```go
func (s *FileService) GetStorage() *storage.StorageManager {
	return s.storage
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/handler/file.go internal/service/file.go
git commit -m "feat: add file download handler"
```

---

## Task 3: 缩略图获取 Service 层

**Files:**
- Modify: `internal/service/file.go`

- [ ] **Step 1: 添加必要的 import**

在 `internal/service/file.go` 的 import 中添加图片处理相关包：

```go
import (
	"cloudbox/internal/model"
	"cloudbox/internal/repository"
	"cloudbox/internal/util/storage"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)
```

- [ ] **Step 2: 添加缩略图相关错误和常量**

在现有 var 块中添加：

```go
var (
	ErrFileNotFound      = errors.New("file not found")
	ErrForbidden         = errors.New("access denied")
	ErrNameConflict      = errors.New("file name already exists")
	ErrCircularReference = errors.New("cannot move to a subfolder")
	ErrInvalidTarget     = errors.New("invalid target folder")
	ErrNotImage          = errors.New("file is not an image")
)

const thumbnailSize = 200
```

- [ ] **Step 3: 添加 GetThumbnail 方法**

在文件末尾添加：

```go
// GetThumbnail returns thumbnail path, generating if needed
func (s *FileService) GetThumbnail(ctx context.Context, userID, fileID int64) (string, error) {
	file, err := s.fileRepo.FindByIDAndOwner(ctx, fileID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrFileNotFound
		}
		return "", fmt.Errorf("failed to find file: %w", err)
	}

	if file.IsFolder || !file.PhysicalID.Valid {
		return "", ErrNotImage
	}

	pf, err := s.physicalRepo.FindByID(ctx, file.PhysicalID.Int64)
	if err != nil {
		return "", fmt.Errorf("failed to find physical file: %w", err)
	}

	// Check if image type
	if !isImageType(pf.MimeType) {
		return "", ErrNotImage
	}

	thumbnailPath := s.storage.ThumbnailPath(pf.ID)

	// Check if thumbnail already exists
	if _, err := os.Stat(thumbnailPath); err == nil {
		return thumbnailPath, nil
	}

	// Generate thumbnail with file lock
	if err := s.generateThumbnail(pf, thumbnailPath); err != nil {
		return "", err
	}

	return thumbnailPath, nil
}

func isImageType(mimeType string) bool {
	imageTypes := []string{"image/jpeg", "image/png", "image/gif", "image/webp"}
	for _, t := range imageTypes {
		if strings.HasPrefix(mimeType, t) {
			return true
		}
	}
	return false
}

func (s *FileService) generateThumbnail(pf *model.PhysicalFile, thumbnailPath string) error {
	// Use file lock for concurrency safety
	lockPath := thumbnailPath + ".lock"
	f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		// Another process is generating, wait and check
		time.Sleep(100 * time.Millisecond)
		if _, err := os.Stat(thumbnailPath); err == nil {
			return nil // Thumbnail was generated by another process
		}
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer os.Remove(lockPath)
	defer f.Close()

	// Decode original image
	absPath := s.storage.ToAbsPath(pf.StoragePath)
	imgFile, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}
	defer imgFile.Close()

	var img image.Image
	switch pf.MimeType {
	case "image/png":
		img, err = png.Decode(imgFile)
	case "image/gif", "image/webp":
		// For gif/webp, try generic decode
		img, _, err = image.Decode(imgFile)
	default: // jpeg
		img, err = jpeg.Decode(imgFile)
	}
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	// Resize
	resized := resizeImage(img, thumbnailSize)

	// Ensure thumbnail directory exists
	if err := s.storage.EnsureParentDir(thumbnailPath); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	// Save as JPEG
	outFile, err := os.Create(thumbnailPath)
	if err != nil {
		return fmt.Errorf("failed to create thumbnail: %w", err)
	}
	defer outFile.Close()

	if err := jpeg.Encode(outFile, resized, &jpeg.Options{Quality: 85}); err != nil {
		return fmt.Errorf("failed to encode thumbnail: %w", err)
	}

	// Update thumbnail path in database
	if err := s.physicalRepo.UpdateThumbnail(context.Background(), pf.ID, thumbnailPath); err != nil {
		log.Printf("warning: failed to update thumbnail path: %v", err)
	}

	return nil
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

	// Use simple nearest-neighbor scaling
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
```

- [ ] **Step 4: Commit**

```bash
git add internal/service/file.go
git commit -m "feat: add thumbnail generation service"
```

---

## Task 4: 缩略图获取 Handler 层

**Files:**
- Modify: `internal/handler/file.go`

- [ ] **Step 1: 添加 GetThumbnail handler**

在文件末尾添加：

```go
func (h *FileHandler) GetThumbnail(c *gin.Context) {
	userID := GetUserID(c)
	fileID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	thumbnailPath, err := h.fileService.GetThumbnail(c.Request.Context(), userID, fileID)
	if err != nil {
		if err == service.ErrFileNotFound {
			response.NotFound(c, "file not found")
			return
		}
		if err == service.ErrNotImage {
			response.Error(c, 400, "file is not an image")
			return
		}
		response.InternalError(c, "failed to get thumbnail")
		return
	}

	c.Header("Content-Type", "image/jpeg")
	c.File(thumbnailPath)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/handler/file.go
git commit -m "feat: add thumbnail handler"
```

---

## Task 5: 文件夹 ZIP 下载 Service 层

**Files:**
- Modify: `internal/service/file.go`

- [ ] **Step 1: 添加必要的 import**

在 import 中添加 `"archive/zip"` 和 `"io"`：

```go
import (
	"archive/zip"
	"cloudbox/internal/model"
	"cloudbox/internal/repository"
	"cloudbox/internal/util/storage"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)
```

- [ ] **Step 2: 添加 StreamFolderZip 方法**

在文件末尾添加：

```go
// StreamFolderZip creates a zip file and streams it to writer
func (s *FileService) StreamFolderZip(ctx context.Context, userID, folderID int64, writer io.Writer) error {
	folder, err := s.fileRepo.FindByIDAndOwner(ctx, folderID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFileNotFound
		}
		return fmt.Errorf("failed to find folder: %w", err)
	}

	if !folder.IsFolder {
		return errors.New("not a folder")
	}

	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()

	// Recursively add files
	return s.addFolderToZip(ctx, zipWriter, folder, folder.Name)
}

func (s *FileService) addFolderToZip(ctx context.Context, zipWriter *zip.Writer, folder *model.File, basePath string) error {
	files, err := s.fileRepo.FindByParentAndOwner(ctx, folder.ID, folder.OwnerID, false)
	if err != nil {
		return fmt.Errorf("failed to list folder contents: %w", err)
	}

	for _, file := range files {
		fullPath := filepath.Join(basePath, file.Name)

		if file.IsFolder {
			// Recursively add subfolder
			if err := s.addFolderToZip(ctx, zipWriter, &file, fullPath); err != nil {
				return err
			}
		} else if file.PhysicalID.Valid {
			// Add file
			pf, err := s.physicalRepo.FindByID(ctx, file.PhysicalID.Int64)
			if err != nil {
				log.Printf("warning: failed to find physical file %d: %v", file.PhysicalID.Int64, err)
				continue
			}

			absPath := s.storage.ToAbsPath(pf.StoragePath)
			if err := s.addFileToZip(zipWriter, absPath, fullPath); err != nil {
				log.Printf("warning: failed to add file %s to zip: %v", fullPath, err)
				continue
			}
		}
	}

	return nil
}

func (s *FileService) addFileToZip(zipWriter *zip.Writer, absPath, zipPath string) error {
	file, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = zipPath
	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	return err
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/service/file.go
git commit -m "feat: add folder zip streaming service"
```

---

## Task 6: 文件夹 ZIP 下载 Handler 层

**Files:**
- Modify: `internal/handler/file.go`

- [ ] **Step 1: 添加 DownloadFolder handler**

在文件末尾添加：

```go
func (h *FileHandler) DownloadFolder(c *gin.Context) {
	userID := GetUserID(c)
	folderID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	// Get folder info first for filename
	folder, err := h.fileService.GetFile(c.Request.Context(), userID, folderID)
	if err != nil {
		if err == service.ErrFileNotFound {
			response.NotFound(c, "folder not found")
			return
		}
		response.InternalError(c, "failed to get folder")
		return
	}

	if !folder.IsFolder {
		response.Error(c, 400, "not a folder")
		return
	}

	// RFC 5987 encoding for Chinese filename
	encodedName := url.PathEscape(folder.Name)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s.zip", encodedName))
	c.Header("Content-Type", "application/zip")

	if err := h.fileService.StreamFolderZip(c.Request.Context(), userID, folderID, c.Writer); err != nil {
		// Headers already sent, can only log error
		log.Printf("error streaming folder zip: %v", err)
	}
}
```

- [ ] **Step 2: 添加 log import**

在 `internal/handler/file.go` 的 import 中添加 `"log"`：

```go
import (
	"cloudbox/internal/service"
	"cloudbox/internal/util/response"
	"fmt"
	"log"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
)
```

- [ ] **Step 3: Commit**

```bash
git add internal/handler/file.go
git commit -m "feat: add folder download handler"
```

---

## Task 7: 清空回收站 Service 层

**Files:**
- Modify: `internal/service/file.go`

- [ ] **Step 1: 添加 EmptyTrash 方法**

在文件末尾添加：

```go
// EmptyTrash permanently deletes all files in trash for a user
func (s *FileService) EmptyTrash(ctx context.Context, userID int64) (int, error) {
	// Get all trashed files
	trashFiles, err := s.fileRepo.FindTrash(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to list trash: %w", err)
	}

	if len(trashFiles) == 0 {
		return 0, nil
	}

	// Collect all files including descendants
	var allFiles []model.File
	physicalRefCount := make(map[int64]int)

	for _, file := range trashFiles {
		allFiles = append(allFiles, file)

		// Get descendants
		descendants, err := s.fileRepo.FindAllDescendants(ctx, file.ID)
		if err != nil {
			log.Printf("warning: failed to get descendants for %d: %v", file.ID, err)
			continue
		}
		allFiles = append(allFiles, descendants...)
	}

	// Count physical file references
	for _, f := range allFiles {
		if !f.IsFolder && f.PhysicalID.Valid {
			physicalRefCount[f.PhysicalID.Int64]++
		}
	}

	// Delete all file records
	for _, f := range allFiles {
		if err := s.fileRepo.Delete(ctx, f.ID); err != nil {
			log.Printf("warning: failed to delete file record %d: %v", f.ID, err)
		}
	}

	// Handle physical files
	for pid, count := range physicalRefCount {
		newRefCount, err := s.physicalRepo.DecrementRefCount(ctx, pid, count)
		if err != nil {
			log.Printf("warning: failed to decrement ref count for physical file %d: %v", pid, err)
			continue
		}

		if newRefCount <= 0 {
			pf, err := s.physicalRepo.FindByID(ctx, pid)
			if err != nil {
				log.Printf("warning: failed to find physical file %d: %v", pid, err)
				continue
			}

			// Delete physical file
			absPath := s.storage.ToAbsPath(pf.StoragePath)
			if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
				log.Printf("warning: failed to delete physical file %s: %v", absPath, err)
			}

			// Delete thumbnail
			if pf.ThumbnailPath != "" {
				if err := os.Remove(pf.ThumbnailPath); err != nil && !os.IsNotExist(err) {
					log.Printf("warning: failed to delete thumbnail %s: %v", pf.ThumbnailPath, err)
				}
			}

			// Delete physical file record
			if err := s.physicalRepo.Delete(ctx, pid); err != nil {
				log.Printf("warning: failed to delete physical file record %d: %v", pid, err)
			}
		}
	}

	return len(trashFiles), nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/service/file.go
git commit -m "feat: add empty trash service"
```

---

## Task 8: 清空回收站 Handler 层

**Files:**
- Modify: `internal/handler/trash.go`

- [ ] **Step 1: 添加 EmptyTrash handler**

在 `internal/handler/trash.go` 文件末尾添加：

```go
func (h *TrashHandler) EmptyTrash(c *gin.Context) {
	userID := GetUserID(c)

	count, err := h.fileService.EmptyTrash(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "failed to empty trash")
		return
	}

	response.Success(c, gin.H{
		"deletedCount": count,
	})
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/handler/trash.go
git commit -m "feat: add empty trash handler"
```

---

## Task 9: 注册路由

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: 添加新路由**

在 `cmd/server/main.go` 的 protected 路由组中添加：

```go
// Files
protected.GET("/files", fileHandler.ListFiles)
protected.GET("/files/lookup", fileHandler.LookupFile)
protected.GET("/files/:id", fileHandler.GetFile)
protected.GET("/files/:id/download", fileHandler.DownloadFile)
protected.GET("/files/:id/thumbnail", fileHandler.GetThumbnail)
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
```

- [ ] **Step 2: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: register download and empty trash routes"
```

---

## Task 10: 构建和测试

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

- [ ] **Step 3: 测试登录获取 Token**

```bash
curl -X POST http://localhost:8080/api/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123"}'
```

- [ ] **Step 4: 测试文件下载接口**

使用上一步获取的 token 测试：

```bash
curl -X GET "http://localhost:8080/api/files/1/download" -H "Authorization: Bearer <token>"
```

- [ ] **Step 5: 最终提交**

```bash
git add -A
git commit -m "feat: complete download APIs implementation"
```

---

## Self-Review Checklist

- [x] 所有 4 个接口都有对应的 Service 和 Handler 实现
- [x] 中文文件名使用 RFC 5987 编码
- [x] 缩略图使用文件锁保证并发安全
- [x] ZIP 使用流式生成
- [x] 清空回收站同步执行
- [x] 所有新增方法都有错误处理
- [x] 路由顺序正确（静态路由在参数路由前）
- [x] 无 placeholder 代码
