# WebDAV Protocol Integration Design

## Goal

Allow Windows/Mac/Linux native file managers to mount CloudBox as a network drive via WebDAV, replacing SMB shares on the campus intranet.

## Scope

- Full read-write WebDAV: PROPFIND, GET, PUT, MKCOL, DELETE, MOVE, COPY
- Path prefix `/dav/`, independent from REST API (`/api/`)
- HTTP Basic Auth against existing user accounts
- In-memory locks (MemLS) for Windows client compatibility

## Architecture

```
WebDAV Client
     |
     v
  Gin Router  /dav/*path
     |
     v
  BasicAuthMiddleware  (username/password → AuthService.Login)
     |
     v
  webdav.Handler  (protocol: XML, locks, method routing)
     |
     v
  cloudFS  (implements webdav.FileSystem)
     |         |
     v         v
  FileService  UploadService
     |              |
     v              v
  FileRepository  PhysicalFileRepository  Storage
```

`webdav.Handler` handles the WebDAV protocol (XML parsing/generation, lock management, method dispatch). `cloudFS` adapts the virtual file tree to the `webdav.FileSystem` interface by calling existing Service methods.

## Authentication

**BasicAuthMiddleware** (`internal/handler/webdav_auth.go`):

1. Extract `Authorization: Basic <encoded>` header
2. Decode to `username:password`
3. Call `authService.ValidateCredentials(ctx, username, password)` — returns `*model.User` without generating JWT or audit log (unlike `Login`)
4. On success: inject `userID`, `username`, `clientIP` into `context.Context` (same pattern as JWTMiddleware)
5. On failure: return 401 + `WWW-Authenticate: Basic realm="CloudBox WebDAV"`
6. Account pending/disabled → reject (ValidateCredentials returns same sentinel errors)
7. Failed auth → `user.login_failed` audit record (explicit in middleware, not in ValidateCredentials)

## Path Resolution

User root = `/dav/`, maps to `parent_id IS NULL AND owner_id = userID`.

`resolvePath(ctx, userID, path) → *model.File`:

1. Strip `/dav/` prefix, split by `/`
2. Walk from root (parentID=0) using `FindByName(ctx, userID, parentID, segment)` at each level
3. Empty path → return virtual root (represents user root directory)
4. Any intermediate segment not found → return `os.ErrNotExist`

## webdav.FileSystem Implementation

### `cloudFS` struct

```go
type cloudFS struct {
    fileService   *service.FileService
    uploadService *service.UploadService
    audit         service.AuditRecorder
    storage       storage.Storage
}
```

### Mkdir(ctx, name, perm) → error

- Resolve parent directory + folder name
- Call `fileService.CreateFolder(ctx, userID, parentID, name)`
- Parent not found → `os.ErrNotExist`
- Name conflict → `os.ErrExist`

### Stat(ctx, name) → FileInfo

- `resolvePath` to get `*model.File`
- Not found → `os.ErrNotExist`
- Return `cloudFileInfo` wrapping `model.File` (implements `os.FileInfo`)

### OpenFile(ctx, name, flag, perm) → File

**Read (O_RDONLY) + directory:**

- `resolvePath` → directory node
- Return `cloudDir` with `Readdir` calling `fileService.ListFiles`

**Read (O_RDONLY) + file:**

- `resolvePath` → file node, load PhysicalFile
- `os.Open(storage.ToAbsPath(pf.StoragePath))`
- Return `cloudFile` wrapping `*os.File` (supports Read/Seek/Close)

**Write (O_WRONLY|O_CREATE) — PUT:**

1. Resolve parent directory + file name
2. Parent not found → `os.ErrNotExist`
3. Check if file already exists (`FindByName`)
   - Exists → `fileService.MoveToTrash` (soft delete old version)
4. Call `uploadService.UploadFile(ctx, userID, parentID, name, size, reader)`
5. Return file handle

### RemoveAll(ctx, name) → error

- `resolvePath` → file/folder
- Call `fileService.MoveToTrash(ctx, userID, fileID)` (soft delete)
- Same behavior as Web UI — user can recover from trash
- Not found → `os.ErrNotExist`

### Rename(ctx, oldName, newName) → error

