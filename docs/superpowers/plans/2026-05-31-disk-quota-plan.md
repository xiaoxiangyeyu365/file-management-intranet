# Disk Quota Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add per-user storage quotas with global default config, per-user admin override, upload rejection when exceeded, and usage display in sidebar and admin.

**Architecture:** Global quota from `config.yaml`, per-user override via `users.disk_quota` column (NULL = use global, 0 = unlimited, > 0 = custom). Usage calculated by real-time SUM query (including trash). Admin uses batch SQL query for all users.

**Tech Stack:** Go (Gin + GORM), Vue 3 (Element Plus), SQLite/MySQL

---

### Task 1: Add DiskConfig to config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `configs/config.yaml`

- [ ] **Step 1: Add DiskConfig struct and embed in Config**

In `internal/config/config.go`, add after `ShareConfig` (line 81):

```go
type DiskConfig struct {
    DefaultQuota int64 `yaml:"default_quota"`
}
```

Add `Disk DiskConfig \`yaml:"disk"\`` to the `Config` struct (after line 27 `Share ShareConfig`).

- [ ] **Step 2: Add default in Load()**

In the defaults section (after line 131 `Share: ShareConfig{...}`), add:

```go
Disk: DiskConfig{
    DefaultQuota: 10 * 1024 * 1024 * 1024, // 10GB
},
```

- [ ] **Step 3: Append to config.yaml**

```yaml
disk:
  default_quota: 10737418240  # 10GB
```

- [ ] **Step 4: Build**

Run: `go build ./cmd/server`

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go configs/config.yaml
git commit -m "feat: add DiskConfig for storage quotas"
```

---

### Task 2: Add DiskQuota to User model and DB migration

**Files:**
- Modify: `internal/model/user.go`
- Modify: `internal/repository/db.go`

- [ ] **Step 1: Add DiskQuota field to User model**

In `internal/model/user.go`, add to the `User` struct after `CreatedAt` (line 18):

```go
DiskQuota *int64 `gorm:"column:disk_quota" json:"diskQuota"`
```

- [ ] **Step 2: Add ALTER TABLE migration in db.go**

In `internal/repository/db.go`, after the `update password_changed` exec (line 52), add:

```go
// Add disk_quota column if not exists
DB.Exec("ALTER TABLE users ADD COLUMN disk_quota BIGINT NULL")
```

Note: SQLite ignores `ADD COLUMN IF NOT EXISTS` — it will error if column exists. The existing `AutoMigrate(&model.User{})` at line 43 will add new columns via GORM's own migration, so `DiskQuota` will be added by AutoMigrate. We keep the raw ALTER TABLE as a safety net for cases where AutoMigrate was run before the field was added. But if SQLite throws an error on duplicate column, it won't be fatal since the column already exists.

To be safe, wrap it:

```go
// Add disk_quota column (safe to ignore "duplicate column" errors)
if err := DB.Exec("ALTER TABLE users ADD COLUMN disk_quota BIGINT NULL").Error; err != nil {
    // SQLite may error on duplicate column — that's fine, column already exists
    log.Printf("disk_quota column migration: %v (may already exist)", err)
}
```

- [ ] **Step 3: Build and verify**

Run: `go build ./cmd/server`

- [ ] **Step 4: Commit**

```bash
git add internal/model/user.go internal/repository/db.go
git commit -m "feat: add DiskQuota field to User model with DB migration"
```

---

### Task 3: Add GetQuota/SetQuota to UserRepository and interface

**Files:**
- Modify: `internal/repository/user.go`
- Modify: `internal/service/interfaces.go`

- [ ] **Step 1: Add GetQuota method to UserRepository**

In `internal/repository/user.go`, append:

```go
func (r *UserRepository) GetQuota(ctx context.Context, userID int64) (*int64, error) {
    var user model.User
    if err := r.db.WithContext(ctx).Select("disk_quota").First(&user, userID).Error; err != nil {
        return nil, err
    }
    return user.DiskQuota, nil
}

func (r *UserRepository) SetQuota(ctx context.Context, userID int64, quota *int64) error {
    return r.db.WithContext(ctx).Model(&model.User{}).
        Where("id = ?", userID).
        Update("disk_quota", quota).Error
}
```

- [ ] **Step 2: Extend UserRepository interface**

In `internal/service/interfaces.go`, add to the `UserRepository` interface (after line 41):

```go
GetQuota(ctx context.Context, userID int64) (*int64, error)
SetQuota(ctx context.Context, userID int64, quota *int64) error
```

- [ ] **Step 3: Build**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/repository/user.go internal/service/interfaces.go
git commit -m "feat: add GetQuota/SetQuota to UserRepository"
```

