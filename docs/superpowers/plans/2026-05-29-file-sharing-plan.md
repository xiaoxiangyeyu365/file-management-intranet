# File Sharing / Public Link Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add file sharing via public links with password protection, expiration, download limits, and a preview page.

**Architecture:** New `file_shares` table + ShareRepository/ShareService/ShareHandler following existing patterns. Public route group `/api/s/:token` bypasses JWT. HMAC-signed download credentials. Frontend adds share context menu, CreateShareDialog, SharePreview page, and SharesView management page.

**Tech Stack:** Go (Gin + GORM), Vue 3 (Element Plus + Pinia), HMAC-SHA256 for download credentials

---

### Task 1: Add ShareConfig to config

**Files:**
- Modify: `internal/config/config.go:17-26` (Config struct)
- Modify: `internal/config/config.go:80-120` (Load defaults)
- Modify: `configs/config.yaml`

- [ ] **Step 1: Add ShareConfig struct and embed in Config**

In `internal/config/config.go`, add after `AuthConfig` (after line 73):

```go
type ShareConfig struct {
	Secret        string `yaml:"secret"`
	TokenLength   int    `yaml:"token_length"`
	CredentialTTL int    `yaml:"credential_ttl"`
}
```

Add `Share ShareConfig \`yaml:"share"\`` to the `Config` struct (after line 25).

- [ ] **Step 2: Add defaults in Load function**

In the defaults section of `Load()` (after line 120), add:

```go
Share: ShareConfig{
	TokenLength:   8,
	CredentialTTL: 300,
},
```

After the JWT secret env var override (after line 149), add share secret handling:

```go
if cfg.Share.Secret == "" {
	cfg.Share.Secret = generateRandomSecret()
	log.Println("warning: share.secret not configured, using random secret (shares will break on restart)")
}
```

Add a `generateRandomSecret()` helper function that returns a 32-byte hex string (same pattern as JWT random secret generation in the codebase).

- [ ] **Step 3: Add share section to config.yaml**

Append to `configs/config.yaml`:

```yaml
share:
  secret: ""
  token_length: 8
  credential_ttl: 300
```

- [ ] **Step 4: Build and verify**

Run: `go build ./cmd/server`
Expected: compiles with no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go configs/config.yaml
git commit -m "feat: add ShareConfig for file sharing"
```

---

### Task 2: Add FileShare model and database table

**Files:**
- Create: `internal/model/share.go`
- Modify: `internal/repository/db.go:18-73` (InitDB)
- Modify: `internal/repository/db.go` (add createFileSharesTable function)

- [ ] **Step 1: Create FileShare model**

Create `internal/model/share.go`:

```go
package model

import (
	"database/sql"
	"time"
)

type FileShare struct {
	ID            int64        `gorm:"primaryKey" json:"id"`
	Token         string       `gorm:"uniqueIndex;size:8;not null" json:"token"`
	FileID        int64        `gorm:"not null;index" json:"fileId"`
	OwnerID       int64        `gorm:"not null;index" json:"ownerId"`
	PasswordHash  sql.NullString `gorm:"size:255" json:"-"`
	ExpiresAt     sql.NullTime `gorm:"column:expires_at;type:datetime" json:"expiresAt"`
	MaxDownloads  int          `gorm:"default:0" json:"maxDownloads"`
	DownloadCount int          `gorm:"default:0" json:"downloadCount"`
	Revoked       bool         `gorm:"default:false" json:"revoked"`
	CreatedAt     time.Time    `json:"createdAt"`
}
```

- [ ] **Step 2: Add createFileSharesTable to db.go**

Add a new function after `createClipboardTable` following the same pattern (check existence, branch on dialect):

```go
func createFileSharesTable(db *gorm.DB) {
	var count int64
	if db.Dialector.Name() == "sqlite" {
		db.Raw("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='file_shares'").Scan(&count)
	} else {
		db.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'file_shares'").Scan(&count)
	}

	if count > 0 {
		return
	}

	if db.Dialector.Name() == "sqlite" {
		db.Exec(`CREATE TABLE file_shares (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token VARCHAR(8) NOT NULL UNIQUE,
			file_id INTEGER NOT NULL,
			owner_id INTEGER NOT NULL,
			password_hash VARCHAR(255),
			expires_at DATETIME,
			max_downloads INTEGER DEFAULT 0,
			download_count INTEGER DEFAULT 0,
			revoked BOOLEAN DEFAULT 0,
			created_at DATETIME NOT NULL
		)`)
	} else {
		db.Exec(`CREATE TABLE file_shares (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			token VARCHAR(8) NOT NULL UNIQUE,
			file_id BIGINT NOT NULL,
			owner_id BIGINT NOT NULL,
			password_hash VARCHAR(255),
			expires_at DATETIME,
			max_downloads INT DEFAULT 0,
			download_count INT DEFAULT 0,
			revoked BOOLEAN DEFAULT FALSE,
			created_at DATETIME NOT NULL,
			INDEX idx_file_shares_file_id (file_id),
			INDEX idx_file_shares_owner_id (owner_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`)
	}

	if db.Dialector.Name() == "sqlite" {
		db.Exec("CREATE INDEX idx_file_shares_file_id ON file_shares(file_id)")
		db.Exec("CREATE INDEX idx_file_shares_owner_id ON file_shares(owner_id)")
	}
}
```

- [ ] **Step 3: Call createFileSharesTable from InitDB**

In `InitDB`, after `createClipboardTable(db)` (line 65), add:

```go
createFileSharesTable(db)
```

- [ ] **Step 4: Build and verify**

Run: `go build ./cmd/server`
Expected: compiles with no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/model/share.go internal/repository/db.go
git commit -m "feat: add FileShare model and file_shares table"
```

