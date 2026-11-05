# Disk Quota Design

## Overview

Add per-user storage quotas to CloudBox. Global default from config, per-user override by admin. When storage usage exceeds quota, uploads are rejected. Usage includes trash (soft-deleted files still occupy disk space).

## Data Model

### User model extension

Add `DiskQuota *int64` to `model.User`:

```go
DiskQuota *int64 `gorm:"column:disk_quota" json:"diskQuota"`
```

| Value | Meaning |
|-------|---------|
| `NULL` | Use global default (`config.Disk.DefaultQuota`) |
| `0` | Unlimited |
| `> 0` | Per-user quota in bytes |

### Config extension

Add to `config.yaml`:

```yaml
disk:
  default_quota: 10737418240  # 10GB, in bytes. 0 = unlimited
```

Add `DiskConfig` struct:

```go
type DiskConfig struct {
    DefaultQuota int64 `yaml:"default_quota"`
}
```

### DB migration

ALTER TABLE users ADD COLUMN `disk_quota BIGINT NULL` (SQLite: already nullable, just add column).

## Storage Usage Calculation

**Critical: includes trash files.** Soft-deleted files still occupy physical disk space. Only permanent delete releases the space. The SQL must NOT filter `deleted_at IS NULL`.

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
```

Both quota check and admin display use the **same query** (including trash), ensuring consistent numbers.

## API Design

### User routes (JWT)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/storage/usage` | Current user storage usage + quota info |

Response:

```json
{
  "code": 0,
  "data": {
    "usedBytes": 2147483648,
    "quotaBytes": 10737418240,
    "effectiveQuota": 10737418240
  }
}
```

- `usedBytes`: real-time SUM including trash
- `quotaBytes`: user's personal quota (null = using global)
- `effectiveQuota`: actual effective quota (personal > global; 0 = unlimited)

### Admin routes (JWT + AdminMiddleware)

| Method | Path | Description |
|--------|------|-------------|
| PUT | `/api/admin/users/:id/quota` | Set per-user quota |

Request:

```json
{ "disk_quota": 5368709120 }   // Set to 5GB
{ "disk_quota": 0 }             // Unlimited
```

To restore global default: omit the field or pass `null`. The admin UI should offer two options: "无限制" (sets 0) and "使用全局默认" (sets null).

### GET /api/admin/users extension

Each user object in the response includes additional fields:

```json
{
  "diskQuota": null,
  "usedBytes": 2147483648
}
```

`usedBytes` is calculated using a single batch query (not N+1), then mapped to users in application layer:

```sql
SELECT f.owner_id, COALESCE(SUM(pf.size), 0) AS used_bytes
FROM files f
JOIN physical_files pf ON f.physical_id = pf.id
WHERE f.is_folder = 0
GROUP BY f.owner_id
```

Admin handler fetches all users, runs this single query, and attaches `usedBytes` to each user object before returning.

## Upload Quota Check

In `UploadService`, add a `checkQuota(ctx, userID, fileSize)` method:

```go
func (s *UploadService) checkQuota(ctx context.Context, userID, fileSize int64) error {
    cfg := config.Get()
    usage, err := s.physicalRepo.CalculateUserStorageUsage(ctx, userID)
    if err != nil {
        return err
    }
    // Resolve effective quota: user-level > global default
    quota := cfg.Disk.DefaultQuota
    userQuota, err := s.userRepo.GetQuota(ctx, userID)
    if err == nil && userQuota != nil {
        quota = *userQuota
    }
    if quota == 0 {
        return nil // unlimited
    }
    if usage+fileSize > quota {
        return ErrQuotaExceeded
    }
    return nil
}
```

`ErrQuotaExceeded` maps to HTTP 413 in handlers. Frontend shows "存储空间不足，请清理文件或扩容".

### Call order in InitUpload

In `InitUpload`, `checkQuota` must be called **before** the instant upload MD5 check:

1. Validate request
2. **Check quota** → if exceeded, return ErrQuotaExceeded (413) immediately
3. Check for instant upload (FindByMD5)
4. Proceed with instant upload or generate uploadID for chunked upload

This ensures quota is enforced regardless of whether the upload is instant (dedup) or new.

### Call order in CompleteUpload

In `CompleteUpload`, `checkQuota` is called **after** chunk merging and MD5 verification but **before** creating the file record:

1. Validate uploadID
2. Merge chunks, verify MD5 + size
3. **Check quota** → if exceeded, return ErrQuotaExceeded (413)
4. Create physical file record
5. Create file record

`ErrQuotaExceeded` maps to HTTP 413 in handlers. Frontend shows "存储空间不足，请清理文件或扩容".

### Instant upload check

Instant upload (dedup via MD5) also adds a file record and increments ref count. It should be checked against quota because the user gains a logical file. However, the physical disk usage does not increase. Two options:

- **Check quota for instant upload**: Consistent with "user owns N bytes of files". Recommended.
- **Skip quota check**: Only checks when actual disk is written. Simpler but allows users to exceed quota via dedup chains.

**Decision: check quota for instant upload too.** The quota represents logical ownership, not raw disk.

## Frontend Changes

### User: Storage usage bar in sidebar

In `AppSidebar.vue`, add a storage usage section below the nav items:

- Show `usedBytes / effectiveQuota` with a progress bar
- Format bytes as human-readable (KB/MB/GB)
- Progress bar turns orange (> 80%) or red (> 95%)
- If quota is 0 (unlimited), show usage only: "已使用 2.1 GB"

### Admin: User table usage column

In `AdminUsersView.vue`, add a "存储" column:

- Display `usedBytes / effectiveQuota` format
- If `diskQuota` is null: show "已使用 X / 全局默认"
- If `diskQuota` is 0: show "已使用 X / 无限制"
- Quota edit: el-popover or inline edit with two radio options: "使用全局默认" (null), "无限制" (0), "自定义" (input field in bytes, converted to GB)

## Files

### New files

| File | Description |
|------|-------------|
| `internal/handler/storage.go` | User storage usage endpoint |

### Modified files

| File | Change |
|------|--------|
| `internal/config/config.go` | Add `DiskConfig` struct |
| `internal/config/config.go` | Add `Disk DiskConfig` to `Config` |
| `internal/config/config.go` | Set default `DefaultQuota: 10737418240` |
| `internal/model/user.go` | Add `DiskQuota *int64` field |
| `internal/repository/db.go` | ALTER TABLE users ADD disk_quota |
| `internal/repository/user.go` | Add `GetQuota` and `SetQuota` methods |
| `internal/repository/physical_file.go` | Add `CalculateUserStorageUsage(ctx, userID) (int64, error)` and `CalculateAllUserStorageUsage(ctx) (map[int64]int64, error)` for admin batch query |
| `internal/service/interfaces.go` | Extend `UserRepository` with `GetQuota`/`SetQuota`, extend `PhysicalFileRepository` with `CalculateUserStorageUsage`/`CalculateAllUserStorageUsage` |
| `internal/service/upload.go` | Add `userRepo UserRepository` field to struct, update `NewUploadService` to accept it, add `checkQuota` method, call from `InitUpload` (before instant upload check) and `CompleteUpload` (before file record creation) |
| `internal/handler/admin.go` | Add `UpdateUserQuota` handler, extend `ListUsers` with `usedBytes` |
| `cmd/server/main.go` | Register storage handler and admin quota route |
| `configs/config.yaml` | Add `disk.default_quota` |
| `web/src/utils/api.js` | Add `storageAPI` module |
| `web/src/components/Layout/AppSidebar.vue` | Add storage usage bar |
| `web/src/views/AdminUsersView.vue` | Add "存储" column and quota edit |

## Sentinel Errors

```go
var ErrQuotaExceeded = errors.New("storage quota exceeded")
```

Maps to HTTP 413 in handler layer. Frontend message: "存储空间不足，请清理文件或扩容".