---

### Task 4: Add CalculateUserStorageUsage to PhysicalFileRepository

**Files:**
- Modify: `internal/repository/physical_file.go`
- Modify: `internal/service/interfaces.go`

- [ ] **Step 1: Add CalculateUserStorageUsage method**

In `internal/repository/physical_file.go`, append:

```go
func (r *PhysicalFileRepository) CalculateUserStorageUsage(ctx context.Context, userID int64) (int64, error) {
    var total int64
    err := r.db.WithContext(ctx).Table("files").
        Select("COALESCE(SUM(physical_files.size), 0)").
        Joins("JOIN physical_files ON files.physical_id = physical_files.id").
        Where("files.owner_id = ? AND files.is_folder = 0", userID).
        Scan(&total).Error
    return total, err
}

func (r *PhysicalFileRepository) CalculateAllUserStorageUsage(ctx context.Context) (map[int64]int64, error) {
    type usageRow struct {
        OwnerID   int64
        UsedBytes int64
    }
    var rows []usageRow
    err := r.db.WithContext(ctx).Table("files").
        Select("files.owner_id, COALESCE(SUM(physical_files.size), 0) AS used_bytes").
        Joins("JOIN physical_files ON files.physical_id = physical_files.id").
        Where("files.is_folder = 0").
        Group("files.owner_id").
        Scan(&rows).Error
    if err != nil {
        return nil, err
    }
    result := make(map[int64]int64, len(rows))
    for _, row := range rows {
        result[row.OwnerID] = row.UsedBytes
    }
    return result, nil
}
```

- [ ] **Step 2: Extend PhysicalFileRepository interface**

In `internal/service/interfaces.go`, add to the `PhysicalFileRepository` interface (after line 34):

```go
CalculateUserStorageUsage(ctx context.Context, userID int64) (int64, error)
CalculateAllUserStorageUsage(ctx context.Context) (map[int64]int64, error)
```

- [ ] **Step 3: Build**

Run: `go build ./...`

- [ ] **Step 4: Commit**

```bash
git add internal/repository/physical_file.go internal/service/interfaces.go
git commit -m "feat: add CalculateUserStorageUsage and batch query"
```

---

### Task 5: Add userRepo and checkQuota to UploadService

**Files:**
- Modify: `internal/service/upload.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add userRepo field and ErrQuotaExceeded**

In `internal/service/upload.go`, add the sentinel error (after line 27):

```go
ErrQuotaExceeded = errors.New("storage quota exceeded")
```

Add `userRepo UserRepository` field to the `UploadService` struct (line 45-51):

```go
type UploadService struct {
    fileRepo       FileRepository
    physicalRepo   PhysicalFileRepository
    userRepo       UserRepository
    storage        Storage
    chunkSize      int64
    previewService ImageProcessor
}
```

- [ ] **Step 2: Update NewUploadService constructor**

Change the constructor to accept `userRepo UserRepository`:

```go
func NewUploadService(
    fileRepo FileRepository,
    physicalRepo PhysicalFileRepository,
    userRepo UserRepository,
    storage Storage,
    previewService ImageProcessor,
    chunkSize int64,
) *UploadService {
    return &UploadService{
        fileRepo:       fileRepo,
        physicalRepo:   physicalRepo,
        userRepo:       userRepo,
        storage:        storage,
        chunkSize:      chunkSize,
        previewService: previewService,
    }
}
```

- [ ] **Step 3: Add checkQuota method**

Add the `checkQuota` method to `UploadService`:

```go
func (s *UploadService) checkQuota(ctx context.Context, userID, fileSize int64) error {
    cfg := config.Get()
    usage, err := s.physicalRepo.CalculateUserStorageUsage(ctx, userID)
    if err != nil {
        return err
    }
    quota := cfg.Disk.DefaultQuota
    userQuota, err := s.userRepo.GetQuota(ctx, userID)
    if err == nil && userQuota != nil {
        quota = *userQuota
    }
    if quota == 0 {
        return nil
    }
    if usage+fileSize > quota {
        return ErrQuotaExceeded
    }
    return nil
}
```

Make sure `"cloudbox/internal/config"` is in the imports.

- [ ] **Step 4: Call checkQuota in InitUpload (before instant upload check)**

In `InitUpload` (line 84), after the filename validation (line 93) and before the instant upload check (line 96), add:

```go
// Check storage quota
if err := s.checkQuota(ctx, userID, req.FileSize); err != nil {
    return nil, err
}
```

- [ ] **Step 5: Call checkQuota in CompleteUpload (before file record creation)**

In `CompleteUpload`, find the line where the physical file record is created (search for `physicalRepo.Create`). Before that line, add:

```go
// Check storage quota
if err := s.checkQuota(ctx, userID, req.FileSize); err != nil {
    return nil, err
}
```

- [ ] **Step 6: Update main.go wiring**

In `cmd/server/main.go`, update the `NewUploadService` call (line 45) to pass `userRepo`:

```go
uploadService := service.NewUploadService(fileRepo, physicalRepo, userRepo, storageManager, previewService, cfg.Upload.ChunkSize)
```

- [ ] **Step 7: Update upload handler error mapping**

In `internal/handler/upload.go`, in the `InitUpload` handler's error handling, add a check for `ErrQuotaExceeded`:

```go
if err == service.ErrQuotaExceeded {
    response.Error(c, 413, "存储空间不足，请清理文件或扩容")
    return
}
```

Do the same in the `CompleteUpload` handler.

- [ ] **Step 8: Build**

Run: `go build ./...`

- [ ] **Step 9: Commit**

```bash
git add internal/service/upload.go cmd/server/main.go internal/handler/upload.go
git commit -m "feat: add storage quota check to upload flow"
```

---

### Task 6: Add StorageHandler for usage endpoint

**Files:**
- Create: `internal/handler/storage.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Create StorageHandler**