---

### Task 3: Add ShareRepository interface and implementation

**Files:**
- Modify: `internal/service/interfaces.go` (add ShareRepository)
- Create: `internal/repository/share.go`

- [ ] **Step 1: Add ShareRepository interface to interfaces.go**

Append to `internal/service/interfaces.go`:

```go
type ShareRepository interface {
	Create(ctx context.Context, share *model.FileShare) error
	FindByToken(ctx context.Context, token string) (*model.FileShare, error)
	FindByOwner(ctx context.Context, ownerID int64) ([]model.FileShare, error)
	FindByFile(ctx context.Context, fileID int64) ([]model.FileShare, error)
	Revoke(ctx context.Context, id, ownerID int64) error
	IncrementDownloadCount(ctx context.Context, token string) (bool, error)
}
```

Add `"cloudbox/internal/model"` to imports if not already present.

- [ ] **Step 2: Create ShareRepository implementation**

Create `internal/repository/share.go`:

```go
package repository

import (
	"cloudbox/internal/model"
	"context"

	"gorm.io/gorm"
)

type ShareRepository struct {
	db *gorm.DB
}

func NewShareRepository(db *gorm.DB) *ShareRepository {
	return &ShareRepository{db: db}
}

func (r *ShareRepository) Create(ctx context.Context, share *model.FileShare) error {
	return r.db.WithContext(ctx).Create(share).Error
}

func (r *ShareRepository) FindByToken(ctx context.Context, token string) (*model.FileShare, error) {
	var share model.FileShare
	if err := r.db.WithContext(ctx).Where("token = ?", token).First(&share).Error; err != nil {
		return nil, err
	}
	return &share, nil
}

func (r *ShareRepository) FindByOwner(ctx context.Context, ownerID int64) ([]model.FileShare, error) {
	var shares []model.FileShare
	if err := r.db.WithContext(ctx).Where("owner_id = ?", ownerID).Order("created_at DESC").Find(&shares).Error; err != nil {
		return nil, err
	}
	return shares, nil
}

func (r *ShareRepository) FindByFile(ctx context.Context, fileID int64) ([]model.FileShare, error) {
	var shares []model.FileShare
	if err := r.db.WithContext(ctx).Where("file_id = ?", fileID).Order("created_at DESC").Find(&shares).Error; err != nil {
		return nil, err
	}
	return shares, nil
}

func (r *ShareRepository) Revoke(ctx context.Context, id, ownerID int64) error {
	return r.db.WithContext(ctx).Model(&model.FileShare{}).
		Where("id = ? AND owner_id = ?", id, ownerID).
		Update("revoked", true).Error
}

func (r *ShareRepository) IncrementDownloadCount(ctx context.Context, token string) (bool, error) {
	result := r.db.WithContext(ctx).Exec(`
		UPDATE file_shares
		SET download_count = download_count + 1
		WHERE token = ? AND (max_downloads = 0 OR download_count < max_downloads) AND revoked = false
	`, token)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}
```

- [ ] **Step 3: Build and verify**

