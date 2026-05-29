# File Sharing / Public Link Design

## Overview

Add file sharing via public links to CloudBox. Users can generate share links for files and folders, with optional password protection, expiration, and download count limits. Recipients access a preview page before downloading. MVP scope is public link download only; collaboration interfaces are reserved for future extension.

## Data Model

### `file_shares` table

| Column | Type | Description |
|--------|------|-------------|
| `id` | int64 PK | Auto-increment primary key |
| `token` | varchar(8) UNIQUE | Short random token (e.g. `a3Kf9x`) |
| `file_id` | int64 FK → files.id | Shared file |
| `owner_id` | int64 FK → users.id | Share creator |
| `password_hash` | varchar(255) NULL | bcrypt hash of access password, NULL = no password |
| `expires_at` | datetime NULL | Expiration time, NULL = never expires |
| `max_downloads` | int DEFAULT 0 | Max download count, 0 = unlimited |
| `download_count` | int DEFAULT 0 | Current download count |
| `revoked` | bool DEFAULT false | Whether share has been revoked |
| `created_at` | datetime | Creation time |

Indexes: `token` (unique), `file_id`, `owner_id`

Design decisions:
- Token is 8 chars (62^8 ≈ 218 trillion combinations, brute-force infeasible on intranet)
- `password_hash` NULL means no password (not empty string); named consistently with `users.password_hash`
- One file can have multiple share links with different settings
- Revocation uses soft `revoked` flag, not physical delete, preserving audit trail
- Expiration check uses `expires_at <= NOW()` (inclusive boundary) for clarity

## API Design

### Public routes (no JWT, mounted on `r.Group("/api/s")` independent of JWT-protected group)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/s/:token` | Get share info (filename, size, hasPassword, expired/revoked status) |
| POST | `/api/s/:token/verify` | Submit password (or empty for no-password shares), returns HMAC download credential |
| GET | `/api/s/:token/download?t=<credential>` | Download file with HMAC credential |