Create `internal/handler/storage.go`:

```go
package handler

import (
    "cloudbox/internal/config"
    "cloudbox/internal/repository"
    "cloudbox/internal/util/response"
    "net/http"

    "github.com/gin-gonic/gin"
)

type StorageHandler struct {
    physicalRepo *repository.PhysicalFileRepository
    userRepo     *repository.UserRepository
}

func NewStorageHandler(physicalRepo *repository.PhysicalFileRepository, userRepo *repository.UserRepository) *StorageHandler {
    return &StorageHandler{physicalRepo: physicalRepo, userRepo: userRepo}
}

func (h *StorageHandler) GetUsage(c *gin.Context) {
    userID := GetUserID(c)

    used, err := h.physicalRepo.CalculateUserStorageUsage(c.Request.Context(), userID)
    if err != nil {
        response.InternalError(c, "failed to calculate storage usage")
        return
    }

    cfg := config.Get()
    effectiveQuota := cfg.Disk.DefaultQuota

    userQuota, err := h.userRepo.GetQuota(c.Request.Context(), userID)
    if err == nil && userQuota != nil {
        effectiveQuota = *userQuota
    }

    var quotaPtr *int64
    if userQuota != nil {
        quotaPtr = userQuota
    }

    c.JSON(http.StatusOK, gin.H{
        "code": 0,
        "data": gin.H{
            "usedBytes":     used,
            "quotaBytes":    quotaPtr,
            "effectiveQuota": effectiveQuota,
        },
    })
}
```

- [ ] **Step 2: Wire in main.go**

In `cmd/server/main.go`, after the `shareHandler` creation (line 60), add:

```go
storageHandler := handler.NewStorageHandler(physicalRepo, userRepo)
```

Add the storage route inside the `protected` group (after the share routes):

```go
// Storage
protected.GET("/storage/usage", storageHandler.GetUsage)
```

- [ ] **Step 3: Build**

Run: `go build ./cmd/server`

- [ ] **Step 4: Commit**

```bash
git add internal/handler/storage.go cmd/server/main.go
git commit -m "feat: add GET /api/storage/usage endpoint"
```

---

### Task 7: Add UpdateUserQuota to admin and extend ListUsers