Run: `go build ./...`
Expected: compiles with no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/service/interfaces.go internal/repository/share.go
git commit -m "feat: add ShareRepository interface and implementation"
```

---

### Task 4: Add StreamFolderZipByID to FileService

**Files:**
- Modify: `internal/service/file.go:482-499` (add StreamFolderZipByID after StreamFolderZip)

- [ ] **Step 1: Add StreamFolderZipByID method**

Add after the `StreamFolderZip` method in `internal/service/file.go`:

```go
func (s *FileService) StreamFolderZipByID(ctx context.Context, folderID int64, writer io.Writer) error {
	folder, err := s.fileRepo.FindByID(ctx, folderID)
	if err != nil {
		return fmt.Errorf("failed to find folder: %w", err)
	}
	if !folder.IsFolder {
		return errors.New("not a folder")
	}

	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()

	return s.addFolderToZip(ctx, zipWriter, folder, folder.Name)
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: compiles with no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/service/file.go
git commit -m "feat: add StreamFolderZipByID for unauthenticated folder download"
```

---

### Task 5: Create ShareService

**Files:**
- Create: `internal/service/share.go`

- [ ] **Step 1: Create ShareService**

Create `internal/service/share.go`:

```go
package service

import (
	"cloudbox/internal/config"
	"cloudbox/internal/model"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"
)

var (
	ErrShareNotFound      = errors.New("share not found")
	ErrShareExpired       = errors.New("share has expired")
	ErrShareRevoked       = errors.New("share has been revoked")
	ErrShareLimitReached  = errors.New("download limit reached")
	ErrInvalidCredential  = errors.New("invalid or expired download credential")
	ErrWrongSharePassword = errors.New("wrong password")
)

type ShareService struct {
	shareRepo    ShareRepository
	fileRepo     FileRepository
	physicalRepo PhysicalFileRepository
	storage      Storage
	fileService  *FileService
	hasher       PasswordHasher
}

func NewShareService(
	shareRepo ShareRepository,
	fileRepo FileRepository,
	physicalRepo PhysicalFileRepository,
	storage Storage,
	fileService *FileService,
	hasher PasswordHasher,
) *ShareService {
	return &ShareService{
		shareRepo:    shareRepo,
		fileRepo:     fileRepo,
		physicalRepo: physicalRepo,
		storage:      storage,
		fileService:  fileService,
		hasher:       hasher,
	}
}

func (s *ShareService) CreateShare(ctx context.Context, userID, fileID int64, password string, expiresAt *time.Time, maxDownloads int) (*model.FileShare, error) {
	// Verify file ownership
	_, err := s.fileRepo.FindByIDAndOwner(ctx, fileID, userID)
	if err != nil {
		return nil, ErrFileNotFound
	}

	token, err := s.generateToken()
	if err != nil {
		return nil, err
	}

	var passwordHash sql.NullString
	if password != "" {
		hash, err := s.hasher.HashPassword(password)
		if err != nil {
			return nil, err
		}
		passwordHash = sql.NullString{String: hash, Valid: true}
	}

	var expiresAtVal sql.NullTime
	if expiresAt != nil {
		expiresAtVal = sql.NullTime{Time: *expiresAt, Valid: true}
	}

	share := &model.FileShare{
		Token:        token,
		FileID:       fileID,
		OwnerID:      userID,
		PasswordHash: passwordHash,
		ExpiresAt:    expiresAtVal,
		MaxDownloads: maxDownloads,
		CreatedAt:    time.Now(),
	}

	if err := s.shareRepo.Create(ctx, share); err != nil {
		return nil, err
	}

	return share, nil
}

func (s *ShareService) GetShareInfo(ctx context.Context, token string) (*model.FileShare, *model.File, error) {
	share, err := s.shareRepo.FindByToken(ctx, token)
	if err != nil {
		return nil, nil, ErrShareNotFound
	}

	if err := s.checkShareValidity(share); err != nil {
		return nil, nil, err
	}

	file, err := s.fileRepo.FindByID(ctx, share.FileID)
	if err != nil {
		return nil, nil, ErrFileNotFound
	}

	return share, file, nil
}

type ShareCredential struct {
	Credential string `json:"credential"`
}

func (s *ShareService) VerifyOrGetCredential(ctx context.Context, token string, password string) (*ShareCredential, error) {
	share, err := s.shareRepo.FindByToken(ctx, token)
	if err != nil {
		return nil, ErrShareNotFound
	}

	if err := s.checkShareValidity(share); err != nil {
		return nil, err
	}

	// If share has a password, verify it
	if share.PasswordHash.Valid {
		if !s.hasher.CheckPassword(password, share.PasswordHash.String) {
			return nil, ErrWrongSharePassword
		}
	}

	cred := s.generateCredential(token)
	return &ShareCredential{Credential: cred}, nil
}

func (s *ShareService) DownloadByShare(ctx context.Context, token string, credential string) (*model.File, *model.PhysicalFile, error) {
	if !s.verifyCredential(token, credential) {
		return nil, nil, ErrInvalidCredential
	}

	share, err := s.shareRepo.FindByToken(ctx, token)
	if err != nil {
		return nil, nil, ErrShareNotFound
	}

	if err := s.checkShareValidity(share); err != nil {
		return nil, nil, err
	}

	ok, err := s.shareRepo.IncrementDownloadCount(ctx, token)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, ErrShareLimitReached
	}

	file, err := s.fileRepo.FindByID(ctx, share.FileID)
	if err != nil {
		return nil, nil, ErrFileNotFound
	}

	if file.IsFolder {
		// Folder downloads are handled separately by the handler via StreamFolderZipByID
		return file, nil, nil
	}

	if file.ContentRef == 0 {
		return nil, nil, ErrNoPhysicalContent
	}

	pf, err := s.physicalRepo.FindByID(ctx, file.ContentRef)
	if err != nil {
		return nil, nil, err
	}

	return file, pf, nil
}

