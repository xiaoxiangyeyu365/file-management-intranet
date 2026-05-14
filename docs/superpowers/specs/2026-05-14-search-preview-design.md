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

**匹配规则：**
- 文件名不区分大小写
- 使用 `LIKE '%keyword%'` 模糊匹配
- 仅搜索当前用户的文件（owner_id 过滤）
- 仅搜索未删除的文件（deleted_at IS NULL）

**排序规则：**
- `relevance`（相关度）：完全匹配 > 开头匹配 > 包含匹配
- `time`：按更新时间降序
- `name`：按文件名升序（拼音排序）

**范围限制：**
- folderId 为空：搜索用户所有文件
- folderId 有值：递归搜索指定文件夹及其所有子文件夹

### 2.3 Repository 层

```go
// FileRepository 新增方法
func (r *FileRepository) Search(ctx context.Context, userID int64, keyword string, folderID *int64, sort string) ([]model.File, error)
```

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
    "camera": "Canon EOS 5D Mark IV",
    "takenAt": "2026-05-14T10:30:00Z",
    "gpsLat": 39.9042,
    "gpsLng": 116.4074,
    "iso": 400,
    "aperture": "f/2.8",
    "shutterSpeed": "1/125",
    "focalLength": "50mm",
    "exif": {
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
| width | INT | 图片宽度（像素） |
| height | INT | 图片高度（像素） |
| camera | VARCHAR(255) | 相机型号 |
| taken_at | DATETIME | 拍摄时间 |
| gps_lat | DECIMAL(10,7) | 纬度 |
| gps_lng | DECIMAL(10,7) | 经度 |
| iso | INT | ISO 感光度 |
| aperture | VARCHAR(20) | 光圈值（如 f/2.8） |
| shutter_speed | VARCHAR(20) | 快门速度（如 1/125） |
| focal_length | VARCHAR(20) | 焦距（如 50mm） |
| exif_json | TEXT | 其他 EXIF 信息（JSON 格式） |

### 3.4 元数据提取流程

**上传时提取：**
1. 图片上传完成后，UploadService 调用 PreviewService
2. PreviewService 检测文件 MIME 类型
3. 若为图片，提取元数据并存储到数据库

**按需提取：**
1. 请求元数据时，检查数据库是否已有
2. 若有，直接返回
3. 若无，实时提取并更新数据库

### 3.5 EXIF 提取实现

使用 Go 库：`github.com/dsoprea/go-exif/v3`

**提取字段映射：**
- 相机型号：`Make` + `Model`
- 拍摄时间：`DateTimeOriginal`
- GPS：`GPSLatitude` + `GPSLongitude`
- ISO：`ISOSpeedRatings`
- 光圈：`FNumber`
- 快门：`ExposureTime`
- 焦距：`FocalLength`

### 3.6 Service 层

```go
// PreviewService 新建
type PreviewService struct {
    physicalRepo *repository.PhysicalFileRepository
    storage      *storage.StorageManager
}

func NewPreviewService(physicalRepo *repository.PhysicalFileRepository, storage *storage.StorageManager) *PreviewService

// 提取并存储元数据
func (s *PreviewService) ExtractAndSaveMetadata(ctx context.Context, physicalID int64) error

// 获取元数据（按需提取）
func (s *PreviewService) GetMetadata(ctx context.Context, userID, fileID int64) (*ImageMetadata, error)
```

### 3.7 Handler 层

```go
// PreviewHandler 新建
type PreviewHandler struct {
    previewService *service.PreviewService
    fileService    *service.FileService
}

func (h *PreviewHandler) GetMetadata(c *gin.Context)
```

---

## 4. 文件变更清单

| 文件 | 变更类型 | 说明 |
|------|----------|------|
| `internal/model/physical_file.go` | Modify | 新增元数据字段 |
| `internal/repository/file.go` | Modify | 新增 Search 方法 |
| `internal/repository/physical_file.go` | Modify | 新增 UpdateMetadata 方法 |
| `internal/service/file.go` | Modify | 新增 SearchFiles 方法 |
| `internal/service/preview.go` | Create | 新建预览服务 |
| `internal/service/upload.go` | Modify | 上传完成后调用预览服务 |
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
- 未来可升级为全文搜索（FTS5）

**文件预览扩展：**
- PDF 预览（转换为图片）
- 纯文本/代码预览（语法高亮）
- Office 文档预览（需转换服务）
- 视频预览（提取关键帧）
