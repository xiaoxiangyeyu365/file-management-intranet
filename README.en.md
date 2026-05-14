# CloudBox Intranet Cloud Storage System

CloudBox is an intranet cloud storage system developed in Go, supporting core cloud storage features such as file upload, download, preview, search, and recycle bin management.

## Features

### File Management
- **File Browsing**: View and manage files by folder hierarchy
- **Folder Creation**: Support for creating multi-level folders
- **File Renaming**: Flexible renaming of files and folders
- **File Moving**: Support batch moving files to specified folders
- **File Deletion**: Soft delete to recycle bin

### File Upload
- **Chunked Upload**: Support for resumable chunked uploads for large files
- **MD5 Verification**: MD5 verification during upload, enabling instant upload functionality
- **Progress Tracking**: Real-time upload progress monitoring

### File Download
- **Single File Download**: Direct file download with support for Chinese filenames
- **Folder ZIP Download**: Pack entire folders into ZIP for download
- **Thumbnail Retrieval**: Retrieve thumbnails for image files

### Recycle Bin
- **View Recycle Bin**: List all deleted files
- **Restore Files**: Restore files from the recycle bin
- **Permanent Deletion**: Permanently delete files to free up storage space
- **Empty Recycle Bin**: One-click emptying of all recycle bin contents

### Search and Preview
- **File Search**: Search by filename keywords, supporting recursive subfolder search
- **Image Preview**: Retrieve image EXIF metadata (resolution, capture information, etc.)

### User Authentication
- **JWT Authentication**: Token-based secure authentication
- **Password Management**: Support for password modification

## Technology Stack

| Category | Technology |
|----------|------------|
| Backend Framework | Go + Gin |
| Database | SQLite + GORM |
| Authentication | JWT |
| Storage | Local File System |

## Quick Start

### Prerequisites

- Go 1.20+
- SQLite3

### Build and Run

```bash
# Full build (frontend + backend)
make build

# Build backend only
make build-backend

# Build frontend only
make build-frontend

# Run in development mode
make run
```

### Configuration

Configuration file located at `configs/config.yaml`:

```yaml
server:
  port: 8080        # Server port
  mode: debug       # Running mode

database:
  path: ./data.db   # Database path

storage:
  root: ./storage   # Storage root directory
  temp: ./temp      # Temporary files directory

jwt:
  secret: ""        # JWT secret (auto-generated)
  expiry: 168h      # Token expiration time

admin:
  username: admin   # Admin username
  password: admin   # Admin default password
```

The first run automatically creates a default admin account; it is recommended to change the password immediately after login.

## API Endpoints

### Authentication Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | /api/auth/login | User login |
| POST | /api/auth/change-password | Change password |
| POST | /api/auth/logout | User logout |
| GET | /api/auth/profile | Get user profile |

### File Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/files | Get file list |
| GET | /api/files/:id | Get file details |
| POST | /api/files | Create folder |
| PUT | /api/files/:id/rename | Rename file/folder |
| PUT | /api/files/:id/move | Move file |
| DELETE | /api/files/:id | Delete file |
| GET | /api/files/:id/download | Download file |
| GET | /api/files/:id/thumbnail | Get thumbnail |
| GET | /api/files/search | Search files |
| GET | /api/folders/:id/download | Download folder as ZIP |

### Upload Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | /api/upload/init | Initialize upload |
| POST | /api/upload/chunk | Upload chunk |
| GET | /api/upload/progress | Get upload progress |
| POST | /api/upload/complete | Complete upload |
| DELETE | /api/upload/cancel | Cancel upload |

### Recycle Bin Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | /api/trash | View recycle bin |
| POST | /api/trash/:id/restore | Restore file |
| DELETE | /api/trash/:id | Permanently delete |
| DELETE | /api/trash | Empty recycle bin |

## Project Structure

```
.
├── cmd/server/           # Application entry point
├── configs/             # Configuration files
├── internal/
│   ├── config/          # Configuration loading
│   ├── handler/        # HTTP handlers
│   ├── model/          # Data models
│   ├── repository/     # Data access layer
│   ├── service/        # Business logic layer
│   └── util/           # Utility functions
│       ├── crypto/     # Cryptography utilities
│       ├── response/   # Response wrappers
│       └── storage/    # Storage management
└── Makefile            # Build script
```

## License

MIT License