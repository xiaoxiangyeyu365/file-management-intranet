# WebDAV Protocol Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable Windows/Mac/Linux file managers to mount CloudBox as a network drive via WebDAV.

**Architecture:** `golang.org/x/net/webdav.Handler` handles the WebDAV protocol layer. A custom `cloudFS` type implements `webdav.FileSystem` by calling existing `FileService` and `UploadService` methods. HTTP Basic Auth validates credentials against the user database without generating JWT tokens.

**Tech Stack:** Go, Gin, `golang.org/x/net/webdav`, existing `FileService`/`UploadService`

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/service/auth.go` | Add `ValidateCredentials` — credential check only, no JWT, no audit |
| `internal/service/upload.go` | Add `UploadFile` — single-request upload with quota/dedup/audit |
| `internal/handler/webdav_auth.go` | `BasicAuthMiddleware` — decode Basic Auth, call ValidateCredentials |
| `internal/handler/webdav.go` | `cloudFS` implementing `webdav.FileSystem`, path resolver, FileInfo/File wrappers |
| `cmd/server/main.go` | Wire `/dav/*path` route with BasicAuth + webdav.Handler |
| `web/src/views/AdminAuditView.vue` | Add `file.copy` / `folder.copy` to audit filter options |
| `internal/handler/webdav_test.go` | Tests for path resolution and cloudFS methods |
| `internal/service/upload_test.go` | Tests for UploadFile method |

---

### Task 1: Add ValidateCredentials to AuthService

**Files:**
- Modify: `internal/service/auth.go`
- Test: `internal/service/auth_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/service/auth_test.go`:

```go
func TestValidateCredentials_Success(t *testing.T) {
    user := &model.User{ID: 1, Username: "alice", PasswordHash: "$2a$10$hash", Role: "user", Status: "approved"}
    mockUserRepo := &mockUserRepo{
        findByUsernameFn: func(ctx context.Context, username string) (*model.User, error) {
            return user, nil
        },
    }
    mockHasher := &mockPasswordHasher{
        checkPasswordFn: func(password, hash string) bool { return true },
    }
    svc := service.NewAuthService(mockUserRepo, mockHasher, &mockTokenGenerator{}, false, false, "", noopAudit)

    result, err := svc.ValidateCredentials(context.Background(), "alice", "password123")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result.ID != 1 || result.Username != "alice" {
        t.Fatalf("unexpected user: %+v", result)
    }
}

func TestValidateCredentials_WrongPassword(t *testing.T) {
    user := &model.User{ID: 1, Username: "alice", PasswordHash: "$2a$10$hash", Role: "user", Status: "approved"}
    mockUserRepo := &mockUserRepo{
        findByUsernameFn: func(ctx context.Context, username string) (*model.User, error) {
            return user, nil
        },
    }
    mockHasher := &mockPasswordHasher{
        checkPasswordFn: func(password, hash string) bool { return false },
    }
    svc := service.NewAuthService(mockUserRepo, mockHasher, &mockTokenGenerator{}, false, false, "", noopAudit)

    _, err := svc.ValidateCredentials(context.Background(), "alice", "wrong")
    if err != service.ErrInvalidCredentials {
        t.Fatalf("expected ErrInvalidCredentials, got: %v", err)
    }
}

func TestValidateCredentials_PendingAccount(t *testing.T) {
    user := &model.User{ID: 2, Username: "bob", PasswordHash: "$2a$10$hash", Role: "user", Status: "pending"}
    mockUserRepo := &mockUserRepo{
        findByUsernameFn: func(ctx context.Context, username string) (*model.User, error) {
            return user, nil
        },
    }
    mockHasher := &mockPasswordHasher{
        checkPasswordFn: func(password, hash string) bool { return true },
    }
    svc := service.NewAuthService(mockUserRepo, mockHasher, &mockTokenGenerator{}, false, false, "", noopAudit)

    _, err := svc.ValidateCredentials(context.Background(), "bob", "password123")
    if err != service.ErrAccountPending {
        t.Fatalf("expected ErrAccountPending, got: %v", err)
    }
}

func TestValidateCredentials_DisabledAccount(t *testing.T) {
    user := &model.User{ID: 3, Username: "charlie", PasswordHash: "$2a$10$hash", Role: "user", Status: "disabled"}
    mockUserRepo := &mockUserRepo{
        findByUsernameFn: func(ctx context.Context, username string) (*model.User, error) {
            return user, nil
        },
    }
    mockHasher := &mockPasswordHasher{
        checkPasswordFn: func(password, hash string) bool { return true },
    }
    svc := service.NewAuthService(mockUserRepo, mockHasher, &mockTokenGenerator{}, false, false, "", noopAudit)

    _, err := svc.ValidateCredentials(context.Background(), "charlie", "password123")
    if err != service.ErrAccountDisabled {
        t.Fatalf("expected ErrAccountDisabled, got: %v", err)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestValidateCredentials -v`
Expected: FAIL with "undefined: service.ValidateCredentials" or similar

- [ ] **Step 3: Implement ValidateCredentials**

Add to `internal/service/auth.go`, after the `GetUserByID` method:

```go
func (s *AuthService) ValidateCredentials(ctx context.Context, username, password string) (*model.User, error) {
	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if !s.hasher.CheckPassword(password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	if user.Status == model.UserStatusPending {
		return nil, ErrAccountPending
	}
	if user.Status == model.UserStatusDisabled {
		return nil, ErrAccountDisabled
	}

	return user, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/service/ -run TestValidateCredentials -v`
Expected: PASS — all 4 cases

- [ ] **Step 5: Commit**

```bash
git add internal/service/auth.go internal/service/auth_test.go
git commit -m "feat: add ValidateCredentials for WebDAV Basic Auth"
```

---

### Task 2: Add UploadFile to UploadService

**Files:**
- Modify: `internal/service/upload.go`
- Test: `internal/service/upload_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/service/upload_test.go`:

```go
func TestUploadFile_BasicUpload(t *testing.T) {
	tmpDir := t.TempDir()
	storageManager := storage.NewStorageManager(tmpDir, filepath.Join(tmpDir, "temp"), filepath.Join(tmpDir, "thumbs"))

	pfCreated := &model.PhysicalFile{}
	mockFileRepo := &mockFileRepo{
		createFn: func(ctx context.Context, file *model.File) error {
			file.ID = 42
			return nil
		},
	}
	mockPhysicalRepo := &mockPhysicalFileRepo{
		createFn: func(ctx context.Context, pf *model.PhysicalFile) error {
			pf.ID = 10
			pfCreated = pf
			return nil
		},
		findByMD5Fn: func(ctx context.Context, md5 string) (*model.PhysicalFile, error) {
			return nil, gorm.ErrRecordNotFound
		},
		updateFn: func(ctx context.Context, pf *model.PhysicalFile) error {
			pfCreated = pf
			return nil
		},
	}
	mockUserRepo := &mockUserRepo{
		getQuotaFn: func(ctx context.Context, userID int64) (*int64, error) {
			return nil, nil // no per-user quota
		},
	}

	svc := service.NewUploadService(mockFileRepo, mockPhysicalRepo, mockUserRepo, storageManager, nil, 5*1024*1024, 0, noopAudit)

	content := "hello world"
	reader := strings.NewReader(content)
	file, err := svc.UploadFile(context.Background(), 1, 0, "test.txt", int64(len(content)), reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if file.ID != 42 {
		t.Fatalf("expected file ID 42, got %d", file.ID)
	}
	if file.Name != "test.txt" {
		t.Fatalf("expected name 'test.txt', got '%s'", file.Name)
	}
	if pfCreated.MD5 != "5eb63bbbe01eeed093cb22bb8f5acdc3" {
		t.Fatalf("unexpected MD5: %s", pfCreated.MD5)
	}
}

func TestUploadFile_InstantUpload(t *testing.T) {
	tmpDir := t.TempDir()
	storageManager := storage.NewStorageManager(tmpDir, filepath.Join(tmpDir, "temp"), filepath.Join(tmpDir, "thumbs"))

	existingPF := &model.PhysicalFile{ID: 5, MD5: "5eb63bbbe01eeed093cb22bb8f5acdc3", Size: 11}
	mockFileRepo := &mockFileRepo{
		createFn: func(ctx context.Context, file *model.File) error {
			file.ID = 99
			return nil
		},
	}
	mockPhysicalRepo := &mockPhysicalFileRepo{
		findByMD5Fn: func(ctx context.Context, md5 string) (*model.PhysicalFile, error) {
			return existingPF, nil
		},
		incrementRefCountFn: func(ctx context.Context, id int64) error { return nil },
	}
	mockUserRepo := &mockUserRepo{
		getQuotaFn: func(ctx context.Context, userID int64) (*int64, error) {
			return nil, nil
		},
	}

	svc := service.NewUploadService(mockFileRepo, mockPhysicalRepo, mockUserRepo, storageManager, nil, 5*1024*1024, 0, noopAudit)

	content := "hello world"
	reader := strings.NewReader(content)
	file, err := svc.UploadFile(context.Background(), 1, 0, "test.txt", int64(len(content)), reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if file.ID != 99 {
		t.Fatalf("expected file ID 99, got %d", file.ID)
	}
	if file.ContentRef != 5 {
		t.Fatalf("expected ContentRef 5 (instant upload), got %d", file.ContentRef)
	}
}

func TestUploadFile_QuotaExceeded(t *testing.T) {
	tmpDir := t.TempDir()
	storageManager := storage.NewStorageManager(tmpDir, filepath.Join(tmpDir, "temp"), filepath.Join(tmpDir, "thumbs"))

	mockPhysicalRepo := &mockPhysicalFileRepo{
		calculateUserStorageUsageFn: func(ctx context.Context, userID int64) (int64, error) {
			return 1000, nil
		},
	}
	mockUserRepo := &mockUserRepo{
		getQuotaFn: func(ctx context.Context, userID int64) (*int64, error) {
			quota := int64(500)
			return &quota, nil
		},
	}

	svc := service.NewUploadService(&mockFileRepo{}, mockPhysicalRepo, mockUserRepo, storageManager, nil, 5*1024*1024, 0, noopAudit)

	content := "hello world"
	reader := strings.NewReader(content)
	_, err := svc.UploadFile(context.Background(), 1, 0, "test.txt", int64(len(content)), reader)
	if err != os.ErrPermission {
		t.Fatalf("expected os.ErrPermission, got: %v", err)
	}
}

func TestUploadFile_UnknownSize(t *testing.T) {
	tmpDir := t.TempDir()
	storageManager := storage.NewStorageManager(tmpDir, filepath.Join(tmpDir, "temp"), filepath.Join(tmpDir, "thumbs"))

	mockFileRepo := &mockFileRepo{
		createFn: func(ctx context.Context, file *model.File) error {
			file.ID = 50
			return nil
		},
	}
	mockPhysicalRepo := &mockPhysicalFileRepo{
		createFn: func(ctx context.Context, pf *model.PhysicalFile) error {
			pf.ID = 20
			return nil
		},
		findByMD5Fn: func(ctx context.Context, md5 string) (*model.PhysicalFile, error) {
			return nil, gorm.ErrRecordNotFound
		},
		updateFn: func(ctx context.Context, pf *model.PhysicalFile) error {
			return nil
		},
	}
	mockUserRepo := &mockUserRepo{
		getQuotaFn: func(ctx context.Context, userID int64) (*int64, error) {
			return nil, nil
		},
	}

	svc := service.NewUploadService(mockFileRepo, mockPhysicalRepo, mockUserRepo, storageManager, nil, 5*1024*1024, 0, noopAudit)

	content := "hello world"
	reader := strings.NewReader(content)
	file, err := svc.UploadFile(context.Background(), 1, 0, "test.txt", 0, reader) // size=0 unknown
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if file.ID != 50 {
		t.Fatalf("expected file ID 50, got %d", file.ID)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestUploadFile -v`
Expected: FAIL with "undefined: service.UploadFile"

- [ ] **Step 3: Implement UploadFile**

Add to `internal/service/upload.go`, after the `createFileRecord` method:

```go
func (s *UploadService) UploadFile(ctx context.Context, userID, parentID int64,
	name string, size int64, reader io.Reader) (*model.File, error) {

	if name == "" {
		return nil, errors.New("file name is required")
	}

	// Write to temp file, computing MD5 and actual size
	tempFile, err := os.CreateTemp("", "webdav-upload-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath) // cleanup on failure

	hash := md5.New()
	actualSize, err := io.Copy(io.MultiWriter(tempFile, hash), reader)
	tempFile.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}

	// Use actual size when caller doesn't know (size=0)
	if size == 0 {
		size = actualSize
	}

	// Quota check
	if err := s.checkQuota(ctx, userID, size); err != nil {
		return nil, os.ErrPermission
	}

	md5Hex := hex.EncodeToString(hash.Sum(nil))

	// Instant upload check
	pf, err := s.physicalRepo.FindByMD5(ctx, md5Hex)
	if err == nil {
		// Instant upload
		if err := s.physicalRepo.IncrementRefCount(ctx, pf.ID); err != nil {
			return nil, fmt.Errorf("failed to increment ref count: %w", err)
		}
		file, err := s.createFileRecord(ctx, userID, name, parentID, pf)
		if err != nil {
			s.physicalRepo.DecrementRefCount(ctx, pf.ID, 1)
			return nil, err
		}
		s.audit.Record(ctx, "file.upload", "file", file.ID, name, fmt.Sprintf(`{"size":%d,"instant":true}`, size))
		return file, nil
	}

	// New physical file
	pf = &model.PhysicalFile{
		MD5:      md5Hex,
		Size:     size,
		MimeType: detectMimeType(name),
	}
	if err := s.physicalRepo.Create(ctx, pf); err != nil {
		return nil, fmt.Errorf("failed to create physical file: %w", err)
	}

	// Generate storage path and move temp file
	ext := filepath.Ext(name)
	relative, absolute := s.storage.GenerateFilePath(pf.ID, ext)
	pf.StoragePath = relative

	if err := s.storage.EnsureParentDir(absolute); err != nil {
		return nil, fmt.Errorf("failed to create storage dir: %w", err)
	}
	if err := os.Rename(tempPath, absolute); err != nil {
		if copyErr := copyFile(tempPath, absolute); copyErr != nil {
			return nil, fmt.Errorf("failed to move/copy file: %w", copyErr)
		}
	}

	if err := s.physicalRepo.Update(ctx, pf); err != nil {
		return nil, fmt.Errorf("failed to update physical file: %w", err)
	}

	// Create file record
	file, err := s.createFileRecord(ctx, userID, name, parentID, pf)
	if err != nil {
		return nil, err
	}

	// Thumbnail generation
	if s.previewService != nil {
		go s.previewService.ProcessImage(context.Background(), pf.ID)
	}

	s.audit.Record(ctx, "file.upload", "file", file.ID, name, fmt.Sprintf(`{"size":%d,"instant":false}`, size))

	return file, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/service/ -run TestUploadFile -v`
Expected: PASS — all 4 cases

- [ ] **Step 5: Commit**

```bash
git add internal/service/upload.go internal/service/upload_test.go
git commit -m "feat: add UploadFile for single-request uploads (WebDAV PUT)"
```

---

### Task 3: BasicAuthMiddleware

**Files:**
- Create: `internal/handler/webdav_auth.go`

- [ ] **Step 1: Create the middleware file**

```go
// internal/handler/webdav_auth.go
package handler

import (
	"cloudbox/internal/service"
	"cloudbox/internal/util/crypto"
	"context"
	"encoding/base64"
	"strings"

	"github.com/gin-gonic/gin"
)

func BasicAuthMiddleware(authService *service.AuthService, audit service.AuditRecorder) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Basic ") {
			c.Header("WWW-Authenticate", `Basic realm="CloudBox WebDAV"`)
			c.AbortWithStatus(401)
			return
		}

		decoded, err := base64.StdEncoding.DecodeString(authHeader[6:])
		if err != nil {
			c.Header("WWW-Authenticate", `Basic realm="CloudBox WebDAV"`)
			c.AbortWithStatus(401)
			return
		}

		parts := strings.SplitN(string(decoded), ":", 2)
		if len(parts) != 2 {
			c.Header("WWW-Authenticate", `Basic realm="CloudBox WebDAV"`)
			c.AbortWithStatus(401)
			return
		}
		username, password := parts[0], parts[1]

		ctx := context.WithValue(c.Request.Context(), "clientIP", c.ClientIP())
		c.Request = c.Request.WithContext(ctx)

		user, err := authService.ValidateCredentials(ctx, username, password)
		if err != nil {
			audit.Record(ctx, "user.login_failed", "user", 0, username, "")
			c.Header("WWW-Authenticate", `Basic realm="CloudBox WebDAV"`)
			c.AbortWithStatus(401)
			return
		}

		c.Set("userID", user.ID)
		c.Set("username", user.Username)
		c.Set("role", user.Role)

		ctx = context.WithValue(ctx, "userID", user.ID)
		ctx = context.WithValue(ctx, "username", user.Username)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// GetUserIDFromContext extracts userID from a context.Context value.
// Used by WebDAV handler which doesn't have a gin.Context.
func GetUserIDFromContext(ctx context.Context) int64 {
	if v, ok := ctx.Value("userID").(int64); ok {
		return v
	}
	panic("GetUserIDFromContext: userID not in context")
}
```

- [ ] **Step 2: Build to verify compilation**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/handler/webdav_auth.go
git commit -m "feat: add BasicAuthMiddleware for WebDAV"
```

---

### Task 4: cloudFS — Path Resolution, Stat, Mkdir, RemoveAll

**Files:**
- Create: `internal/handler/webdav.go`
- Test: `internal/handler/webdav_test.go`

- [ ] **Step 1: Create cloudFS with path resolver and basic operations**

```go
// internal/handler/webdav.go
package handler

import (
	"cloudbox/internal/model"
	"cloudbox/internal/service"
	"cloudbox/internal/util/storage"
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/net/webdav"
)

// cloudFS implements webdav.FileSystem backed by CloudBox services.
type cloudFS struct {
	fileService   *service.FileService
	uploadService *service.UploadService
	audit         service.AuditRecorder
	storage       storage.Storage
}

// NewWebDAVHandler returns an http.Handler that serves WebDAV requests.
func NewWebDAVHandler(fileService *service.FileService, uploadService *service.UploadService, audit service.AuditRecorder, storage storage.Storage) http.Handler {
	fs := &cloudFS{
		fileService:   fileService,
		uploadService: uploadService,
		audit:         audit,
		storage:       storage,
	}

	return &webdav.Handler{
		FileSystem: fs,
		LockSystem: webdav.NewMemLS(),
	}
}

// resolvePath walks the virtual path and returns the File record.
// Empty path returns a virtual root with ID=0 (IsFolder=true).
func (fs *cloudFS) resolvePath(ctx context.Context, userID int64, webdavPath string) (*model.File, error) {
	webdavPath = path.Clean(webdavPath)
	webdavPath = strings.TrimPrefix(webdavPath, "/")
	if webdavPath == "" || webdavPath == "." {
		// Virtual root
		return &model.File{ID: 0, Name: "", IsFolder: true, OwnerID: userID}, nil
	}

	segments := strings.Split(webdavPath, "/")
	parentID := int64(0)
	var current *model.File

	for _, seg := range segments {
		if seg == "" {
			continue
		}
		f, err := fs.fileService.FindByName(ctx, userID, parentID, seg)
		if err != nil {
			return nil, os.ErrNotExist
		}
		current = f
		parentID = f.ID
	}

	return current, nil
}

// splitParent splits a path into parent directory path and the final name.
// e.g. "/docs/report.pdf" → "/docs", "report.pdf"
func splitParent(webdavPath string) (parentPath, name string) {
	webdavPath = path.Clean(webdavPath)
	dir := path.Dir(webdavPath)
	base := path.Base(webdavPath)
	if dir == "." {
		dir = ""
	}
	return dir, base
}

// Mkdir implements webdav.FileSystem.
func (fs *cloudFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	userID := GetUserIDFromContext(ctx)
	parentPath, folderName := splitParent(name)

	parent, err := fs.resolvePath(ctx, userID, parentPath)
	if err != nil {
		return os.ErrNotExist
	}

	_, err = fs.fileService.CreateFolder(ctx, userID, parent.ID, folderName)
	if err != nil {
		if err == service.ErrNameConflict {
			return os.ErrExist
		}
		return err
	}
	return nil
}

// Stat implements webdav.FileSystem.
func (fs *cloudFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	userID := GetUserIDFromContext(ctx)
	file, err := fs.resolvePath(ctx, userID, name)
	if err != nil {
		return nil, err
	}
	return &cloudFileInfo{file: file}, nil
}

// RemoveAll implements webdav.FileSystem.
func (fs *cloudFS) RemoveAll(ctx context.Context, name string) error {
	userID := GetUserIDFromContext(ctx)
	file, err := fs.resolvePath(ctx, userID, name)
	if err != nil {
		return err
	}
	if file.ID == 0 {
		return os.ErrPermission // can't delete root
	}

	if err := fs.fileService.MoveToTrash(ctx, userID, file.ID); err != nil {
		if err == service.ErrFileNotFound {
			return os.ErrNotExist
		}
		return err
	}
	return nil
}

// Rename implements webdav.FileSystem.
func (fs *cloudFS) Rename(ctx context.Context, oldName, newName string) error {
	userID := GetUserIDFromContext(ctx)
	srcFile, err := fs.resolvePath(ctx, userID, oldName)
	if err != nil {
		return err
	}
	if srcFile.ID == 0 {
		return os.ErrPermission // can't rename root
	}

	newParentPath, newBaseName := splitParent(newName)
	dstParent, err := fs.resolvePath(ctx, userID, newParentPath)
	if err != nil {
		return os.ErrExist // destination parent not found → 409
	}

	// Check if destination exists
	target, err := fs.resolvePath(ctx, userID, newName)
	if err == nil {
		// Destination exists
		if target.IsFolder {
			// MOVE into folder: preserve source filename
			newBaseName = srcFile.Name
			dstParent = target
		} else {
			// Destination is a file: soft delete it first
			_ = fs.fileService.MoveToTrash(ctx, userID, target.ID)
		}
	}

	// Same parent? → Rename. Different parent? → MoveFiles.
	srcParentID := int64(0)
	if srcFile.ParentID.Valid {
		srcParentID = srcFile.ParentID.Int64
	}

	if srcParentID == dstParent.ID && newBaseName == srcFile.Name {
		return nil // no-op
	}

	if srcParentID == dstParent.ID {
		return fs.fileService.Rename(ctx, userID, srcFile.ID, newBaseName)
	}

	// Cross-folder move
	if err := fs.fileService.MoveFiles(ctx, userID, []int64{srcFile.ID}, dstParent.ID); err != nil {
		if err == service.ErrCircularReference {
			return os.ErrPermission
		}
		if err == service.ErrNameConflict {
			return os.ErrExist
		}
		return err
	}

	// If name also changed, rename after move
	if newBaseName != srcFile.Name {
		return fs.fileService.Rename(ctx, userID, srcFile.ID, newBaseName)
	}
	return nil
}

// cloudFileInfo implements os.FileInfo for a model.File.
type cloudFileInfo struct {
	file *model.File
}

func (fi *cloudFileInfo) Name() string      { return fi.file.Name }
func (fi *cloudFileInfo) Size() int64       {
	if fi.file.Physical != nil {
		return fi.file.Physical.Size
	}
	return 0
}
func (fi *cloudFileInfo) Mode() os.FileMode {
	if fi.file.IsFolder {
		return os.ModeDir | 0755
	}
	return 0644
}
func (fi *cloudFileInfo) ModTime() time.Time { return fi.file.UpdatedAt }
func (fi *cloudFileInfo) IsDir() bool        { return fi.file.IsFolder }
func (fi *cloudFileInfo) Sys() interface{}   { return nil }

// virtualRootInfo is the FileInfo for the virtual root directory.
type virtualRootInfo struct {
	userID int64
}

func (fi *virtualRootInfo) Name() string      { return "" }
func (fi *virtualRootInfo) Size() int64       { return 0 }
func (fi *virtualRootInfo) Mode() os.FileMode { return os.ModeDir | 0755 }
func (fi *virtualRootInfo) ModTime() time.Time { return time.Time{} }
func (fi *virtualRootInfo) IsDir() bool        { return true }
func (fi *virtualRootInfo) Sys() interface{}   { return nil }

// serviceErrorToOSError maps service-layer errors to os errors for webdav.Handler.
func serviceErrorToOSError(err error) error {
	switch err {
	case service.ErrFileNotFound:
		return os.ErrNotExist
	case service.ErrForbidden:
		return os.ErrPermission
	case service.ErrNameConflict:
		return os.ErrExist
	case service.ErrCircularReference:
		return os.ErrPermission
	case service.ErrInvalidTarget:
		return os.ErrNotExist
	case service.ErrIsFolder:
		return os.ErrPermission
	case service.ErrNoPhysicalContent:
		return os.ErrNotExist
	case service.ErrQuotaExceeded:
		return os.ErrPermission
	default:
		return err
	}
}

func (fs *cloudFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	// Implemented in Task 5
	return nil, fmt.Errorf("not implemented")
}
```

- [ ] **Step 2: Build to verify compilation**

Run: `go build ./...`
Expected: PASS (OpenFile returns stub error, but compiles)

- [ ] **Step 3: Write tests for path resolution**

Create `internal/handler/webdav_test.go`:

```go
package handler

import (
	"cloudbox/internal/model"
	"context"
	"os"
	"testing"
)

func TestSplitParent(t *testing.T) {
	tests := []struct {
		input      string
		parentPath string
		name       string
	}{
		{"", "", ""},
		{"/docs/report.pdf", "/docs", "report.pdf"},
		{"/report.pdf", "", "report.pdf"},
		{"report.pdf", "", "report.pdf"},
		{"/a/b/c", "/a/b", "c"},
	}
	for _, tt := range tests {
		p, n := splitParent(tt.input)
		if p != tt.parentPath || n != tt.name {
			t.Errorf("splitParent(%q) = (%q, %q), want (%q, %q)", tt.input, p, n, tt.parentPath, tt.name)
		}
	}
}

func TestCloudFileInfo(t *testing.T) {
	f := &model.File{ID: 1, Name: "test.txt", IsFolder: false, UpdatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Physical: &model.PhysicalFile{Size: 1024}}
	fi := &cloudFileInfo{file: f}

	if fi.Name() != "test.txt" {
		t.Errorf("Name() = %q", fi.Name())
	}
	if fi.Size() != 1024 {
		t.Errorf("Size() = %d", fi.Size())
	}
	if fi.IsDir() {
		t.Error("IsDir() should be false")
	}
	if fi.Mode() != 0644 {
		t.Errorf("Mode() = %o", fi.Mode())
	}

	dir := &model.File{ID: 2, Name: "docs", IsFolder: true}
	di := &cloudFileInfo{file: dir}
	if !di.IsDir() {
		t.Error("IsDir() should be true for folder")
	}
	if di.Mode()&os.ModeDir == 0 {
		t.Error("Mode() should have ModeDir bit")
	}
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/handler/ -run "TestSplitParent|TestCloudFileInfo" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/handler/webdav.go internal/handler/webdav_test.go
git commit -m "feat: add cloudFS with path resolution, Stat, Mkdir, RemoveAll, Rename"
```

---

### Task 5: cloudFS — OpenFile (Read + Write)

**Files:**
- Modify: `internal/handler/webdav.go`

- [ ] **Step 1: Implement OpenFile**

Replace the stub `OpenFile` in `internal/handler/webdav.go`:

```go
// OpenFile implements webdav.FileSystem.
func (fs *cloudFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	userID := GetUserIDFromContext(ctx)

	// Write mode — PUT
	if flag&(os.O_WRONLY|os.O_CREATE) != 0 {
		return fs.openForWrite(ctx, userID, name)
	}

	// Read mode — GET or PROPFIND
	file, err := fs.resolvePath(ctx, userID, name)
	if err != nil {
		return nil, err
	}

	if file.ID == 0 {
		// Virtual root — return directory listing
		return &cloudDir{
			fs:     fs,
			ctx:    ctx,
			userID: userID,
			parentID: 0,
		}, nil
	}

	if file.IsFolder {
		return &cloudDir{
			fs:     fs,
			ctx:    ctx,
			userID: userID,
			parentID: file.ID,
			file:   file,
		}, nil
	}

	// File — open physical file for reading
	if file.ContentRef == 0 {
		return nil, os.ErrNotExist
	}
	if file.Physical == nil {
		return nil, os.ErrNotExist
	}

	absPath := fs.storage.ToAbsPath(file.Physical.StoragePath)
	physicalFile, err := os.Open(absPath)
	if err != nil {
		return nil, os.ErrNotExist
	}

	return &cloudFile{
		File:    physicalFile,
		fileInfo: &cloudFileInfo{file: file},
	}, nil
}

func (fs *cloudFS) openForWrite(ctx context.Context, userID int64, name string) (webdav.File, error) {
	parentPath, fileName := splitParent(name)
	parent, err := fs.resolvePath(ctx, userID, parentPath)
	if err != nil {
		return nil, os.ErrNotExist
	}

	// Check if file already exists → soft delete old version
	existing, err := fs.resolvePath(ctx, userID, name)
	if err == nil && existing.ID != 0 {
		_ = fs.fileService.MoveToTrash(ctx, userID, existing.ID)
	}

	// Return a write buffer that flushes to UploadFile on Close
	return &cloudWriteFile{
		fs:       fs,
		ctx:      ctx,
		userID:   userID,
		parentID: parent.ID,
		fileName: fileName,
	}, nil
}

// cloudFile wraps os.File for reading. Implements webdav.File.
type cloudFile struct {
	*os.File
	fileInfo os.FileInfo
}

func (f *cloudFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, fmt.Errorf("not a directory")
}

func (f *cloudFile) Stat() (os.FileInfo, error) {
	return f.fileInfo, nil
}

func (f *cloudFile) Write(p []byte) (int, error) {
	return 0, os.ErrPermission
}

// cloudDir implements webdav.File for directory listings.
type cloudDir struct {
	fs       *cloudFS
	ctx      context.Context
	userID   int64
	parentID int64
	file     *model.File
}

func (d *cloudDir) Read(p []byte) (int, error)         { return 0, fmt.Errorf("is a directory") }
func (d *cloudDir) Write(p []byte) (int, error)        { return 0, fmt.Errorf("is a directory") }
func (d *cloudDir) Seek(offset int64, whence int) (int64, error) { return 0, fmt.Errorf("is a directory") }
func (d *cloudDir) Close() error                        { return nil }

func (d *cloudDir) Readdir(count int) ([]os.FileInfo, error) {
	files, err := d.fs.fileService.ListFiles(d.ctx, d.userID, d.parentID)
	if err != nil {
		return nil, err
	}

	infos := make([]os.FileInfo, 0, len(files))
	for i := range files {
		infos = append(infos, &cloudFileInfo{file: &files[i]})
	}

	if count > 0 && len(infos) > count {
		return infos[:count], nil
	}
	return infos, nil
}

func (d *cloudDir) Stat() (os.FileInfo, error) {
	if d.file != nil {
		return &cloudFileInfo{file: d.file}, nil
	}
	return &virtualRootInfo{userID: d.userID}, nil
}

// cloudWriteFile buffers a PUT body and flushes to UploadFile on Close.
type cloudWriteFile struct {
	fs       *cloudFS
	ctx      context.Context
	userID   int64
	parentID int64
	fileName string
	buf      []byte
	closed   bool
}

func (f *cloudWriteFile) Read(p []byte) (int, error)         { return 0, fmt.Errorf("write-only") }
func (f *cloudWriteFile) Seek(offset int64, whence int) (int64, error) { return 0, fmt.Errorf("write-only") }
func (f *cloudWriteFile) Readdir(count int) ([]os.FileInfo, error) { return nil, fmt.Errorf("not a directory") }

func (f *cloudWriteFile) Write(p []byte) (int, error) {
	if f.closed {
		return 0, os.ErrClosed
	}
	f.buf = append(f.buf, p...)
	return len(p), nil
}

func (f *cloudWriteFile) Close() error {
	if f.closed {
		return nil
	}
	f.closed = true

	if len(f.buf) == 0 {
		return nil
	}

	reader := strings.NewReader(string(f.buf))
	_, err := f.fs.uploadService.UploadFile(f.ctx, f.userID, f.parentID, f.fileName, int64(len(f.buf)), reader)
	if err != nil {
		return serviceErrorToOSError(err)
	}
	return nil
}

func (f *cloudWriteFile) Stat() (os.FileInfo, error) {
	return &cloudFileInfo{file: &model.File{
		Name:    f.fileName,
		IsFolder: false,
	}}, nil
}
```

- [ ] **Step 2: Add missing import**

Ensure `strings` is in the import list of `internal/handler/webdav.go`.

- [ ] **Step 3: Build to verify compilation**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/handler/webdav.go
git commit -m "feat: implement OpenFile for WebDAV read and write operations"
```

---

### Task 6: cloudFS — COPY operation

**Files:**
- Modify: `internal/handler/webdav.go`

- [ ] **Step 1: Implement Copy method**

Add to `internal/handler/webdav.go`:

```go
// Copy implements webdav.FileSystem. The webdav.Handler calls this for COPY requests.
func (fs *cloudFS) Copy(ctx context.Context, src, dst string, recursive bool) (int, error) {
	userID := GetUserIDFromContext(ctx)

	srcFile, err := fs.resolvePath(ctx, userID, src)
	if err != nil {
		return http.StatusNotFound, err
	}

	dstParentPath, dstBaseName := splitParent(dst)
	dstParent, err := fs.resolvePath(ctx, userID, dstParentPath)
	if err != nil {
		return http.StatusConflict, os.ErrExist
	}

	// Check if destination exists
	dstTarget, err := fs.resolvePath(ctx, userID, dst)
	if err == nil && dstTarget.ID != 0 {
		if dstTarget.IsFolder {
			dstBaseName = srcFile.Name
			dstParent = dstTarget
		} else {
			_ = fs.fileService.MoveToTrash(ctx, userID, dstTarget.ID)
		}
	}

	if srcFile.IsFolder {
		return fs.copyFolder(ctx, userID, srcFile, dstParent.ID, dstBaseName)
	}

	return fs.copyFile(ctx, userID, srcFile, dstParent.ID, dstBaseName)
}

func (fs *cloudFS) copyFile(ctx context.Context, userID int64, src *model.File, dstParentID int64, dstName string) (int, error) {
	if src.ContentRef == 0 || src.Physical == nil {
		return http.StatusNotFound, os.ErrNotExist
	}

	// Reuse existing physical file (increment ref count)
	pf := src.Physical
	if err := fs.uploadService.CreateFileFromPhysical(ctx, userID, dstParentID, dstName, pf); err != nil {
		return http.StatusInternalServerError, serviceErrorToOSError(err)
	}

	fs.audit.Record(ctx, "file.copy", "file", src.ID, src.Name, fmt.Sprintf(`{"dst":"%s"}`, dstName))
	return http.StatusCreated, nil
}

func (fs *cloudFS) copyFolder(ctx context.Context, userID int64, src *model.File, dstParentID int64, dstName string) (int, error) {
	newFolder, err := fs.fileService.CreateFolder(ctx, userID, dstParentID, dstName)
	if err != nil {
		if err == service.ErrNameConflict {
			return http.StatusPreconditionFailed, os.ErrExist
		}
		return http.StatusInternalServerError, err
	}

	children, err := fs.fileService.ListFiles(ctx, userID, src.ID)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	for _, child := range children {
		if child.IsFolder {
			if _, err := fs.copyFolder(ctx, userID, &child, newFolder.ID, child.Name); err != nil {
				return http.StatusInternalServerError, err
			}
		} else {
			if _, err := fs.copyFile(ctx, userID, &child, newFolder.ID, child.Name); err != nil {
				return http.StatusInternalServerError, err
			}
		}
	}

	fs.audit.Record(ctx, "folder.copy", "folder", src.ID, src.Name, fmt.Sprintf(`{"dst":"%s"}`, dstName))
	return http.StatusCreated, nil
}
```

- [ ] **Step 2: Add CreateFileFromPhysical to UploadService**

Add to `internal/service/upload.go`:

```go
// CreateFileFromPhysical creates a new file record pointing to an existing PhysicalFile.
// Used by WebDAV COPY to avoid re-reading/writing file content.
func (s *UploadService) CreateFileFromPhysical(ctx context.Context, userID, parentID int64, name string, pf *model.PhysicalFile) error {
	if err := s.physicalRepo.IncrementRefCount(ctx, pf.ID); err != nil {
		return fmt.Errorf("failed to increment ref count: %w", err)
	}

	file, err := s.createFileRecord(ctx, userID, name, parentID, pf)
	if err != nil {
		s.physicalRepo.DecrementRefCount(ctx, pf.ID, 1)
		return err
	}

	if s.previewService != nil {
		go s.previewService.ProcessImage(context.Background(), pf.ID)
	}

	s.audit.Record(ctx, "file.upload", "file", file.ID, name, fmt.Sprintf(`{"size":%d,"instant":true,"copy":true}`, pf.Size))
	return nil
}
```

- [ ] **Step 3: Build to verify compilation**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/handler/webdav.go internal/service/upload.go
git commit -m "feat: implement COPY for WebDAV and CreateFileFromPhysical"
```

---

### Task 7: Wire Routes in main.go

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add import and route registration**

Add to imports in `cmd/server/main.go`:

```go
import (
	// ... existing imports ...
	"golang.org/x/net/webdav"
)
```

Add after the admin routes block (before `r.Static`):

```go
// WebDAV routes
davHandler := handler.NewWebDAVHandler(fileService, uploadService, auditService, storageManager)
davAuth := handler.BasicAuthMiddleware(authService, auditService)
r.Any("/dav/*path", davAuth, func(c *gin.Context) {
	davHandler.ServeHTTP(c.Writer, c.Request)
})
```

- [ ] **Step 2: Install dependency**

Run: `go get golang.org/x/net/webdav`
Expected: dependency added to go.mod

- [ ] **Step 3: Build to verify compilation**

Run: `go build ./...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add cmd/server/main.go go.mod go.sum
git commit -m "feat: wire WebDAV route at /dav/ with Basic Auth"
```

---

### Task 8: Frontend — Add COPY to Audit Viewer

**Files:**
- Modify: `web/src/views/AdminAuditView.vue`

- [ ] **Step 1: Add file.copy and folder.copy to filter options and label map**

In the `<el-select>` for action filters, add inside the `<el-option-group label="文件">` group (after the "移动" option):

```html
<el-option label="复制" value="file.copy" />
```

In the `<el-option-group label="文件夹">` group, add:

```html
<el-option label="复制文件夹" value="folder.copy" />
```

In the `actionLabel` map, add:

```js
'file.copy': '复制', 'folder.copy': '复制文件夹',
```

- [ ] **Step 2: Build frontend**

Run: `cd /e/fileManagementIntranet/web && npm run build`
Expected: Build succeeds, static files updated in `../static/`

- [ ] **Step 3: Commit**

```bash
git add web/src/views/AdminAuditView.vue static/
git commit -m "feat: add file.copy/folder.copy to audit log viewer"
```

---

### Task 9: End-to-End Verification

- [ ] **Step 1: Build and start server**

```bash
cd /e/fileManagementIntranet
go build ./...
./cloudbox.exe  # or go run cmd/server/main.go
```

- [ ] **Step 2: Test WebDAV with curl — PROPFIND**

```bash
# Login to get credentials (Basic Auth uses the same user/password)
# Test PROPFIND on root
curl -u admin:123456 -X PROPFIND http://localhost:8080/dav/ -H "Depth: 1"
```

Expected: XML response listing files in root directory

- [ ] **Step 3: Test WebDAV — MKCOL (create folder)**

```bash
curl -u admin:123456 -X MKCOL http://localhost:8080/dav/test-folder/
```

Expected: 201 Created

- [ ] **Step 4: Test WebDAV — PUT (upload file)**

```bash
echo "hello webdav" > /tmp/test-webdav.txt
curl -u admin:123456 -X PUT http://localhost:8080/dav/test-webdav.txt \
  -H "Content-Type: application/octet-stream" \
  --data-binary @/tmp/test-webdav.txt
```

Expected: 201 Created or 204 No Content

- [ ] **Step 5: Test WebDAV — GET (download file)**

```bash
curl -u admin:123456 http://localhost:8080/dav/test-webdav.txt
```

Expected: "hello webdav"

- [ ] **Step 6: Test WebDAV — DELETE**

```bash
curl -u admin:123456 -X DELETE http://localhost:8080/dav/test-webdav.txt
```

Expected: 204 No Content

- [ ] **Step 7: Test WebDAV — Windows Explorer mapping**

1. Open Windows Explorer
2. Right-click "This PC" → "Map network drive"
3. Enter `http://<server-ip>:8080/dav/`
4. Check "Connect using different credentials"
5. Enter username/password
6. Verify: drive maps, files visible, drag-and-drop works

- [ ] **Step 8: Verify audit logs**

Check the audit log viewer in the admin panel. WebDAV operations (upload, create folder, delete, copy) should appear in the logs.

- [ ] **Step 9: Commit E2E confirmation**

```bash
git commit --allow-empty -m "test: WebDAV E2E verification complete"
```

---

## Verification Checklist

| Check | Command |
|---|---|
| Go build | `go build ./...` |
| Unit tests | `go test ./internal/service/ -v && go test ./internal/handler/ -v` |
| PROPFIND | `curl -u admin:123456 -X PROPFIND http://localhost:8080/dav/ -H "Depth: 1"` |
| MKCOL | `curl -u admin:123456 -X MKCOL http://localhost:8080/dav/test/` |
| PUT | `curl -u admin:123456 -X PUT http://localhost:8080/dav/test.txt -d "content"` |
| GET | `curl -u admin:123456 http://localhost:8080/dav/test.txt` |
| DELETE | `curl -u admin:123456 -X DELETE http://localhost:8080/dav/test.txt` |
| MOVE | `curl -u admin:123456 -X MOVE http://localhost:8080/dav/test.txt -H "Destination: /dav/renamed.txt"` |
| COPY | `curl -u admin:123456 -X COPY http://localhost:8080/dav/test.txt -H "Destination: /dav/copy.txt"` |
| Auth failure | `curl -u admin:wrong -X PROPFIND http://localhost:8080/dav/ -v` (expect 401) |