**Files:**
- Modify: `internal/handler/admin.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add UpdateUserQuota handler**

In `internal/handler/admin.go`, add:

```go
func (h *AdminHandler) UpdateUserQuota(c *gin.Context) {
    var req struct {
        DiskQuota *int64 `json:"disk_quota"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        response.BadRequest(c, "invalid request body")
        return
    }

    id, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        response.BadRequest(c, "invalid user id")
        return
    }

    if err := h.adminService.SetUserQuota(c.Request.Context(), id, req.DiskQuota); err != nil {
        response.InternalError(c, err.Error())
        return
    }

    response.Success(c, nil)
}
```

Make sure `strconv` is imported.

- [ ] **Step 2: Add SetUserQuota to AdminService**

In `internal/service/admin.go`, add:

```go
func (s *AdminService) SetUserQuota(ctx context.Context, userID int64, quota *int64) error {
    return s.userRepo.SetQuota(ctx, userID, quota)
}
```

Note: `AdminService` uses the concrete `*repository.UserRepository`, so `SetQuota` is available directly.

- [ ] **Step 3: Extend ListUsers to include usedBytes**

In `internal/service/admin.go`, in the `ListUsers` method, after fetching users, call a batch usage query and attach `UsedBytes` to each user. Since the `AdminUserListResult` returns `[]model.User`, and we can't add fields to the model without affecting serialization, we need to return a response struct instead.

Add a response struct in `internal/handler/admin.go`:

```go
type adminUserResponse struct {
    model.User
    UsedBytes int64 `json:"usedBytes"`
}
```

Modify the `ListUsers` handler to query batch usage and build the response:

```go
func (h *AdminHandler) ListUsers(c *gin.Context) {
    status := c.Query("status")

    result, err := h.adminService.ListUsers(c.Request.Context(), status)
    if err != nil {
        response.InternalError(c, "failed to list users")
        return
    }

    // Batch fetch storage usage for all users
    usageMap, err := h.adminService.GetAllUserStorageUsage(c.Request.Context())
    if err != nil {
        usageMap = make(map[int64]int64)
    }

    users := make([]adminUserResponse, 0, len(result.Users))
    for _, u := range result.Users {
        users = append(users, adminUserResponse{
            User:      u,
            UsedBytes: usageMap[u.ID],
        })
    }

    c.JSON(http.StatusOK, gin.H{
        "code": 0,
        "data": gin.H{
            "users":        users,
            "total":        result.Total,
            "pendingCount": result.PendingCount,
        },
    })
}
```

- [ ] **Step 4: Add GetAllUserStorageUsage to AdminService**

In `internal/service/admin.go`:

```go
func (s *AdminService) GetAllUserStorageUsage(ctx context.Context) (map[int64]int64, error) {
    return s.physicalRepo.CalculateAllUserStorageUsage(ctx)
}
```

Note: `AdminService` uses the concrete `*repository.PhysicalFileRepository`, so this is available.

- [ ] **Step 5: Wire UpdateUserQuota route in main.go**

In `cmd/server/main.go`, add inside the admin route group:

```go
admin.PUT("/users/:id/quota", adminHandler.UpdateUserQuota)
```

- [ ] **Step 6: Build**

Run: `go build ./cmd/server`

- [ ] **Step 7: Commit**

```bash
git add internal/handler/admin.go internal/service/admin.go cmd/server/main.go
git commit -m "feat: add admin quota management and per-user usage display"
```

---

### Task 8: Run tests and verify backend

- [ ] **Step 1: Run all tests**

Run: `go test ./... -v -timeout 30s`
Expected: all tests pass (95 existing + no new tests for this task since it's config/model/repo only).

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`

- [ ] **Step 3: Fix any compilation or vet issues**

- [ ] **Step 4: Commit if needed**

---

### Task 9: Add storageAPI to frontend and sidebar usage bar

**Files:**
- Modify: `web/src/utils/api.js`
- Modify: `web/src/components/Layout/AppSidebar.vue`

- [ ] **Step 1: Add storageAPI to api.js**

In `web/src/utils/api.js`, after the `shareAPI` export, add:

```js
export const storageAPI = {
  getUsage: () => axios.get('/storage/usage'),
}
```

- [ ] **Step 2: Add storage usage bar to AppSidebar**

In `web/src/components/Layout/AppSidebar.vue`, add a `<script setup>` section and usage data:

Add imports and state:

```js
import { ref, onMounted } from 'vue'
import { storageAPI } from '@/utils/api'

const usedBytes = ref(0)
const effectiveQuota = ref(0)

onMounted(async () => {
  try {
    const res = await storageAPI.getUsage()
    usedBytes.value = res.data.usedBytes
    effectiveQuota.value = res.data.effectiveQuota
  } catch (e) { /* ignore */ }
})

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function usagePercent() {
  if (!effectiveQuota.value) return 0
  return Math.min(100, Math.round((usedBytes.value / effectiveQuota.value) * 100))
}

function usageStatus() {
  const pct = usagePercent()
  if (pct > 95) return 'danger'
  if (pct > 80) return 'warning'
  return ''
}
```

Add the usage section in the template, below the `<nav>` and before `</aside>`:

```html
<div v-if="usedBytes > 0" class="storage-usage">
  <div class="storage-text">
    <span>{{ formatBytes(usedBytes) }} 已使用</span>
    <span v-if="effectiveQuota"> / {{ formatBytes(effectiveQuota) }}</span>
  </div>
  <el-progress v-if="effectiveQuota" :percentage="usagePercent()" :status="usageStatus()" :stroke-width="6" />
  <div v-else class="storage-text">{{ formatBytes(usedBytes) }}</div>
</div>
```

Add scoped styles:

```scss
.storage-usage {
  padding: 16px 12px;
  border-top: 1px solid #e4e7ed;
  margin-top: auto;
}

.storage-text {
  font-size: 12px;
  color: #909399;
  margin-bottom: 8px;
}
```

- [ ] **Step 3: Build frontend**

Run: `cd web && npm run build`
Expected: builds successfully.

- [ ] **Step 4: Commit**

```bash
git add web/src/utils/api.js web/src/components/Layout/AppSidebar.vue static/index.html
git commit -m "feat: add storage usage bar to sidebar and storageAPI"
```

---

### Task 10: Extend AdminUsersView with storage column

**Files:**
- Modify: `web/src/views/AdminUsersView.vue`

- [ ] **Step 1: Add storage column to the table**

In the `el-table` in `AdminUsersView.vue`, add a new column after the "密码状态" column:

```html
<el-table-column label="存储" width="160">
  <template #default="{ row }">
    <span>{{ formatBytes(row.usedBytes) }}</span>
    <span v-if="row.diskQuota"> / {{ formatBytes(row.diskQuota) }}</span>
    <span v-else-if="row.effectiveQuota"> / {{ formatBytes(row.effectiveQuota) }} (全局)</span>
  </template>
</el-table-column>
```

- [ ] **Step 2: Add formatBytes helper**

Add the `formatBytes` function to the `<script setup>`:

```js
function formatBytes(bytes) {
  if (!bytes || bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}
```

- [ ] **Step 3: Add quota edit via el-popover**

In the operations column of `AdminUsersView.vue`, add a quota edit button:

```html
<el-popover trigger="click" width="280">
  <template #reference>
    <el-button size="small" link>配额</el-button>
  </template>
  <div>
    <el-radio-group v-model="quotaMode" style="display: flex; flex-direction: column; gap: 8px;">
      <el-radio value="global">使用全局默认</el-radio>
      <el-radio value="unlimited">无限制</el-radio>
      <el-radio value="custom">自定义</el-radio>
    </el-radio-group>
    <el-input
      v-if="quotaMode === 'custom'"
      v-model="customQuotaGB"
      type="number"
      min="0.1"
      step="0.1"
      placeholder="GB"
      style="margin-top: 8px;"
    >
      <template #append>GB</template>
    </el-input>
    <el-button type="primary" size="small" style="margin-top: 8px; width: 100%;" @click="handleQuotaChange(row)">
      保存
    </el-button>
  </div>
</el-popover>
```

Add the state and handler:

```js
const quotaMode = ref('global')
const customQuotaGB = ref(10)

async function handleQuotaChange(user) {
  let quota = null
  if (quotaMode.value === 'unlimited') quota = 0
  else if (quotaMode.value === 'custom') quota = Math.round(customQuotaGB.value * 1024 * 1024 * 1024)

  try {
    await axios.put(`/admin/users/${user.id}/quota`, { disk_quota: quota })
    ElMessage.success('配额已更新')
    fetchUsers()
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '更新失败')
  }
}
```

Import `axios` and `ElMessage` if not already imported.

- [ ] **Step 4: Build frontend**

Run: `cd web && npm run build`

- [ ] **Step 5: Commit**

```bash
git add web/src/views/AdminUsersView.vue static/index.html
git commit -m "feat: add storage column and quota edit to admin user table"
```

---

### Task 11: End-to-end verification

- [ ] **Step 1: Build backend**

Run: `go build ./cmd/server`

- [ ] **Step 2: Run all tests**

Run: `go test ./... -timeout 30s`

- [ ] **Step 3: Start server and smoke test**

1. Start the server
2. Login as admin
3. Upload a file — verify upload succeeds
4. Check sidebar — verify storage usage bar shows correct values
5. Navigate to 用户管理 — verify "存储" column shows usage
6. Click "配额" button — set to 1 GB, verify it saves
7. Try uploading a file larger than 1 GB — verify upload is rejected with "存储空间不足"
8. Delete the file to trash — verify usage does NOT decrease (trash still counted)
9. Permanently delete the file — verify usage decreases
10. Revert quota to "使用全局默认" — verify it saves correctly

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "feat: complete disk quota feature"
```