### Authenticated routes (JWT required)

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/shares` | Create share link |
| GET | `/api/shares?fileId={id}` | List shares for a file |
| GET | `/api/shares/mine` | List current user's all shares |
| DELETE | `/api/shares/:id` | Revoke share |

### Download credential flow

Unified flow regardless of password:
1. `GET /api/s/:token` → preview page shows file info
2. User clicks download → `POST /api/s/:token/verify` (with password if required, empty if not)
3. Backend returns HMAC credential: `token.timestamp.signature`
4. `GET /api/s/:token/download?t=<credential>` → file stream

HMAC signature: `HMAC-SHA256(token + "." + timestamp, shareSecret)`, validity window ±5 minutes (300s) to tolerate clock drift.

### Atomic download count increment

```sql
UPDATE file_shares
SET download_count = download_count + 1
WHERE token = ? AND (max_downloads = 0 OR download_count < max_downloads) AND revoked = false
```
`RowsAffected == 1` = success, `RowsAffected == 0` = limit reached or revoked.

## Service Layer

### ShareRepository interface

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

`IncrementDownloadCount` returns `(ok bool, err error)`: `ok=true` if atomically incremented, `ok=false` if limit reached/revoked.

### ShareService methods

| Method | Logic |
|--------|-------|
| `CreateShare(ctx, userID, fileID, password, expiresAt, maxDownloads)` | Verify file ownership → generate 8-char token (retry on collision) → bcrypt hash password → insert |
| `GetShareInfo(ctx, token)` | Find by token → check expired/revoked → return file name/size/hasPassword (never return password_hash or credentials) |
| `VerifyOrGetCredential(ctx, token, password)` | Find by token → check expired/revoked → if no password: generate HMAC credential; if has password: bcrypt verify then generate credential |
| `DownloadByShare(ctx, token, credential)` | Verify HMAC credential → atomic IncrementDownloadCount → find file → if folder: stream as ZIP via `StreamFolderZipByID` (no auth check); if file: return file + physicalFile |
| `ListMyShares(ctx, userID)` | Find all shares by owner |
| `ListFileShares(ctx, userID, fileID)` | Find all shares for a file (owner-scoped) |
| `RevokeShare(ctx, userID, shareID)` | Verify ownership → set revoked=true |

### Folder download auth decoupling

`StreamFolderZip` requires a `userID` for ownership verification. In the share context, authorization is already proven via the share token, so `ShareService` cannot call `StreamFolderZip` directly. Solution: add a `StreamFolderZipByID(ctx, folderID, writer)` method to `FileService` that streams the folder as ZIP without ownership checks. `ShareService.DownloadByShare` calls this method for folder shares. The existing `StreamFolderZip` handler continues to use the authenticated path.

## Frontend

### New pages

**SharePreview `/s/:token`** (public, no login required):
- Centered card: file name, size, creation time
- Password input if `hasPassword=true`, download button after verify
- Status messages in Chinese: "该分享已过期" / "该分享已被撤销" / "下载次数已达上限"
- Responsive design (max-width, padding), no overflow on mobile
- Layout: no AppHeader/Sidebar (standalone page)

**SharesView `/shares`** (authenticated):
- Table: file name, created time, expires at, download count/limit, status (active/expired/revoked)
- Actions: copy link, revoke
- Status filter tabs reserved (not implemented in MVP)

### Modified components

- **FileGrid / FileList / MobileFileList**: Add "分享" to context menu, emit `share` event
- **FilesView**: Listen for `share` event, open CreateShareDialog
- **AppSidebar / MobileTabBar**: Add "我的分享" navigation entry

### New components

- **CreateShareDialog.vue**: Expiration dropdown (1h/1d/7d/永久), optional password, optional download limit. After creation: show link + copy button. On mobile: fullscreen dialog.
- **SharePreview.vue**: Public share preview page (no auth layout)

### New store

- `shares.js`: Share CRUD state management, refresh after create

### Clipboard fallback

```js
if (!navigator.clipboard) {
  const textArea = document.createElement('textarea');
  textArea.value = shareUrl;
  document.body.appendChild(textArea);
  textArea.select();
  document.execCommand('copy');
  document.body.removeChild(textArea);
} else {
  await navigator.clipboard.writeText(shareUrl);
}
```

### Router

```js
{ path: '/s/:token', component: SharePreview, meta: { public: true } }
{ path: '/shares', component: SharesView, meta: { requiresAuth: true } }
```

Route guard: `if (to.meta.public) return next()` — skip JWT check.

## Configuration

```yaml
share:
  secret: ""          # HMAC signing key, empty = random on startup (with warning)
  token_length: 8     # Share token character length
  credential_ttl: 300 # Download credential TTL in seconds
```

## File Changes

### New files

| File | Description |
|------|-------------|
| `internal/model/share.go` | FileShare model |
| `internal/repository/share.go` | ShareRepository implementation |
| `internal/service/share.go` | ShareService implementation |
| `internal/handler/share.go` | Public + authenticated share handlers |
| `web/src/views/SharePreview.vue` | Public share preview page |
| `web/src/views/SharesView.vue` | Share management page |
| `web/src/components/Dialogs/CreateShareDialog.vue` | Create share dialog |
| `web/src/stores/shares.js` | Share Pinia store |
| `web/src/utils/api.js` | Add shareAPI module |

### Modified files

| File | Change |
|------|--------|
| `internal/repository/db.go` | Add `file_shares` table creation |
| `internal/service/interfaces.go` | Add ShareRepository interface |
| `internal/config/config.go` | Add ShareConfig |
| `cmd/server/main.go` | Add public route group + ShareHandler wiring |
| `web/src/router/index.js` | Add `/s/:token` and `/shares` routes |
| `web/src/components/Files/FileGrid.vue` | Add "分享" to context menu |
| `web/src/components/Files/FileList.vue` | Add "分享" to context menu |
| `web/src/components/Files/MobileFileList.vue` | Add "分享" to long-press menu |
| `web/src/views/FilesView.vue` | Listen for share event, open dialog |
| `web/src/components/Layout/AppSidebar.vue` | Add "我的分享" nav item |
| `web/src/components/Layout/MobileTabBar.vue` | Add "分享" tab |
