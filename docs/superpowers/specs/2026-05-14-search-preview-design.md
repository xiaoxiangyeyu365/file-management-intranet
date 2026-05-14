# CloudBox 文件搜索与预览功能设计

## 1. 概述

为 CloudBox 后端补充两个功能：
- 文件搜索：按文件名模糊搜索，支持多种排序和范围限制
- 文件预览：图片元数据提取（尺寸、EXIF），支持按需和上传时提取

## 2. 文件搜索

### 2.1 API 设计

**接口：** `GET /api/files/search`

**参数：**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| keyword | string | 是 | 搜索关键词 |
| folderId | int64 | 否 | 文件夹ID，空则全局搜索，有值则递归搜索子文件夹 |
| sort | string | 否 | 排序方式：`relevance`（默认）、`time`、`name` |

**响应：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "files": [
      {
        "id": 1,
        "name": "文档.pdf",
        "isFolder": false,
        "size": 102400,
        "createdAt": "2026-05-14T10:00:00Z",
        "updatedAt": "2026-05-14T10:00:00Z"
      }
    ]
  }
}
```

### 2.2 搜索逻辑

#### 2.2.1 递归搜索子文件夹

当 folderId 有值时，使用 SQLite 递归 CTE 一次性查询所有子孙文件夹，避免应用层逐级拉取：

```sql
WITH RECURSIVE subfolders AS (
    SELECT id FROM files
    WHERE id = ? AND owner_id = ? AND is_folder = 1 AND deleted_at IS NULL
    UNION ALL
    SELECT f.id FROM files f
    JOIN subfolders s ON f.parent_id = s.id
    WHERE f.owner_id = ? AND f.is_folder = 1 AND f.deleted_at IS NULL
)
SELECT f.* FROM files f
JOIN subfolders s ON f.parent_id = s.id
WHERE f.owner_id = ?
  AND f.deleted_at IS NULL
  AND f.name LIKE '%' || ? || '%'
ORDER BY ...
```

#### 2.2.2 相关性排序

使用 SQL CASE 表达式实现相关度排序：

```sql
ORDER BY
  CASE
    WHEN name = ? THEN 0
    WHEN name LIKE ? || '%' THEN 1
    ELSE 2
  END,
  name ASC
```

#### 2.2.3 排序方式

- `relevance`（相关度）：完全匹配 > 开头匹配 > 包含匹配（CASE 加权）
- `time`：按更新时间降序
- `name`：按 UTF-8 字节序（SQLite 原生不支持拼音排序，接受字节序作为中文排序）

#### 2.2.4 搜索性能

- 当前数据量小，`LIKE '%keyword%'` 性能足够
- 文件数过万时，建议升级为 FTS5 全文搜索（预留扩展）

### 2.3 Repository 层

```go
// FileRepository 新增方法
func (r *FileRepository) Search(ctx context.Context, userID int64, keyword string, folderID *int64, sort string) ([]model.File, error)
```

实现要点：
- 使用原生 SQL + 递归 CTE（folderId 有值时）
- 动态构造 ORDER BY 子句
- GORM 占位符防 SQL 注入

### 2.4 Service 层

```go
// FileService 新增方法
func (s *FileService) SearchFiles(ctx context.Context, userID int64, keyword string, folderID *int64, sort string) ([]model.File, error)
```

### 2.5 Handler 层

```go
func (h *FileHandler) SearchFiles(c *gin.Context)
```

---

## 3. 文件预览服务

### 3.1 架构设计

预览服务作为独立模块，与主服务同进程运行：

```
┌─────────────────────────────────────────────────────┐
│                   Go Server (:8080)                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │
│  │ FileService │  │UploadService│  │PreviewService│ │
│  └─────────────┘  └─────────────┘  └─────────────┘ │
│         │                │                │         │
│         └────────────────┴────────────────┘         │
│                          │                          │
│                    ┌─────┴─────┐                    │
│                    │Repository │                    │
│                    └───────────┘                    │
└─────────────────────────────────────────────────────┘
```

### 3.2 API 设计

**接口：** `GET /api/files/:id/metadata`

**响应：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "width": 1920,
    "height": 1080,
    "exif": {
      "Camera": "Canon EOS 5D Mark IV",
      "DateTimeOriginal": "2026-05-14T10:30:00Z",
      "GPSLatitude": 39.9042,
      "GPSLongitude": 116.4074,
      "ISOSpeedRatings": 400,
      "FNumber": "f/2.8",
      "ExposureTime": "1/125",
      "FocalLength": "50mm",
      "Software": "Adobe Lightroom",
      "Orientation": 1
    }
  }
}
```

