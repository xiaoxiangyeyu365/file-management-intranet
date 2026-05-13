# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CloudBox is an internal network cloud storage application for campus network environments. It replaces Windows file sharing (SMB) limitations by providing a web-based file manager with upload/download, folder management, and a CLI tool.

**Current Phase:** Design complete, implementation pending (Phase 1: Go backend)

**Design Spec:** `docs/superpowers/specs/2025-05-13-cloudbox-design.md`

## Architecture

### Three-Phase Implementation

1. **Phase 1: Go Backend** - REST API server with SQLite database
2. **Phase 2: Vue Frontend** - Web UI served by embedded static files
3. **Phase 3: CLI Tool** - Command-line interface for scripting

### Key Design Decisions

**File Storage Model:**
- `physical_files` table: Stores actual file data, referenced by MD5 for deduplication (instant upload)
- `files` table: User-facing records with owner_id for multi-user isolation, parent_id for directory tree
- Reference counting (`ref_count`) is on `physical_files`, not on `files` records

**Upload Flow:**
- `uploadID = MD5(fileContent) + "_" + userID` for temp directory naming
- Chunk size: 5MB, concurrent uploads: max 3
- Frontend calculates MD5 via Web Worker to enable instant upload check

**Security:**
- JWT authentication on all API routes except login
- User isolation via owner_id filtering on every file operation
- Sensitive operations (delete, download) require frontend confirmation dialog

**Database:**
- SQLite single-file database at `./data/cloudbox.db`
- Repository pattern with interface abstraction for future MySQL migration

## Project Structure (Target)

```
cloudbox/
├── cmd/server/main.go          # Backend entry point
├── internal/
│   ├── config/                 # Configuration loading
│   ├── handler/                # HTTP handlers (auth, files, folders, upload, trash)
│   ├── service/                # Business logic layer
│   ├── repository/             # Data access layer
│   ├── model/                  # Data models
│   └── util/                   # Crypto, storage, response utilities
├── web/                        # Vue frontend source
├── configs/config.yaml         # Configuration template
├── embed.go                    # Static file embedding (//go:embed web/dist)
└── Makefile                    # Build commands
```

## Build Commands (Planned)

```bash
# Full build (frontend + backend)
make build

# Backend only
make build-server

# Frontend only
make build-frontend

# Run
./cloudbox.exe
```

## Database Schema

**users**: id, username, password_hash, role, created_at

**physical_files**: id, storage_path, md5 (UNIQUE), size, mime_type, ref_count, thumbnail_path, created_at

**files**: id, name, physical_id (FK), parent_id (FK), owner_id (FK), is_folder, deleted_at, created_at, updated_at

Indexes: `(owner_id, parent_id, deleted_at)`, `(owner_id, deleted_at)`, `(parent_id, name, deleted_at)`, `(physical_id)`, `(md5 on physical_files)`

## API Routes

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/auth/login` | Login |
| POST | `/api/auth/password` | Change password |
| GET | `/api/files?folderId={id}` | List files |
| GET | `/api/files/lookup?parentId={id}&name={name}` | Lookup by name |
| PUT | `/api/files/{id}` | Rename |
| DELETE | `/api/files/{id}` | Move to trash |
| PATCH | `/api/files/move` | Move files |
| GET | `/api/files/{id}/download` | Download file |
| POST | `/api/folders` | Create folder |
| GET | `/api/folders/{id}/download` | Download folder as ZIP |
| POST | `/api/upload/init` | Init upload (instant upload check) |
| PUT | `/api/upload/{uploadID}/chunk/{index}` | Upload chunk |
| GET | `/api/upload/{uploadID}/progress` | Upload progress |
| POST | `/api/upload/{uploadID}/complete` | Complete upload |
| GET | `/api/trash` | List trash |
| POST | `/api/trash/{id}/restore` | Restore |
| DELETE | `/api/trash/{id}` | Permanent delete |

## CLI Tool (Phase 3)

```bash
cloudbox login       # Login
cloudbox ls [path]   # List files
cloudbox upload <local> [remote]  # Upload
cloudbox download <remote> [local] # Download
cloudbox rm <path>   # Delete (to trash)
cloudbox mkdir <path> # Create folder
```

Config stored at `~/.cloudbox/config.json` with `0600` permissions.

## Configuration

All paths are relative to executable location, converted to absolute paths at startup:
- Database: `./data/cloudbox.db`
- File storage: `./data/files/`
- Temp chunks: `./data/temp/`
- Thumbnails: `./data/thumbnails/`
- Logs: `./data/logs/cloudbox.log`

JWT secret from `JWT_SECRET` env var or generated randomly (warns in production).