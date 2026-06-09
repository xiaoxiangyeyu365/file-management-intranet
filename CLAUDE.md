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

## HTTPS / TLS

Self-signed TLS for WebDAV encryption. Config in `configs/config.yaml` under `tls:`.

- `tls.enabled: true` starts a second `http.Server` on `tls.port` (default 8443) with `ListenAndServeTLS`
- Certificates auto-generated on first start to `data/tls/` (CA key 4096-bit, server key 2048-bit, server cert valid 1 year)
- Auto-renews if cert expires within 30 days
- CA cert (`data/tls/ca.crt`) must be imported on client machines for trusted HTTPS
- HTTP port still active; `/dav` paths on HTTP get 307 redirect to HTTPS
- Private key files have 0600 permissions
- Port validation: `tls.port` must differ from `server.port`
- Import alias `tlspkg` used for `internal/util/tls` to avoid shadowing `crypto/tls`

## Database

MySQL by default (configs/config.yaml). Tables: `users`, `files`, `physical_files`, `clipboard_records`, `file_shares`, `audit_logs`, `file_tags`.

Schema managed in `internal/repository/db.go` — each table has a `create*Table()` function with dual SQLite/MySQL DDL. Auto-migration checks if table exists before creating. Column additions (like `disk_quota`, `summary`, `summary_generated_at`) use raw ALTER TABLE with error suppression for "duplicate column".

## AI Summarization

Self-signed LLM integration for file summaries and tags. Config in `configs/config.yaml` under `ai:`.

- `ai.enabled: true` enables async AI processing after file upload
- Summary stored on `physical_files` (dedup — same content only calls LLM once)
- Tags stored per-File in `file_tags` table (initial copy from PhysicalFile, then independent)
- Auto-processes text/PDF only; image/video require manual trigger via `POST /api/files/:id/ai-summary`
- `vision_model` falls back to `model` if empty
- Uses OpenAI-compatible API format (`/chat/completions`)
- Response parsing expects `摘要：...\n标签：tag1,tag2,...` format
- `ai.api_key` falls back to `AI_API_KEY` env var
- Worker pool limits concurrent LLM calls (`max_concurrent`, default 2)
- AIService.Shutdown() waits for in-progress tasks on graceful shutdown
- Permanent delete cascades to file_tags; soft delete preserves tags

**Config:** `configs/config.yaml` (MySQL default) or `configs/config.mysql.yaml`

## Audit Logging

Async: `AuditService` with buffered channel (cap=256), batch size 50, 500ms flush. `Record()` extracts userID/username/clientIP from context. Graceful shutdown via `Shutdown()` — HTTP server stops first, then audit flush.

## Git Conventions

- Use `git add` with specific files, not `git add -A`
- Commit messages in Chinese or English, prefixed with `feat:`, `fix:`, `test:`, `docs:`
- Co-authored-by line on commits