func (s *ShareService) ListMyShares(ctx context.Context, userID int64) ([]model.FileShare, error) {
	return s.shareRepo.FindByOwner(ctx, userID)
}

func (s *ShareService) ListFileShares(ctx context.Context, userID, fileID int64) ([]model.FileShare, error) {
	shares, err := s.shareRepo.FindByFile(ctx, fileID)
	if err != nil {
		return nil, err
	}
	// Filter to only return shares owned by the requesting user
	var result []model.FileShare
	for _, share := range shares {
		if share.OwnerID == userID {
			result = append(result, share)
		}
	}
	return result, nil
}

func (s *ShareService) RevokeShare(ctx context.Context, userID, shareID int64) error {
	return s.shareRepo.Revoke(ctx, shareID, userID)
}

// --- internal helpers ---

func (s *ShareService) checkShareValidity(share *model.FileShare) error {
	if share.Revoked {
		return ErrShareRevoked
	}
	if share.ExpiresAt.Valid && !share.ExpiresAt.Time.IsZero() && !share.ExpiresAt.Time.After(time.Now()) {
		return ErrShareExpired
	}
	if share.MaxDownloads > 0 && share.DownloadCount >= share.MaxDownloads {
		return ErrShareLimitReached
	}
	return nil
}

func (s *ShareService) generateToken() (string, error) {
	cfg := config.Get()
	length := cfg.Share.TokenLength
	if length <= 0 {
		length = 8
	}
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		if err != nil {
			return "", err
		}
		result[i] = chars[n.Int64()]
	}
	return string(result), nil
}

func (s *ShareService) generateCredential(token string) string {
	cfg := config.Get()
	ts := time.Now().Unix()
	message := fmt.Sprintf("%s.%d", token, ts)
	mac := hmac.New(sha256.New, []byte(cfg.Share.Secret))
	mac.Write([]byte(message))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%d.%s", token, ts, sig)
}

func (s *ShareService) verifyCredential(token string, credential string) bool {
	cfg := config.Get()
	credTTL := time.Duration(cfg.Share.CredentialTTL) * time.Second
	if credTTL <= 0 {
		credTTL = 300 * time.Second
	}

	// Parse credential: token.timestamp.signature
	var credToken, sigHex string
	var ts int64
	n, err := fmt.Sscanf(credential, "%s.%d.%s", &credToken, &ts, &sigHex)
	if err != nil || n != 3 || credToken != token {
		return false
	}

	// Check timestamp is within ±TTL
	now := time.Now().Unix()
	diff := now - ts
	if diff < 0 {
		diff = -diff
	}
	if diff > int64(credTTL.Seconds()) {
		return false
	}

	// Verify HMAC
	message := fmt.Sprintf("%s.%d", token, ts)
	mac := hmac.New(sha256.New, []byte(cfg.Share.Secret))
	mac.Write([]byte(message))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(sigHex), []byte(expectedSig))
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: compiles with no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/service/share.go
git commit -m "feat: add ShareService with HMAC credential and full CRUD"
```

---

### Task 6: Create ShareHandler

**Files:**
- Create: `internal/handler/share.go`

- [ ] **Step 1: Create ShareHandler**

Create `internal/handler/share.go`:

```go
package handler

import (
	"cloudbox/internal/model"
	"cloudbox/internal/service"
	"cloudbox/internal/util/response"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type ShareHandler struct {
	shareService *service.ShareService
	fileService  *service.FileService
}

func NewShareHandler(shareService *service.ShareService, fileService *service.FileService) *ShareHandler {
	return &ShareHandler{
		shareService: shareService,
		fileService:  fileService,
	}
}

// --- Public handlers (no JWT) ---

func (h *ShareHandler) GetShareInfo(c *gin.Context) {
	token := c.Param("token")
	share, file, err := h.shareService.GetShareInfo(c.Request.Context(), token)
	if err != nil {
		switch err {
		case service.ErrShareNotFound:
			response.NotFound(c, err.Error())
		case service.ErrShareExpired:
			response.Error(c, http.StatusGone, err.Error())
		case service.ErrShareRevoked:
			response.Error(c, http.StatusGone, err.Error())
		case service.ErrShareLimitReached:
			response.Error(c, http.StatusGone, err.Error())
		default:
			response.InternalError(c, err.Error())
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"fileName":    file.Name,
			"fileSize":    h.getFileSize(file),
			"isFolder":    file.IsFolder,
			"hasPassword": share.PasswordHash.Valid,
			"createdAt":   share.CreatedAt,
			"expiresAt":   share.ExpiresAt,
		},
	})
}