### 3.3 数据库设计

`physical_files` 表新增字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| width | INT | 图片宽度（像素），用于缩略图生成优化 |
| height | INT | 图片高度（像素） |
| metadata_json | TEXT | 图片元数据（JSON 格式），包含 EXIF 信息 |

**设计说明：**
- 仅保留 `width`、`height` 作为独立字段（缩略图生成需要）
- 所有 EXIF 信息存储在 `metadata_json`，扩展性强
- 非 图片文件的这些字段为 NULL，SQLite NULL 开销小

### 3.4 元数据提取流程

#### 上传时提取（合并缩略图生成）：

```
上传完成 → 后台 goroutine:
  1. 打开图片文件
  2. image.DecodeConfig 快速获取宽高
  3. 若需要缩略图，生成并保存
  4. go-exif 解析完整 EXIF
  5. 组装 metadata_json
  6. 一次性更新 physical_files (thumbnail_path, width, height, metadata_json)
```

#### 按需提取：

```
请求元数据 → 检查 metadata_json:
  - 非空：直接返回
  - 为空：加锁提取，更新数据库，返回结果
```

### 3.5 并发控制

按需提取时使用与缩略图生成相同的文件锁机制：
- 锁文件：`{physicalID}.meta.lock`
- 检查 metadata_json 非空则直接返回
- 空则获取锁，提取后更新数据库

### 3.6 EXIF 提取实现

使用 Go 库：`github.com/dsoprea/go-exif/v3`

**提取字段（存入 metadata_json）：**
- 相机：`Make` + `Model` → `Camera`
- 拍摄时间：`DateTimeOriginal`
- GPS：`GPSLatitude`、`GPSLongitude`
- ISO：`ISOSpeedRatings`
- 光圈：`FNumber`
- 快门：`ExposureTime`
- 焦距：`FocalLength`
- 其他：全部原始 EXIF 标签

### 3.7 Service 层

```go
// PreviewService 新建
type PreviewService struct {
    physicalRepo *repository.PhysicalFileRepository
    storage      *storage.StorageManager
}

func NewPreviewService(physicalRepo *repository.PhysicalFileRepository, storage *storage.StorageManager) *PreviewService

// 合并处理：生成缩略图 + 提取元数据
func (s *PreviewService) ProcessImage(ctx context.Context, physicalID int64) error

// 获取元数据（按需提取）
func (s *PreviewService) GetMetadata(ctx context.Context, userID, fileID int64) (*ImageMetadata, error)
```

### 3.8 Handler 层

```go
// PreviewHandler 新建
type PreviewHandler struct {
    previewService *service.PreviewService
    fileService    *service.FileService
}

func (h *PreviewHandler) GetMetadata(c *gin.Context)
```

### 3.9 权限校验

GetMetadata 通过 fileId 找到文件记录，校验 `owner_id == 当前用户`，再通过 `physical_id` 获取物理文件元数据。

---

## 4. 文件变更清单

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/model/physical_file.go` | Modify | 新增 width, height, metadata_json 字段 |
| `internal/repository/file.go` | Modify | 新增 Search 方法（递归 CTE） |
| `internal/repository/physical_file.go` | Modify | 新增 UpdateMetadata 方法 |
| `internal/service/file.go` | Modify | 新增 SearchFiles 方法 |
| `internal/service/preview.go` | Create | 新建预览服务（合并缩略图+元数据） |
| `internal/service/upload.go` | Modify | 上传完成后调用 PreviewService.ProcessImage |
| `internal/handler/file.go` | Modify | 新增 SearchFiles handler |
| `internal/handler/preview.go` | Create | 新建预览 handler |
| `cmd/server/main.go` | Modify | 注册新路由 |
| `go.mod` | Modify | 添加 EXIF 解析库依赖 |

---

## 5. API 路由汇总

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/files/search` | 文件搜索 |
| GET | `/api/files/:id/metadata` | 获取图片元数据 |

---

## 6. 扩展预留

**文件搜索扩展：**
- 支持文件类型筛选（MIME type）
- 支持文件大小范围筛选
- 支持时间范围筛选
- 升级为 FTS5 全文搜索（文件数过万时）

**文件预览扩展：**
- PDF 预览（转换为图片）
- 纯文本/代码预览（语法高亮）
- Office 文档预览（需转换服务）
- 视频预览（提取关键帧）