1. Resolve `oldName` → source file
2. Parse `newName` into target parent path + target name
3. Stat target:
   - Target exists and is a folder → move source **into** that folder, preserving source filename
     - `newName = filepath.Join(newName, filepath.Base(oldName))`
   - Target exists and is a file → soft delete target, then rename/move source
   - Target does not exist → proceed
4. Same parent directory, different name → `fileService.Rename`
5. Different parent → `fileService.MoveFiles`
6. Target parent not found → `os.ErrExist` (triggers 409 Conflict per WebDAV spec)

## UploadService.UploadFile

New method for single-request uploads (used by WebDAV PUT and potentially future API):

```go
func (s *UploadService) UploadFile(ctx context.Context, userID, parentID int64,
    name string, size int64, reader io.Reader) (*model.File, error)
```

**Flow:**

1. Write reader to temp file, computing MD5 and accumulating actual size
2. If `size == 0` (unknown Content-Length), use actual size from step 1
3. **Quota check** — `checkQuota(ctx, userID, actualSize)`
   - Over limit → delete temp file, return `os.ErrPermission`
4. **Instant upload** — `physicalRepo.FindByMD5(ctx, md5)`
   - Hit → `IncrementRefCount`, reuse existing PhysicalFile
   - Miss → create PhysicalFile, move temp file to permanent storage
5. Create File record (ownerID, parentID, name, contentRef=physicalID)
6. Thumbnail generation — `previewService.ProcessImage`
7. Audit — `audit.Record(ctx, "file.upload", "file", ...)`
8. Return `*model.File`

## COPY Operation

- File: read source PhysicalFile, call `UploadFile` to create new File record (same PhysicalFile → ref_count+1)
- Folder: `CreateFolder` + recursively copy children (file → UploadFile, subfolder → CreateFolder + recurse)
- COPY target exists and is a folder → copy source **into** that folder (same rule as MOVE)
- COPY target exists and is a file → soft delete target, then copy
- Audit: `file.copy` / `folder.copy` recorded in cloudFS (not in Service layer)

## Locks

- `webdav.NewMemLS()` — in-memory lock store, built into webdav.Handler
- Satisfies Windows clients that issue LOCK before editing
- Locks lost on server restart — clients auto-retry, acceptable for single-instance intranet
- No persistent lock storage needed

## Error Mapping

Service errors map to `os` errors; `webdav.Handler` converts these to HTTP status codes automatically:

| Condition | os Error | HTTP Status |
|---|---|---|
| File/folder not found | `os.ErrNotExist` | 404 |
| Permission denied | `os.ErrPermission` | 403 |
| Quota exceeded | `os.ErrPermission` | 403 (acceptable for intranet) |
| Name conflict | `os.ErrExist` | 409 |
| Target parent not found | `os.ErrExist` | 409 |
| Circular move | `os.ErrPermission` | 403 |

## Audit Log Coverage

| WebDAV Method | Action | Where Recorded |
|---|---|---|
| GET | `file.download` | FileService.DownloadFile |
| PUT | `file.upload` | UploadService.UploadFile |
| MKCOL | `folder.create` | FileService.CreateFolder |
| DELETE | `file.delete` | FileService.MoveToTrash |
| MOVE | `file.move` | FileService.MoveFiles/Rename |
| COPY | `file.copy` / `folder.copy` | cloudFS (explicit) |
| Auth failure | `user.login_failed` | BasicAuthMiddleware |

## Files to Create/Modify

- **Create:** `internal/handler/webdav.go` — cloudFS, cloudFileInfo, cloudFile, cloudDir types
- **Create:** `internal/handler/webdav_auth.go` — BasicAuthMiddleware
- **Modify:** `internal/service/auth.go` — add ValidateCredentials method (credential check only, no JWT, no audit)
- **Modify:** `internal/service/upload.go` — add UploadFile method
- **Modify:** `internal/service/interfaces.go` — no changes needed (UploadService is concrete)
- **Modify:** `cmd/server/main.go` — register `/dav/*path` route, wire cloudFS + webdav.Handler
- **Modify:** `internal/handler/middleware.go` — add `GetUserIDFromContext(ctx)` helper for WebDAV path
- **Modify:** `web/src/views/AdminAuditView.vue` — add `file.copy` / `folder.copy` to filter options and label map
