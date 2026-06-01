# Audit Operation Logs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add async buffered audit logging for admin + user key operations, with admin-only query API and frontend viewer.

**Architecture:** AuditService receives Record calls from service methods, pushes to a buffered channel, and a background goroutine batch-writes to DB. JWTMiddleware injects user context for automatic extraction. Admin handler exposes a paginated query endpoint. Frontend adds an audit log viewer page.

**Tech Stack:** Go (Gin + GORM), Vue 3 + Element Plus, SQLite/MySQL dual DDL

---

### Task 1: AuditLog model + DB migration

**Files:**
- Create: `internal/model/audit.go`
- Modify: `internal/repository/db.go`

- [ ] **Step 1: Create AuditLog model**

Create `internal/model/audit.go`:

```go
package model

import "time"

type AuditLog struct {
	ID         int64     `gorm:"primaryKey" json:"id"`
	UserID     int64     `gorm:"not null;index:idx_audit_user" json:"userId"`
	Username   string    `gorm:"size:50;not null" json:"username"`
	Action     string    `gorm:"size:50;not null;index:idx_audit_action" json:"action"`
	TargetType string    `gorm:"size:30;not null" json:"targetType"`
	TargetID   *int64    `json:"targetId"`
	TargetName string    `gorm:"size:255" json:"targetName"`
	Detail     string    `json:"detail"`
	IPAddress  string    `gorm:"size:45" json:"ipAddress"`
	CreatedAt  time.Time `gorm:"not null;index:idx_audit_time" json:"createdAt"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}