func (h *ShareHandler) VerifyShare(c *gin.Context) {
	token := c.Param("token")
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Password = ""
	}

	cred, err := h.shareService.VerifyOrGetCredential(c.Request.Context(), token, req.Password)
	if err != nil {
		switch err {
		case service.ErrShareNotFound:
			response.NotFound(c, err.Error())
		case service.ErrShareExpired, service.ErrShareRevoked, service.ErrShareLimitReached:
			response.Error(c, http.StatusGone, err.Error())
		case service.ErrWrongSharePassword:
			response.Error(c, http.StatusForbidden, err.Error())
		default:
			response.InternalError(c, err.Error())
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"credential": cred.Credential,
		},
	})
}

func (h *ShareHandler) DownloadByShare(c *gin.Context) {
	token := c.Param("token")
	credential := c.Query("t")
	if credential == "" {
		response.BadRequest(c, "missing download credential")
		return
	}

	file, pf, err := h.shareService.DownloadByShare(c.Request.Context(), token, credential)
	if err != nil {
		switch err {
		case service.ErrInvalidCredential:
			response.Error(c, http.StatusForbidden, err.Error())
		case service.ErrShareNotFound:
			response.NotFound(c, err.Error())
		case service.ErrShareExpired, service.ErrShareRevoked, service.ErrShareLimitReached:
			response.Error(c, http.StatusGone, err.Error())
		default:
			response.InternalError(c, err.Error())
		}
		return
	}

	if file.IsFolder {
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"; filename*=UTF-8''%s.zip`,
			file.Name, encodeRFC5987(file.Name)))
		c.Header("Content-Type", "application/zip")
		if err := h.fileService.StreamFolderZipByID(c.Request.Context(), file.ID, c.Writer); err != nil {
			// Header already sent, can't change status
			return
		}
		return
	}

	absPath := h.fileService.GetStorage().ToAbsPath(pf.StoragePath)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		file.Name, encodeRFC5987(file.Name)))
	c.Header("Content-Type", pf.MimeType)
	c.Header("Content-Length", strconv.FormatInt(pf.Size, 10))
	c.File(absPath)
}

// --- Authenticated handlers (JWT) ---

func (h *ShareHandler) CreateShare(c *gin.Context) {
	userID := GetUserID(c)
	var req struct {
		FileID       int64  `json:"fileId" binding:"required"`
		Password     string `json:"password"`
		ExpiresIn    int    `json:"expiresIn"`    // seconds, 0 = never
		MaxDownloads int    `json:"maxDownloads"` // 0 = unlimited
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresIn) * time.Second)
		expiresAt = &t
	}

	share, err := h.shareService.CreateShare(c.Request.Context(), userID, req.FileID, req.Password, expiresAt, req.MaxDownloads)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"id":        share.ID,
			"token":     share.Token,
			"createdAt": share.CreatedAt,
			"expiresAt": share.ExpiresAt,
		},
	})
}

func (h *ShareHandler) ListFileShares(c *gin.Context) {
	userID := GetUserID(c)
	fileIDStr := c.Query("fileId")
	fileID, err := strconv.ParseInt(fileIDStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid fileId")
		return
	}

	shares, err := h.shareService.ListFileShares(c.Request.Context(), userID, fileID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": shares,
	})
}

func (h *ShareHandler) ListMyShares(c *gin.Context) {
	userID := GetUserID(c)
	shares, err := h.shareService.ListMyShares(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": shares,
	})
}

func (h *ShareHandler) RevokeShare(c *gin.Context) {
	userID := GetUserID(c)
	shareID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid share id")
		return
	}

	if err := h.shareService.RevokeShare(c.Request.Context(), userID, shareID); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0})
}

func (h *ShareHandler) getFileSize(file *model.File) int64 {
	if file.Physical != nil {
		return file.Physical.Size
	}
	return 0
}

