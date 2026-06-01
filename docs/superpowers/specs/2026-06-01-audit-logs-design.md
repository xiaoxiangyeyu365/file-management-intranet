# Audit Operation Logs — Design Spec

## Overview

Add operation audit logging to CloudBox. Record admin operations and user key operations (login, upload, delete, share, etc.) with service-layer instrumentation, async buffered writes, and an admin-only viewing page.

## Data Model

```sql
CREATE TABLE audit_logs (
  id          BIGINT PRIMARY KEY AUTOINCREMENT,
  user_id     BIGINT NOT NULL,
  username    VARCHAR(50) NOT NULL,
  action      VARCHAR(50) NOT NULL,
  target_type VARCHAR(30) NOT NULL,
  target_id   BIGINT,
  target_name VARCHAR(255),
  detail      TEXT,
  ip_address  VARCHAR(45),
  created_at  DATETIME NOT NULL
);

CREATE INDEX idx_audit_user   ON audit_logs(user_id);
CREATE INDEX idx_audit_action ON audit_logs(action);
CREATE INDEX idx_audit_time   ON audit_logs(created_at);
```

`username` is denormalized to avoid JOINs on every query. `detail` stores JSON for additional context (e.g. `{"deletedFiles":3,"deletedFolders":1}`).

## Action Enum

| Category | Action | Description |
|----------|--------|-------------|
| Auth | `user.login` | Login success |
| Auth | `user.login_failed` | Login failure |
| Auth | `user.change_password` | Password changed |
| File | `file.upload` | File uploaded |
| File | `file.delete` | Moved to trash |
| File | `file.restore` | Restored from trash |
| File | `file.permanent_delete` | Permanently deleted |
| File | `file.rename` | Renamed |
| File | `file.move` | Moved |
| File | `file.download` | Downloaded |
| File | `folder.create` | Folder created |
| Trash | `trash.empty` | Trash emptied |
| Share | `share.create` | Share created |
| Share | `share.revoke` | Share revoked |
| Share | `share.download` | Downloaded via share |
| Clipboard | `clipboard.create` | Clipboard entry created |
| Clipboard | `clipboard.delete` | Clipboard entry deleted |
| Admin | `admin.create_user` | User created |
| Admin | `admin.delete_user` | User deleted |
| Admin | `admin.reset_password` | Password reset |
| Admin | `admin.update_status` | User status changed |
| Admin | `admin.set_quota` | Quota set |

## Architecture

```
Service method → audit.Record(ctx, action, targetType, targetID, targetName, detail)
                      ↓
                buffered channel (cap=256)
                      ↓
                background goroutine: batch INSERT (≤50 or 500ms flush)
```

### AuditRecorder Interface

```go
type AuditRecorder interface {
    Record(ctx context.Context, action, targetType string, targetID int64, targetName, detail string)
}
```

### AuditService Struct

```go
type AuditService struct {
    repo          AuditRepository
    ch            chan *AuditEntry
    wg            sync.WaitGroup
    batchSize     int             // 50
    flushInterval time.Duration   // 500ms
    dropped       atomic.Int64
    closed        atomic.Bool
}
```

Methods:
- `Record` — checks `closed` first; if closed, increments `dropped` and returns. Otherwise pushes to channel. If channel full, increments `dropped` and logs warning.
- `Shutdown` — sets `closed=true`, closes channel, waits on `wg`
- `DroppedCount` — returns `dropped.Load()`

### Context Injection

JWTMiddleware injects user info into request context:

```go
ctx := context.WithValue(c.Request.Context(), "userID", userID)
ctx = context.WithValue(ctx, "username", username)
ctx = context.WithValue(ctx, "clientIP", c.ClientIP())
c.Request = c.Request.WithContext(ctx)
```

`AuditService.Record` extracts these from ctx. No manual user/IP passing needed at call sites.

## Graceful Shutdown Order

On SIGINT/SIGTERM:

1. `srv.Shutdown(ctx)` — stop accepting new HTTP requests (no new audit entries generated)
2. `auditService.Shutdown()` — close channel, flush remaining entries to DB, wait for goroutine exit

This guarantees no entries are lost during shutdown.

## Query API

```
GET /api/admin/audit-logs
```

| Parameter | Type | Description |
|-----------|------|-------------|
| `action` | string | Filter by action |
| `userId` | int64 | Filter by operator |
| `targetType` | string | Filter by target type |
| `startDate` | string | Start time (RFC3339) |
| `endDate` | string | End time (RFC3339) |
| `keyword` | string | Search targetName / detail |
| `page` | int | Page number, default 1 |
| `pageSize` | int | Page size, default 50 |

Response:

```json
{
  "logs": [{ "id": 1, "userId": 1, "username": "admin", "action": "file.upload", ... }],
  "total": 1234,
  "page": 1,
  "pageSize": 50
}
```

Admin-only access (JWTMiddleware + AdminMiddleware).

## Auto-Cleanup

Config:

```yaml
audit:
  retention_days: 90
```

```go
type AuditConfig struct {
    RetentionDays int `yaml:"retention_days"`
}
```

Background goroutine (started in `main.go` alongside `StartTempCleanup`), runs once per day at midnight. Deletes rows where `created_at < NOW() - retention_days`. If `retention_days == 0`, cleanup is disabled.

## Frontend

- Sidebar: add "审计日志" nav item (admin-only)
- Route: `/admin/audit`
- View: `AdminAuditView.vue`
  - Filter bar: action dropdown, username input, date range picker, target type dropdown
  - `el-table`: timestamp, username, action label, target, IP, detail
  - Pagination

## Implementation Checklist

| Component | File | Description |
|-----------|------|-------------|
| Model | `internal/model/audit.go` | AuditLog struct |
| Repository | `internal/repository/audit.go` | BatchCreate, FindWithFilter, DeleteBefore |
| Interface | `internal/service/interfaces.go` | AuditRepository + AuditRecorder |
| Service | `internal/service/audit.go` | AuditService (channel + batch write + graceful shutdown) |
| Instrumentation | Each service file | 22 Record calls across auth/file/upload/share/clipboard/admin |
| Context injection | `internal/handler/middleware.go` | JWTMiddleware injects userID/username/clientIP |
| Handler | `internal/handler/admin.go` | ListAuditLogs endpoint |
| Routes | `cmd/server/main.go` | GET /api/admin/audit-logs + graceful shutdown + cleanup goroutine |
| Frontend view | `web/src/views/AdminAuditView.vue` | Log table + filters + pagination |
| Frontend route | `web/src/router/index.js` | /admin/audit |
| Sidebar | `web/src/components/Layout/AppSidebar.vue` | Add "审计日志" nav |
| Migration | `internal/repository/db.go` | createAuditLogsTable |
| Config | `internal/config/config.go` | AuditConfig |
