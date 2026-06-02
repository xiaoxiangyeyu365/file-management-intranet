# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CloudBox is an internal network cloud storage application for campus network environments. Go/Gin backend + Vue 3/Element Plus frontend, supporting WebDAV, audit logging, quota management, file sharing, and clipboard.

## Build & Run

```bash
# Build and run (port from configs/config.yaml, default 80)
make run                    # go run ./cmd/server

# Full build (frontend + backend)
make build                  # builds web/ then go build

# Frontend only
cd web && npm run build

# Backend only
go build -o cloudbox.exe ./cmd/server

# Run tests
go test ./internal/service/ -v
go test ./internal/handler/ -v
```

## Architecture

**Layers:** Gin routes → handler → service → repository → database (MySQL by default)

**File Storage Model:**
- `physical_files` table: actual file data keyed by MD5, with `ref_count` for deduplication
- `files` table: virtual file tree with `owner_id` isolation, `parent_id` hierarchy, `physical_ref` (NOT `physical_id` — this is the GORM column name)
- Soft delete via `deleted_at`; reference counting is on `physical_files.ref_count`

**Upload Flow:** MD5 calculated on frontend → `POST /api/upload/init` (instant upload check) → chunked upload → `POST /api/upload/:id/complete` (merge chunks, create records, thumbnail)

**Key Go Dependencies:** Gin, GORM (MySQL+SQLite), `golang.org/x/net/webdav`, JWT, Swaggo

**Service Interfaces:** Defined in `internal/service/interfaces.go` — `FileRepository`, `PhysicalFileRepository`, `UserRepository`, `Storage`, `AuditRecorder`, etc. Mock structs are in `internal/service/mock_test.go` with function-pointer fields (nil panics with method name).

## WebDAV (`/dav/`)

Registered via explicit `r.Handle()` for each method (PROPFIND, MKCOL, MOVE, COPY, LOCK, UNLOCK, PROPPATCH) — `r.Any()` only covers standard HTTP methods.

**Critical gotchas:**
- `webdav.Handler` needs `Prefix: "/dav"` to strip the URL prefix before calling FileSystem methods
- CORS middleware skips `/dav` paths so the handler can respond with WebDAV headers (`Ms-Author-Via: DAV`, `Dav: 1, 2`)
- Windows WebClient sends unauthenticated OPTIONS to root path `/` for service discovery — CORS response must include `Ms-Author-Via: DAV`
- BasicAuth middleware skips OPTIONS (Windows may not retry with credentials)
- Route `/dav` (no trailing slash) must be registered separately alongside `/dav/*path`
- DeadPropsHolder must actually store PROPPATCH-ed properties and return them in subsequent PROPFIND; returning ErrNotImplemented causes Windows to DELETE after upload
- `permissiveLockSystem` implemented because Windows sends LOCK but doesn't include lock tokens in subsequent PUT requests
- Column in `files` table is `physical_ref`, not `physical_id` — raw SQL JOINs must use the correct name

## Database

MySQL by default (configs/config.yaml). Tables: `users`, `files`, `physical_files`, `clipboard_records`, `file_shares`, `audit_logs`.

Schema managed in `internal/repository/db.go` — each table has a `create*Table()` function with dual SQLite/MySQL DDL. Auto-migration checks if table exists before creating. Column additions (like `disk_quota`) use raw ALTER TABLE with error suppression for "duplicate column".

**Config:** `configs/config.yaml` (MySQL default) or `configs/config.mysql.yaml`

## Audit Logging

Async: `AuditService` with buffered channel (cap=256), batch size 50, 500ms flush. `Record()` extracts userID/username/clientIP from context. Graceful shutdown via `Shutdown()` — HTTP server stops first, then audit flush.

## Git Conventions

- Use `git add` with specific files, not `git add -A`
- Commit messages in Chinese or English, prefixed with `feat:`, `fix:`, `test:`, `docs:`
- Co-authored-by line on commits