func encodeRFC5987(s string) string {
	// Reuse the same RFC 5987 encoding as handler/file.go
	var buf []byte
	for _, b := range []byte(s) {
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '-' || b == '.' || b == '_' || b == '~' {
			buf = append(buf, b)
		} else {
			buf = append(buf, []byte(fmt.Sprintf("%%%02X", b))...)
		}
	}
	return string(buf)
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: compiles with no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/handler/share.go
git commit -m "feat: add ShareHandler with public and authenticated endpoints"
```

---

### Task 7: Wire up routes in main.go

**Files:**
- Modify: `cmd/server/main.go:34-57` (add share repo/service/handler)
- Modify: `cmd/server/main.go:83-151` (add route groups)

- [ ] **Step 1: Add share repository, service, and handler initialization**

In `cmd/server/main.go`, after `clipboardRepo` (line 37), add:

```go
shareRepo := repository.NewShareRepository(db)
```

After `clipboardService` (line 44), add:

```go
shareService := service.NewShareService(shareRepo, fileRepo, physicalRepo, storageManager, fileService, cryptoAdapter)
```

After `adminHandler` (line 56), add:

```go
shareHandler := handler.NewShareHandler(shareService, fileService)
```

- [ ] **Step 2: Add public share routes**

After the `api` group definition (after line 83), add the public share route group:

```go
// Public share routes (no JWT required)
sGroup := r.Group("/api/s")
{
	sGroup.GET("/:token", shareHandler.GetShareInfo)
	sGroup.POST("/:token/verify", shareHandler.VerifyShare)
	sGroup.GET("/:token/download", shareHandler.DownloadByShare)
}
```

- [ ] **Step 3: Add authenticated share routes**

Inside the `protected` group (after the upload routes, before admin routes), add:

```go
// Shares
protected.POST("/shares", shareHandler.CreateShare)
protected.GET("/shares", shareHandler.ListFileShares)
protected.GET("/shares/mine", shareHandler.ListMyShares)
protected.DELETE("/shares/:id", shareHandler.RevokeShare)
```

- [ ] **Step 4: Build and verify**

Run: `go build ./cmd/server`
Expected: compiles with no errors.

- [ ] **Step 5: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: wire share routes in main.go"
```

---

### Task 8: Write unit tests for ShareService

**Files:**
- Modify: `internal/service/mock_test.go` (add mockShareRepo)
- Create: `internal/service/share_test.go`

- [ ] **Step 1: Add mockShareRepo to mock_test.go**

Append to `internal/service/mock_test.go`:

```go
type mockShareRepo struct {
	createFn               func(ctx context.Context, share *model.FileShare) error
	findByTokenFn          func(ctx context.Context, token string) (*model.FileShare, error)
	findByOwnerFn          func(ctx context.Context, ownerID int64) ([]model.FileShare, error)
	findByFileFn           func(ctx context.Context, fileID int64) ([]model.FileShare, error)
	revokeFn               func(ctx context.Context, id, ownerID int64) error
	incrementDownloadCountFn func(ctx context.Context, token string) (bool, error)
}

func (m *mockShareRepo) Create(ctx context.Context, share *model.FileShare) error {
	if m.createFn == nil {
		panic("mockShareRepo.Create not set")
	}
	return m.createFn(ctx, share)
}
func (m *mockShareRepo) FindByToken(ctx context.Context, token string) (*model.FileShare, error) {
	if m.findByTokenFn == nil {
		panic("mockShareRepo.FindByToken not set")
	}
	return m.findByTokenFn(ctx, token)
}
func (m *mockShareRepo) FindByOwner(ctx context.Context, ownerID int64) ([]model.FileShare, error) {
	if m.findByOwnerFn == nil {
		panic("mockShareRepo.FindByOwner not set")
	}
	return m.findByOwnerFn(ctx, ownerID)
}
func (m *mockShareRepo) FindByFile(ctx context.Context, fileID int64) ([]model.FileShare, error) {
	if m.findByFileFn == nil {
		panic("mockShareRepo.FindByFile not set")
	}
	return m.findByFileFn(ctx, fileID)
}
func (m *mockShareRepo) Revoke(ctx context.Context, id, ownerID int64) error {
	if m.revokeFn == nil {
		panic("mockShareRepo.Revoke not set")
	}
	return m.revokeFn(ctx, id, ownerID)
}
func (m *mockShareRepo) IncrementDownloadCount(ctx context.Context, token string) (bool, error) {
	if m.incrementDownloadCountFn == nil {
		panic("mockShareRepo.IncrementDownloadCount not set")
	}
	return m.incrementDownloadCountFn(ctx, token)
}
```

- [ ] **Step 2: Create share_test.go**

Create `internal/service/share_test.go` with table-driven tests for:

1. `TestShareService_CreateShare`: file not found, success with password, success without password, success with expiration
2. `TestShareService_GetShareInfo`: not found, expired, revoked, limit reached, success
3. `TestShareService_VerifyOrGetCredential`: not found, expired, wrong password, success with password, success without password
4. `TestShareService_DownloadByShare`: invalid credential, expired share, limit reached, success file, success folder
5. `TestShareService_RevokeShare`: success

Each test constructs a `ShareService` with all mocks. For HMAC credential tests, use a known secret set in config defaults.

- [ ] **Step 3: Run tests**

Run: `go test ./internal/service/ -run TestShareService -v`
Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/service/mock_test.go internal/service/share_test.go
git commit -m "test: add ShareService unit tests"
```

---

### Task 9: Add shareAPI to frontend API client

**Files:**
- Modify: `web/src/utils/api.js:120-130` (add shareAPI module)

- [ ] **Step 1: Add shareAPI module**

In `web/src/utils/api.js`, after `adminAPI` (after line 128), add:

```js
export const shareAPI = {
  // Public (no auth header needed)
  getInfo: (token) => axios.get(`/s/${token}`),
  verify: (token, password = '') => axios.post(`/s/${token}/verify`, { password }),
  downloadUrl: (token, credential) => `${baseURL}/s/${token}/download?t=${encodeURIComponent(credential)}`,

  // Authenticated
  create: (data) => axios.post('/shares', data),
  listFile: (fileId) => axios.get('/shares', { params: { fileId } }),
  listMine: () => axios.get('/shares/mine'),
  revoke: (id) => axios.delete(`/shares/${id}`),
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/utils/api.js
git commit -m "feat: add shareAPI module to frontend API client"
```

---

### Task 10: Add share store and router

**Files:**
- Create: `web/src/stores/shares.js`
- Modify: `web/src/router/index.js:10-47` (add routes)
- Modify: `web/src/router/index.js:54-70` (add public route guard)

- [ ] **Step 1: Create shares store**

Create `web/src/stores/shares.js`:

```js
import { defineStore } from 'pinia'
import { shareAPI } from '../utils/api'
import { useAuthStore } from './auth'

export const useSharesStore = defineStore('shares', {
  state: () => ({
    myShares: [],
    fileShares: [],
    loading: false,
  }),
  actions: {
    async createShare(fileId, { password, expiresIn, maxDownloads } = {}) {
      const res = await shareAPI.create({ fileId, password: password || '', expiresIn: expiresIn || 0, maxDownloads: maxDownloads || 0 })
      return res.data
    },
    async fetchMyShares() {
      this.loading = true
      try {
        const res = await shareAPI.listMine()
        this.myShares = res.data || []
      } finally {
        this.loading = false
      }
    },
    async fetchFileShares(fileId) {
      const res = await shareAPI.listFile(fileId)
      this.fileShares = res.data || []
    },
    async revokeShare(id) {
      await shareAPI.revoke(id)
      this.myShares = this.myShares.filter(s => s.id !== id)
    },
    async getShareInfo(token) {
      const res = await shareAPI.getInfo(token)
      return res.data
    },
    async verifyShare(token, password = '') {
      const res = await shareAPI.verify(token, password)
      return res.data
    },
    getDownloadUrl(token, credential) {
      return shareAPI.downloadUrl(token, credential)
    },
  },
})
```

- [ ] **Step 2: Add routes to router**

In `web/src/router/index.js`, add these routes (after the admin route, around line 47):

```js
{
  path: '/s/:token',
  name: 'SharePreview',
  component: () => import('../views/SharePreview.vue'),
  meta: { public: true }
},
{
  path: '/shares',
  name: 'Shares',
  component: () => import('../views/SharesView.vue'),
  meta: { requiresAuth: true }
},
```

- [ ] **Step 3: Add public route guard**

In the `router.beforeEach` guard, add at the very beginning (before the token check):

```js
if (to.meta.public) {
  return next()
}
```

- [ ] **Step 4: Commit**

```bash
git add web/src/stores/shares.js web/src/router/index.js
git commit -m "feat: add shares store and router with public guard"
```

---

### Task 11: Add SharePreview page

**Files:**
- Create: `web/src/views/SharePreview.vue`

- [ ] **Step 1: Create SharePreview.vue**

Create `web/src/views/SharePreview.vue` — a standalone page (no AppHeader/Sidebar) with:
- Centered card layout, responsive (max-width: 480px, padding)
- On mount: call `sharesStore.getShareInfo(token)` to fetch file info
- If hasPassword: show password input + submit button
- If no password: auto-call `verifyShare(token)` to get credential
- After getting credential: show download button that opens `sharesStore.getDownloadUrl(token, credential)` in a new tab
- Error states with Chinese messages: "该分享已过期" / "该分享已被撤销" / "下载次数已达上限" / "分享不存在"
- Style: clean white card on gray background, Element Plus components

- [ ] **Step 2: Commit**

```bash
git add web/src/views/SharePreview.vue
git commit -m "feat: add SharePreview public page"
```

---

### Task 12: Add CreateShareDialog

**Files:**
- Create: `web/src/components/Dialogs/CreateShareDialog.vue`

- [ ] **Step 1: Create CreateShareDialog.vue**

Create `web/src/components/Dialogs/CreateShareDialog.vue` with:
- El-dialog, fullscreen on mobile (check `window.innerWidth < 768`)
- Props: `visible` (v-model), `file` (the file object to share)
- Expiration dropdown: 1小时(3600) / 1天(86400) / 7天(604800) / 永久(0)
- Optional password input (el-input type=password, with toggle visibility)
- Optional max downloads input (el-input-number, min=0, 0=unlimited)
- Submit: calls `sharesStore.createShare(file.id, { password, expiresIn, maxDownloads })`
- After success: show generated link in el-input (readonly) + copy button
- Copy with fallback: `navigator.clipboard.writeText` or `execCommand('copy')`
- Close: emit `update:visible` with false

- [ ] **Step 2: Commit**

```bash
git add web/src/components/Dialogs/CreateShareDialog.vue
git commit -m "feat: add CreateShareDialog component"
```

---

### Task 13: Add SharesView management page

**Files:**
- Create: `web/src/views/SharesView.vue`

- [ ] **Step 1: Create SharesView.vue**

Create `web/src/views/SharesView.vue` with:
- Uses AppHeader + AppSidebar layout (same as FilesView/TrashView)
- On mount: call `sharesStore.fetchMyShares()`
- El-table: columns for file name (resolved from fileId via a lookup or stored), created time, expires at, downloads (count/max), status
- Status computed: active (green), expired (gray), revoked (red), limit reached (orange)
- Actions per row: copy link (with clipboard fallback), revoke (with confirm dialog)
- Empty state: "暂无分享记录"

- [ ] **Step 2: Commit**

```bash
git add web/src/views/SharesView.vue
git commit -m "feat: add SharesView management page"
```

---

### Task 14: Add share action to context menus and FilesView

**Files:**
- Modify: `web/src/components/Files/FileGrid.vue:23-47` (context menu)
- Modify: `web/src/components/Files/FileList.vue:46-70` (context menu)
- Modify: `web/src/components/Files/MobileFileList.vue:49-73` (context menu)
- Modify: `web/src/views/FilesView.vue` (add dialog + handler)

- [ ] **Step 1: Add "分享" to FileGrid context menu**

In `FileGrid.vue`, add a menu item after "下载" (after line 34):

```html
<div class="context-menu-item" @click="emit('share', file)">
  <el-icon><Share /></el-icon>
  <span>分享</span>
</div>
```

Add `Share` to the icon imports and add `'share'` to the emits array.

- [ ] **Step 2: Add "分享" to FileList context menu**

Same change in `FileList.vue` — add after "下载" (after line 57).

- [ ] **Step 3: Add "分享" to MobileFileList context menu**

Same change in `MobileFileList.vue` — add after "下载" (after line 60).

- [ ] **Step 4: Handle share event in FilesView**

In `FilesView.vue`:
1. Import and register `CreateShareDialog` component
2. Add reactive state: `shareDialogVisible = ref(false)`, `shareFile = ref(null)`
3. Add `handleShareFile(file)` handler that sets `shareFile` and opens dialog
4. Wire `@share="handleShareFile"` on all three file list components
5. Add `<CreateShareDialog v-model:visible="shareDialogVisible" :file="shareFile" />` in template

- [ ] **Step 5: Commit**

```bash
git add web/src/components/Files/FileGrid.vue web/src/components/Files/FileList.vue web/src/components/Files/MobileFileList.vue web/src/views/FilesView.vue
git commit -m "feat: add share action to file context menus and FilesView"
```

---

### Task 15: Add navigation entries for shares

**Files:**
- Modify: `web/src/components/Layout/AppSidebar.vue:6-39` (add nav item)
- Modify: `web/src/components/Layout/MobileTabBar.vue:5-37` (add tab)

- [ ] **Step 1: Add "我的分享" to AppSidebar**

In `AppSidebar.vue`, after the clipboard nav item (after line 28), add:

```html
<router-link to="/shares" class="nav-item" active-class="active">
  <el-icon><Share /></el-icon>
  <span>我的分享</span>
</router-link>
```

Import `Share` icon from `@element-plus/icons-vue`.

- [ ] **Step 2: Add "分享" tab to MobileTabBar**

In `MobileTabBar.vue`, after the clipboard tab (after line 18), add:

```html
<router-link to="/shares" class="tab-item" active-class="active">
  <el-icon><Share /></el-icon>
  <span>分享</span>
</router-link>
```

Import `Share` icon from `@element-plus/icons-vue`.

- [ ] **Step 3: Commit**

```bash
git add web/src/components/Layout/AppSidebar.vue web/src/components/Layout/MobileTabBar.vue
git commit -m "feat: add share navigation to sidebar and mobile tab bar"
```

---

### Task 16: End-to-end verification

- [ ] **Step 1: Build backend**

Run: `go build ./cmd/server`
Expected: compiles with no errors.

- [ ] **Step 2: Run all tests**

Run: `go test ./... -v`
Expected: all tests pass.

- [ ] **Step 3: Build frontend**

Run: `cd web && npm run build`
Expected: build succeeds.

- [ ] **Step 4: Start server and smoke test**

1. Start the server
2. Login as admin
3. Upload a file
4. Right-click the file → click "分享"
5. In the dialog: set expiration to 1 day, set a password, click create
6. Copy the share link
7. Open the share link in an incognito browser
8. Verify the preview page shows file name and password input
9. Enter the password, click verify
10. Click download — verify file downloads
11. Go to "我的分享" page — verify the share is listed
12. Click revoke — verify the share link returns "已撤销" when accessed

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "feat: complete file sharing feature with public links"
```