```

- [ ] **Step 2: Add createAuditLogsTable to db.go**

Add after `createFileSharesTable` in `internal/repository/db.go`:

```go
func createAuditLogsTable() error {
	var count int64
	if DB.Dialector.Name() == "sqlite" {
		DB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='audit_logs'").Scan(&count)
	} else {
		DB.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'cloudbox' AND table_name = 'audit_logs'").Scan(&count)
	}
	if count > 0 {
		return nil
	}

	if DB.Dialector.Name() == "sqlite" {
		sql := `CREATE TABLE audit_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			username VARCHAR(50) NOT NULL,
			action VARCHAR(50) NOT NULL,
			target_type VARCHAR(30) NOT NULL,
			target_id INTEGER,
			target_name VARCHAR(255),
			detail TEXT,
			ip_address VARCHAR(45),
			created_at DATETIME NOT NULL
		)`
		if err := DB.Exec(sql).Error; err != nil {
			return err
		}
		DB.Exec("CREATE INDEX idx_audit_user ON audit_logs(user_id)")
		DB.Exec("CREATE INDEX idx_audit_action ON audit_logs(action)")
		DB.Exec("CREATE INDEX idx_audit_time ON audit_logs(created_at)")
	} else {
		sql := `CREATE TABLE audit_logs (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			user_id BIGINT NOT NULL,
			username VARCHAR(50) NOT NULL,
			action VARCHAR(50) NOT NULL,
			target_type VARCHAR(30) NOT NULL,
			target_id BIGINT,
			target_name VARCHAR(255),
			detail TEXT,
			ip_address VARCHAR(45),
			created_at DATETIME NOT NULL,
			INDEX idx_audit_user (user_id),
			INDEX idx_audit_action (action),
			INDEX idx_audit_time (created_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
		if err := DB.Exec(sql).Error; err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 3: Call createAuditLogsTable from InitDB**

In `internal/repository/db.go`, add after the `createFileSharesTable` call:

```go
		if err := createAuditLogsTable(); err != nil {
			log.Fatalf("failed to create audit_logs table: %v", err)
		}
```

- [ ] **Step 4: Verify build**

Run: `cd /e/fileManagementIntranet && go build ./...`
Expected: success (no output)

- [ ] **Step 5: Commit**

```bash
git add internal/model/audit.go internal/repository/db.go
git commit -m "feat: add AuditLog model and DB migration"
```

---

### Task 2: AuditRepository + interface

**Files:**
- Create: `internal/repository/audit.go`
- Modify: `internal/service/interfaces.go`

- [ ] **Step 1: Create AuditRepository**

Create `internal/repository/audit.go`:

```go
package repository

import (
	"cloudbox/internal/model"
	"context"
	"time"

	"gorm.io/gorm"
)

type AuditRepository struct {
	db *gorm.DB
}

func NewAuditRepository(db *gorm.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) BatchCreate(ctx context.Context, logs []model.AuditLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&logs).Error
}

func (r *AuditRepository) FindWithFilter(ctx context.Context, action string, userID int64, targetType string, keyword string, startDate, endDate *time.Time, page, pageSize int) ([]model.AuditLog, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}

	query := r.db.WithContext(ctx).Model(&model.AuditLog{})

	if action != "" {
		query = query.Where("action = ?", action)
	}
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if targetType != "" {
		query = query.Where("target_type = ?", targetType)
	}
	if keyword != "" {
		query = query.Where("target_name LIKE ?", "%"+keyword+"%")
	}
	if startDate != nil {
		query = query.Where("created_at >= ?", startDate)
	}
	if endDate != nil {
		query = query.Where("created_at <= ?", endDate)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var logs []model.AuditLog
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

func (r *AuditRepository) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Where("created_at < ?", before).Delete(&model.AuditLog{})
	return result.RowsAffected, result.Error
}
```

- [ ] **Step 2: Add AuditRepository + AuditRecorder interfaces**

Add to `internal/service/interfaces.go`:

```go
type AuditRepository interface {
	BatchCreate(ctx context.Context, logs []model.AuditLog) error
	FindWithFilter(ctx context.Context, action string, userID int64, targetType string, keyword string, startDate, endDate *time.Time, page, pageSize int) ([]model.AuditLog, int64, error)
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
}

type AuditRecorder interface {
	Record(ctx context.Context, action, targetType string, targetID int64, targetName, detail string)
}
```

No shared filter struct needed — the interface uses simple parameters to avoid import cycles. The repository implementation wraps these into a GORM query internally.

- [ ] **Step 3: Verify build**

Run: `cd /e/fileManagementIntranet && go build ./...`
Expected: success

- [ ] **Step 4: Commit**

```bash
git add internal/repository/audit.go internal/service/interfaces.go
git commit -m "feat: add AuditRepository and AuditRecorder interface"
```

---

### Task 3: AuditService (channel + batch write + graceful shutdown)

**Files:**
- Create: `internal/service/audit.go`

- [ ] **Step 1: Create AuditService**

Create `internal/service/audit.go`:

```go
package service

import (
	"cloudbox/internal/model"
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const (
	auditChannelSize  = 256
	auditBatchSize    = 50
	auditFlushInterval = 500 * time.Millisecond
	maxDetailLen      = 4096
)

type auditEntry struct {
	UserID     int64
	Username   string
	Action     string
	TargetType string
	TargetID   int64
	TargetName string
	Detail     string
	IPAddress  string
	CreatedAt  time.Time
}

type AuditService struct {
	repo          AuditRepository
	ch            chan *auditEntry
	wg            sync.WaitGroup
	dropped       atomic.Int64
	closed        atomic.Bool
}

func NewAuditService(repo AuditRepository) *AuditService {
	s := &AuditService{
		repo: repo,
		ch:   make(chan *auditEntry, auditChannelSize),
	}
	s.wg.Add(1)
	go s.consume()
	return s
}

func (s *AuditService) Record(ctx context.Context, action, targetType string, targetID int64, targetName, detail string) {
	if s.closed.Load() {
		s.dropped.Add(1)
		return
	}

	entry := &auditEntry{
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		TargetName: targetName,
		Detail:     detail,
		CreatedAt:  time.Now(),
	}

	if len(entry.Detail) > maxDetailLen {
		entry.Detail = entry.Detail[:maxDetailLen]
	}

	// Extract user info from context
	if v, ok := ctx.Value("userID").(int64); ok {
		entry.UserID = v
	}
	if v, ok := ctx.Value("username").(string); ok {
		entry.Username = v
	}
	if v, ok := ctx.Value("clientIP").(string); ok {
		entry.IPAddress = v
	}

	select {
	case s.ch <- entry:
	default:
		s.dropped.Add(1)
		log.Printf("[audit] channel full, dropped entry: action=%s target=%s", action, targetName)
	}
}

func (s *AuditService) Shutdown() {
	s.closed.Store(true)
	close(s.ch)
	s.wg.Wait()
}

func (s *AuditService) DroppedCount() int64 {
	return s.dropped.Load()
}

func (s *AuditService) consume() {
	defer s.wg.Done()

	batch := make([]model.AuditLog, 0, auditBatchSize)
	ticker := time.NewTicker(auditFlushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.repo.BatchCreate(context.Background(), batch); err != nil {
			log.Printf("[audit] batch write failed (%d entries): %v", len(batch), err)
			s.dropped.Add(int64(len(batch)))
		}
		batch = batch[:0]
	}

	for {
		select {
		case entry, ok := <-s.ch:
			if !ok {
				// Channel closed, flush remaining
				flush()
				return
			}
			batch = append(batch, model.AuditLog{
				UserID:     entry.UserID,
				Username:   entry.Username,
				Action:     entry.Action,
				TargetType: entry.TargetType,
				TargetID:   &entry.TargetID,
				TargetName: entry.TargetName,
				Detail:     entry.Detail,
				IPAddress:  entry.IPAddress,
				CreatedAt:  entry.CreatedAt,
			})
			if len(batch) >= auditBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
```

- [ ] **Step 2: Verify build**

Run: `cd /e/fileManagementIntranet && go build ./...`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add internal/service/audit.go
git commit -m "feat: add AuditService with async buffered batch writes"
```

---

### Task 4: Inject context in JWTMiddleware

**Files:**
- Modify: `internal/handler/middleware.go`

- [ ] **Step 1: Add context value injection**

In `internal/handler/middleware.go`, after the existing `c.Set("role", claims.Role)` line, add:

```go
			// Inject user info into request context for audit logging
			ctx := context.WithValue(c.Request.Context(), "userID", claims.UserID)
			ctx = context.WithValue(ctx, "username", claims.Username)
			ctx = context.WithValue(ctx, "clientIP", c.ClientIP())
			c.Request = c.Request.WithContext(ctx)
```

Also add `"context"` to the import list.

- [ ] **Step 2: Verify build**

Run: `cd /e/fileManagementIntranet && go build ./...`
Expected: success

- [ ] **Step 3: Commit**

```bash
git add internal/handler/middleware.go
git commit -m "feat: inject userID/username/clientIP into request context for audit"
```

---

### Task 5: Add AuditService to service constructors + wire in main.go

**Files:**
- Modify: `internal/service/auth.go`
- Modify: `internal/service/file.go`
- Modify: `internal/service/upload.go`
- Modify: `internal/service/share.go`
- Modify: `internal/service/clipboard.go`
- Modify: `internal/service/admin.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add audit field to AuthService**

In `internal/service/auth.go`, add `audit AuditRecorder` field to `AuthService` struct and `audit AuditRecorder` param to `NewAuthService`. Store it: `audit: audit`.

Update constructor:

```go
func NewAuthService(
	userRepo UserRepository,
	hasher PasswordHasher,
	tokenGen TokenGenerator,
	registration bool,
	approvalReq bool,
	adminPassword string,
	audit AuditRecorder,
) *AuthService {
	return &AuthService{
		userRepo:      userRepo,
		hasher:        hasher,
		tokenGen:      tokenGen,
		registration:  registration,
		approvalReq:   approvalReq,
		adminPassword: adminPassword,
		audit:         audit,
	}
}
```

- [ ] **Step 2: Add audit field to FileService**

In `internal/service/file.go`, add `audit AuditRecorder` field to `FileService` struct and `audit AuditRecorder` param to `NewFileService`. Store it: `audit: audit`.

- [ ] **Step 3: Add audit field to UploadService**

In `internal/service/upload.go`, add `audit AuditRecorder` field to `UploadService` struct and `audit AuditRecorder` param to `NewUploadService`. Store it: `audit: audit`.

- [ ] **Step 4: Add audit field to ShareService**

In `internal/service/share.go`, add `audit AuditRecorder` field to `ShareService` struct and `audit AuditRecorder` param to `NewShareService`. Store it: `audit: audit`.

- [ ] **Step 5: Add audit field to ClipboardService**

In `internal/service/clipboard.go`, add `audit AuditRecorder` field to `ClipboardService` struct and `audit AuditRecorder` param to `NewClipboardService`. Store it: `audit: audit`.

- [ ] **Step 6: Add audit field to AdminService**

In `internal/service/admin.go`, add `audit AuditRecorder` field to `AdminService` struct and `audit AuditRecorder` param to `NewAdminService`. Store it: `audit: audit`.

- [ ] **Step 7: Update main.go wiring**

In `cmd/server/main.go`, create AuditService early and pass it to all service constructors:

```go
		// Initialize audit service
		auditRepo := repository.NewAuditRepository(db)
		auditService := service.NewAuditService(auditRepo)

		// Initialize services
		cryptoAdapter := service.NewCryptoAdapter()
		authService := service.NewAuthService(userRepo, cryptoAdapter, cryptoAdapter, cfg.Auth.Registration, cfg.Auth.ApprovalRequired, cfg.Admin.Password, auditService)
		previewService := service.NewPreviewService(physicalRepo, fileRepo, storageManager)
		fileService := service.NewFileService(fileRepo, physicalRepo, storageManager, auditService)
		uploadService := service.NewUploadService(fileRepo, physicalRepo, userRepo, storageManager, previewService, cfg.Upload.ChunkSize, cfg.Disk.DefaultQuota, auditService)
		clipboardService := service.NewClipboardService(clipboardRepo, auditService)
		shareService := service.NewShareService(shareRepo, fileRepo, physicalRepo, storageManager, fileService, cryptoAdapter, auditService)

		// Initialize admin service and handler
		adminService := service.NewAdminService(userRepo, fileRepo, physicalRepo, clipboardRepo, fileService, cryptoAdapter, auditService)
```

- [ ] **Step 8: Verify build**

Run: `cd /e/fileManagementIntranet && go build ./...`
Expected: success

- [ ] **Step 9: Fix test files for new constructor params**

Update `internal/service/auth_test.go`, `internal/service/file_test.go`, `internal/service/upload_test.go` to pass a no-op AuditRecorder. Create a helper in mock_test.go:

Add to `internal/service/mock_test.go`:

```go
type noopAuditRecorder struct{}

func (noopAuditRecorder) Record(ctx context.Context, action, targetType string, targetID int64, targetName, detail string) {}

var noopAudit = noopAuditRecorder{}
```

Then update all test constructors to pass `noopAudit` as the last argument.

- [ ] **Step 10: Verify tests pass**

Run: `cd /e/fileManagementIntranet && go test ./internal/service/ -v -count=1`
Expected: all tests PASS

- [ ] **Step 11: Commit**

```bash
git add internal/service/ cmd/server/main.go
git commit -m "feat: inject AuditRecorder into all services"
```

---

### Task 6: Add audit Record calls to all service methods (22 call sites)

**Files:**
- Modify: `internal/service/auth.go`
- Modify: `internal/service/file.go`
- Modify: `internal/service/upload.go`
- Modify: `internal/service/share.go`
- Modify: `internal/service/clipboard.go`
- Modify: `internal/service/admin.go`

Each Record call is one line, added after the successful operation. Pattern:

```go
s.audit.Record(ctx, "action.name", "targetType", targetID, "targetName", `{"key":"value"}`)
```

- [ ] **Step 1: AuthService audit calls**

In `internal/service/auth.go`:

After successful login (before return in `Login`):
```go
s.audit.Record(ctx, "user.login", "user", user.ID, user.Username, "")
```

On login failure (after `!s.hasher.CheckPassword` returns true, before return):
```go
s.audit.Record(ctx, "user.login_failed", "user", 0, username, "")
```

After successful `ChangePassword` (before return):
```go
s.audit.Record(ctx, "user.change_password", "user", userID, "", "")
```

- [ ] **Step 2: FileService audit calls**

In `internal/service/file.go`:

After `CreateFolder` success (before return):
```go
s.audit.Record(ctx, "folder.create", "folder", file.ID, file.Name, "")
```

After `Rename` success (before return):
```go
s.audit.Record(ctx, "file.rename", targetType, file.ID, file.Name, fmt.Sprintf(`{"oldName":"%s"}`, oldName))
```

Where `targetType` is `"folder"` if `file.IsFolder` else `"file"`.

After `MoveToTrash` success (before return):
```go
targetType := "file"
if file.IsFolder {
	targetType = "folder"
}
s.audit.Record(ctx, "file.delete", targetType, file.ID, file.Name, "")
```

After `RestoreFile` success (before return):
```go
s.audit.Record(ctx, "file.restore", "file", id, file.Name, "")
```

After `PermanentDelete` success (for each file deleted in the loop where ref count reaches 0, after decrement):
```go
targetType := "file"
if f.IsFolder {
	targetType = "folder"
}
s.audit.Record(ctx, "file.permanent_delete", targetType, f.ID, f.Name, "")
```

After `MoveFiles` success (before return):
```go
s.audit.Record(ctx, "file.move", "file", 0, "", fmt.Sprintf(`{"count":%d,"targetFolder":%d}`, len(fileIDs), targetFolderID))
```

After `DownloadFile` success (before return):
```go
s.audit.Record(ctx, "file.download", "file", file.ID, file.Name, "")
```

After `EmptyTrash` success (in the method, before return). Note: `EmptyTrash` may not exist yet; check file.go. If it does, add:
```go
s.audit.Record(ctx, "trash.empty", "trash", 0, "", fmt.Sprintf(`{"count":%d}`, len(files)))
```

- [ ] **Step 3: UploadService audit calls**

In `internal/service/upload.go`:

After instant upload success in `InitUpload` (before return):
```go
s.audit.Record(ctx, "file.upload", "file", file.ID, req.FileName, fmt.Sprintf(`{"size":%d,"instant":true}`, req.FileSize))
```

After `CompleteUpload` success (before return):
```go
s.audit.Record(ctx, "file.upload", "file", file.ID, req.FileName, fmt.Sprintf(`{"size":%d,"instant":false}`, req.FileSize))
```

- [ ] **Step 4: ShareService audit calls**

In `internal/service/share.go`:

After `CreateShare` success (before return):
```go
s.audit.Record(ctx, "share.create", "share", share.ID, fmt.Sprintf("token=%s", share.Token), fmt.Sprintf(`{"fileId":%d,"hasPassword":%v}`, fileID, password != ""))
```

After `RevokeShare` success (before return):
```go
s.audit.Record(ctx, "share.revoke", "share", shareID, "", "")
```

After `DownloadByShare` success (before return):
```go
s.audit.Record(ctx, "share.download", "share", share.ID, file.Name, fmt.Sprintf(`{"token":"%s"}`, token))
```

- [ ] **Step 5: ClipboardService audit calls**

In `internal/service/clipboard.go`:

After `Create` success (before return):
```go
s.audit.Record(ctx, "clipboard.create", "clipboard", record.ID, record.DeviceName, "")
```

After `Delete` success (before return):
```go
s.audit.Record(ctx, "clipboard.delete", "clipboard", recordID, "", "")
```

- [ ] **Step 6: AdminService audit calls**

In `internal/service/admin.go`:

After `CreateUser` success (before return):
```go
s.audit.Record(ctx, "admin.create_user", "user", user.ID, user.Username, fmt.Sprintf(`{"role":"%s"}`, role))
```

After `DeleteUser` success (before return):
```go
s.audit.Record(ctx, "admin.delete_user", "user", userID, "", fmt.Sprintf(`{"deletedFiles":%d,"deletedFolders":%d}`, result.DeletedFiles, result.DeletedFolders))
```

After `ResetPassword` success (before return):
```go
s.audit.Record(ctx, "admin.reset_password", "user", userID, "", "")
```

After `UpdateUser` success, when status changed (inside the `if status != ""` block, after UpdateStatus):
```go
s.audit.Record(ctx, "admin.update_status", "user", userID, "", fmt.Sprintf(`{"status":"%s"}`, status))
```

After `SetUserQuota` success (before return):
```go
quotaStr := "global"
if quota != nil {
	if *quota == 0 {
		quotaStr = "unlimited"
	} else {
		quotaStr = fmt.Sprintf("%d", *quota)
	}
}
s.audit.Record(ctx, "admin.set_quota", "user", userID, "", fmt.Sprintf(`{"quota":"%s"}`, quotaStr))
```

- [ ] **Step 7: Verify build + tests**

Run: `cd /e/fileManagementIntranet && go build ./... && go test ./internal/service/ -count=1`
Expected: build success, all tests pass

- [ ] **Step 8: Commit**

```bash
git add internal/service/
git commit -m "feat: add audit Record calls to all 22 service operations"
```

---

### Task 7: Admin handler + route for audit logs query

**Files:**
- Modify: `internal/handler/admin.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add ListAuditLogs handler**

In `internal/handler/admin.go`, add a new method to `AdminHandler`:

```go
func (h *AdminHandler) ListAuditLogs(c *gin.Context) {
	action := c.Query("action")
	targetType := c.Query("targetType")
	keyword := c.Query("keyword")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))

	var userID int64
	if v := c.Query("userId"); v != "" {
		userID, _ = strconv.ParseInt(v, 10, 64)
	}

	var startDate, endDate *time.Time
	if v := c.Query("startDate"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			startDate = &t
		}
	}
	if v := c.Query("endDate"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			endDate = &t
		}
	}

	logs, total, err := h.adminService.ListAuditLogs(c.Request.Context(), action, userID, targetType, keyword, startDate, endDate, page, pageSize)
	if err != nil {
		response.InternalError(c, "failed to query audit logs")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"logs":     logs,
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		},
	})
}
```

Add `"time"` to the import list in `admin.go`.

- [ ] **Step 2: Add ListAuditLogs method to AdminService**

In `internal/service/admin.go`, add:

```go
func (s *AdminService) ListAuditLogs(ctx context.Context, action string, userID int64, targetType string, keyword string, startDate, endDate *time.Time, page, pageSize int) ([]model.AuditLog, int64, error) {
	return s.auditRepo.FindWithFilter(ctx, action, userID, targetType, keyword, startDate, endDate, page, pageSize)
}
```

Add `auditRepo AuditRepository` field to `AdminService` struct and `auditRepo AuditRepository` param to `NewAdminService`. Note: this is the repository (for querying), separate from the `audit AuditRecorder` field (for recording).

Update `NewAdminService` to accept and store `auditRepo AuditRepository` as well.

- [ ] **Step 3: Update main.go wiring**

Update the AdminService constructor call in `cmd/server/main.go` to pass `auditRepo`:

```go
		adminService := service.NewAdminService(userRepo, fileRepo, physicalRepo, clipboardRepo, fileService, cryptoAdapter, auditService, auditRepo)
```

- [ ] **Step 4: Add route**

In `cmd/server/main.go`, add to the admin routes group:

```go
			admin.GET("/audit-logs", adminHandler.ListAuditLogs)
```

- [ ] **Step 5: Verify build**

Run: `cd /e/fileManagementIntranet && go build ./...`
Expected: success

- [ ] **Step 6: Commit**

```bash
git add internal/handler/admin.go internal/service/admin.go cmd/server/main.go
git commit -m "feat: add GET /api/admin/audit-logs endpoint"
```

---

### Task 8: AuditConfig + auto-cleanup goroutine + graceful shutdown

**Files:**
- Modify: `internal/config/config.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add AuditConfig**

In `internal/config/config.go`, add to `Config` struct:

```go
	Audit    AuditConfig    `yaml:"audit"`
```

Add the type:

```go
type AuditConfig struct {
	RetentionDays int `yaml:"retention_days"`
}
```

Add default in the `once.Do` block:

```go
				Audit: AuditConfig{
					RetentionDays: 90,
				},
```

- [ ] **Step 2: Add auto-cleanup goroutine**

In `internal/util/storage/path.go` (or a new `internal/service/audit_cleanup.go`), add:

Actually, keep it in `cmd/server/main.go` since it's startup wiring. Add a function:

```go
func startAuditCleanup(auditRepo *repository.AuditRepository, retentionDays int) {
	if retentionDays <= 0 {
		return
	}

	go func() {
		// First cleanup after 2 minutes
		time.Sleep(2 * time.Minute)
		cleanupAuditLogs(auditRepo, retentionDays)

		// Then daily at midnight
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
```

Add necessary imports to `main.go`: `"time"`, `"context"`.

Call in `main()` after auditRepo creation:

```go
		startAuditCleanup(auditRepo, cfg.Audit.RetentionDays)
```

- [ ] **Step 3: Add graceful shutdown**

In `cmd/server/main.go`, change from `r.Run(addr)` to using `http.Server` with shutdown:

Replace:
```go
		log.Printf("Server starting at http://%s", addr)
		if err := r.Run(addr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
```

With:
```go
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

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}

		auditService.Shutdown()
		log.Println("Server exited")
```

Add imports: `"net/http"`, `"os"`, `"os/signal"`, `"syscall"`.

- [ ] **Step 4: Verify build**

Run: `cd /e/fileManagementIntranet && go build ./...`
Expected: success

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go cmd/server/main.go
git commit -m "feat: add AuditConfig, auto-cleanup, and graceful shutdown"
```

---

### Task 9: Frontend — AdminAuditView + route + sidebar

**Files:**
- Create: `web/src/views/AdminAuditView.vue`
- Modify: `web/src/router/index.js`
- Modify: `web/src/components/Layout/AppSidebar.vue`

- [ ] **Step 1: Create AdminAuditView**

Create `web/src/views/AdminAuditView.vue`:

```vue
<template>
  <div class="admin-audit-view">
    <AppHeader />
    <div class="files-layout">
      <AppSidebar />
      <main class="files-main">
        <div class="page-header">
          <h2>审计日志</h2>
        </div>

        <div class="filter-bar">
          <el-select v-model="filters.action" placeholder="操作类型" clearable style="width: 180px" @change="fetchLogs">
            <el-option-group label="认证">
              <el-option label="登录" value="user.login" />
              <el-option label="登录失败" value="user.login_failed" />
              <el-option label="修改密码" value="user.change_password" />
            </el-option-group>
            <el-option-group label="文件">
              <el-option label="上传" value="file.upload" />
              <el-option label="删除" value="file.delete" />
              <el-option label="恢复" value="file.restore" />
              <el-option label="永久删除" value="file.permanent_delete" />
              <el-option label="重命名" value="file.rename" />
              <el-option label="移动" value="file.move" />
              <el-option label="下载" value="file.download" />
            </el-option-group>
            <el-option-group label="文件夹">
              <el-option label="创建文件夹" value="folder.create" />
            </el-option-group>
            <el-option-group label="回收站">
              <el-option label="清空回收站" value="trash.empty" />
            </el-option-group>
            <el-option-group label="分享">
              <el-option label="创建分享" value="share.create" />
              <el-option label="撤销分享" value="share.revoke" />
              <el-option label="分享下载" value="share.download" />
            </el-option-group>
            <el-option-group label="剪切板">
              <el-option label="创建记录" value="clipboard.create" />
              <el-option label="删除记录" value="clipboard.delete" />
            </el-option-group>
            <el-option-group label="管理">
              <el-option label="创建用户" value="admin.create_user" />
              <el-option label="删除用户" value="admin.delete_user" />
              <el-option label="重置密码" value="admin.reset_password" />
              <el-option label="修改状态" value="admin.update_status" />
              <el-option label="设置配额" value="admin.set_quota" />
            </el-option-group>
          </el-select>

          <el-select v-model="filters.targetType" placeholder="目标类型" clearable style="width: 140px" @change="fetchLogs">
            <el-option label="文件" value="file" />
            <el-option label="文件夹" value="folder" />
            <el-option label="用户" value="user" />
            <el-option label="分享" value="share" />
            <el-option label="剪切板" value="clipboard" />
            <el-option label="回收站" value="trash" />
          </el-select>

          <el-input v-model="filters.keyword" placeholder="搜索目标名称" clearable style="width: 200px" @clear="fetchLogs" @keyup.enter="fetchLogs" />

          <el-date-picker
            v-model="filters.dateRange"
            type="datetimerange"
            range-separator="至"
            start-placeholder="开始时间"
            end-placeholder="结束时间"
            value-format="YYYY-MM-DDTHH:mm:ssZ"
            style="width: 380px"
            @change="fetchLogs"
          />

          <el-button type="primary" @click="fetchLogs">查询</el-button>
        </div>

        <el-table :data="logs" v-loading="loading" stripe>
          <el-table-column prop="createdAt" label="时间" width="180">
            <template #default="{ row }">{{ formatDate(row.createdAt) }}</template>
          </el-table-column>
          <el-table-column prop="username" label="操作者" width="120" />
          <el-table-column prop="action" label="操作" width="150">
            <template #default="{ row }">
              <el-tag :type="actionTagType(row.action)" size="small">{{ actionLabel(row.action) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="targetType" label="目标类型" width="100">
            <template #default="{ row }">{{ targetTypeLabel(row.targetType) }}</template>
          </el-table-column>
          <el-table-column prop="targetName" label="目标" min-width="150" />
          <el-table-column prop="ipAddress" label="IP" width="140" />
          <el-table-column prop="detail" label="详情" min-width="200">
            <template #default="{ row }">
              <span v-if="row.detail" class="detail-text">{{ row.detail }}</span>
              <span v-else class="detail-empty">-</span>
            </template>
          </el-table-column>
        </el-table>

        <div class="pagination-wrapper">
          <el-pagination
            v-model:current-page="page"
            v-model:page-size="pageSize"
            :total="total"
            :page-sizes="[20, 50, 100]"
            layout="total, sizes, prev, pager, next"
            @size-change="fetchLogs"
            @current-change="fetchLogs"
          />
        </div>
      </main>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import axios from 'axios'
import AppHeader from '@/components/Layout/AppHeader.vue'
import AppSidebar from '@/components/Layout/AppSidebar.vue'

const logs = ref([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)

const filters = reactive({
  action: '',
  targetType: '',
  keyword: '',
  dateRange: null
})

function formatDate(dateStr) {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  return d.toLocaleString('zh-CN')
}

function actionLabel(action) {
  const map = {
    'user.login': '登录', 'user.login_failed': '登录失败', 'user.change_password': '修改密码',
    'file.upload': '上传', 'file.delete': '删除', 'file.restore': '恢复',
    'file.permanent_delete': '永久删除', 'file.rename': '重命名', 'file.move': '移动',
    'file.download': '下载', 'folder.create': '创建文件夹', 'trash.empty': '清空回收站',
    'share.create': '创建分享', 'share.revoke': '撤销分享', 'share.download': '分享下载',
    'clipboard.create': '创建剪切板', 'clipboard.delete': '删除剪切板',
    'admin.create_user': '创建用户', 'admin.delete_user': '删除用户',
    'admin.reset_password': '重置密码', 'admin.update_status': '修改状态', 'admin.set_quota': '设置配额'
  }
  return map[action] || action
}

function actionTagType(action) {
  if (action.startsWith('admin.')) return 'danger'
  if (action.includes('delete') || action.includes('failed')) return 'warning'
  if (action.includes('create') || action.includes('upload') || action === 'user.login') return 'success'
  return 'info'
}

function targetTypeLabel(type) {
  const map = { file: '文件', folder: '文件夹', user: '用户', share: '分享', clipboard: '剪切板', trash: '回收站' }
  return map[type] || type
}

async function fetchLogs() {
  loading.value = true
  try {
    const params = { page: page.value, pageSize: pageSize.value }
    if (filters.action) params.action = filters.action
    if (filters.targetType) params.targetType = filters.targetType
    if (filters.keyword) params.keyword = filters.keyword
    if (filters.dateRange && filters.dateRange.length === 2) {
      params.startDate = filters.dateRange[0]
      params.endDate = filters.dateRange[1]
    }
    const res = await axios.get('/admin/audit-logs', { params })
    const data = res.data.data
    logs.value = data.logs || []
    total.value = data.total || 0
  } catch (err) {
    logs.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

onMounted(() => { fetchLogs() })
</script>

<style scoped lang="scss">
.admin-audit-view {
  height: 100vh;
  display: flex;
  flex-direction: column;
}

.files-layout {
  display: flex;
  flex: 1;
  overflow: hidden;
}

.files-main {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;

  h2 {
    margin: 0;
    font-size: 20px;
    color: #303133;
  }
}

.filter-bar {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 20px;
  align-items: center;
}

.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
}

.detail-text {
  font-size: 12px;
  color: #909399;
}

.detail-empty {
  color: #c0c4cc;
}
</style>
```

- [ ] **Step 2: Add route**

In `web/src/router/index.js`, import and add:

```js
import AdminAuditView from '@/views/AdminAuditView.vue'
```

Add route (after AdminUsers):

```js
  {
    path: '/admin/audit',
    name: 'AdminAudit',
    component: AdminAuditView,
    meta: { requiresAuth: true, requiresAdmin: true }
  },
```

- [ ] **Step 3: Add sidebar nav item**

In `web/src/components/Layout/AppSidebar.vue`, add after the "用户管理" router-link:

```vue
      <router-link
        v-if="authStore.isAdmin"
        to="/admin/audit"
        class="nav-item"
        :class="{ active: route.path === '/admin/audit' }"
      >
        <el-icon><List /></el-icon>
        <span>审计日志</span>
      </router-link>
```

Add `List` to the icons import:

```js
import { Folder, Delete, Document, User, Share, List } from '@element-plus/icons-vue'
```

- [ ] **Step 4: Build frontend**

Run: `export PATH="/c/Program Files/nodejs:$PATH" && cd /e/fileManagementIntranet/web && npm run build`
Expected: build success

- [ ] **Step 5: Commit**

```bash
git add web/src/views/AdminAuditView.vue web/src/router/index.js web/src/components/Layout/AppSidebar.vue static/
git commit -m "feat: add audit log viewer page with filters and pagination"
```

---

### Task 10: E2E verification

**Files:** None (verification only)

- [ ] **Step 1: Build backend**

Run: `cd /e/fileManagementIntranet && go build ./...`
Expected: success

- [ ] **Step 2: Run all service tests**

Run: `cd /e/fileManagementIntranet && go test ./internal/service/ -v -count=1`
Expected: all tests pass

- [ ] **Step 3: Build frontend**

Run: `export PATH="/c/Program Files/nodejs:$PATH" && cd /e/fileManagementIntranet/web && npm run build`
Expected: build success

- [ ] **Step 4: Final commit (if any pending changes)**

```bash
git add -A
git status
# Only commit if there are uncommitted changes
```
