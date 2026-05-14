# CloudBox 下载接口补充设计

## 1. 概述

补全 CloudBox 后端缺失的 4 个下载相关接口：
- 文件下载
- 缩略图获取
- 文件夹 ZIP 打包下载
- 清空回收站

## 2. 文件下载 `GET /api/files/{id}/download`

### 2.1 接口说明

下载指定 ID 的文件，支持断点续传和中文文件名。

### 2.2 Service 层

```go
// FileService 新增方法
func (s *FileService) DownloadFile(ctx context.Context, userID, fileID int64) (*model.File, *model.PhysicalFile, error)
```

**逻辑**：
1. 根据 fileID 和 userID 查询文件记录
2. 验证文件不是文件夹
3. 加载 PhysicalFile 关联
4. 返回文件信息和物理文件路径

### 2.3 Handler 层

```go
func (h *FileHandler) DownloadFile(c *gin.Context)
```

**逻辑**：
1. 调用 service 获取文件信息
2. 设置响应头：
   - `Content-Type`: 从 PhysicalFile.MimeType 获取
   - `Content-Disposition`: RFC 5987 格式编码中文文件名
   - `Content-Length`: 文件大小
3. 调用 `c.File(absolutePath)` 支持断点续传

**中文文件名处理**：
```go
encodedName := url.PathEscape(file.Name)
c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", encodedName))
```

---

## 3. 缩略图获取 `GET /api/files/{id}/thumbnail`

### 3.1 接口说明

获取图片文件的缩略图，按需生成，固定尺寸 200x200 像素。

### 3.2 Service 层

```go
// FileService 新增方法
func (s *FileService) GetThumbnail(ctx context.Context, userID, fileID int64) (string, error)
```

**逻辑**：
1. 获取文件信息，验证所有权
2. 验证是否为图片类型（jpg, jpeg, png, gif, webp）
3. 检查 `physical_files.thumbnail_path` 是否已存在
   - 存在 → 直接返回路径
   - 不存在 → 生成缩略图
4. 使用文件锁保证并发安全
5. 更新 `physical_files.thumbnail_path`
6. 返回缩略图路径

### 3.3 缩略图生成

**工具**：Go 标准库 `image` + `draw`

**流程**：
1. 解码原图
2. 计算缩放比例，保持宽高比
3. 缩放到最大 200x200
4. 编码为 JPEG
5. 保存到 `data/thumbnails/{physicalID}.jpg`

**并发安全**：
- 使用 `os.OpenFile` with `O_EXCL | O_CREATE` 创建锁文件
- 如果锁文件已存在，等待重试或返回占位图

### 3.4 Handler 层

```go
func (h *FileHandler) GetThumbnail(c *gin.Context)
```

**逻辑**：
1. 调用 service 获取缩略图路径
2. 返回缩略图文件

---

## 4. 文件夹 ZIP 下载 `GET /api/folders/{id}/download`

### 4.1 接口说明

将文件夹打包为 ZIP 文件下载，流式生成，不支持断点续传。

### 4.2 Service 层

```go
// FileService 新增方法
func (s *FileService) StreamFolderZip(ctx context.Context, userID, folderID int64, writer io.Writer) error
```

**逻辑**：
1. 验证文件夹存在且属于用户
2. 创建 `zip.NewWriter(writer)`
3. 递归遍历文件夹
4. 对每个文件：
   - 创建 zip entry（保留相对路径）
   - 写入文件内容
5. 调用 `zipWriter.Close()`

**路径处理**：
- ZIP 内路径格式：`文件夹名/子文件夹/文件名`
- 中文路径使用 UTF-8 编码

### 4.3 Handler 层

```go
func (h *FileHandler) DownloadFolder(c *gin.Context)
```

**逻辑**：
1. 获取文件夹信息
2. 设置响应头：
   - `Content-Type`: `application/zip`
   - `Content-Disposition`: RFC 5987 格式，文件名 `{文件夹名}.zip`
   - 不设置 `Content-Length`（流式传输）
3. 调用 service 流式写入 `c.Writer`

---

## 5. 清空回收站 `DELETE /api/trash`

### 5.1 接口说明

清空当前用户回收站中的所有文件，同步执行，事务保证原子性。

### 5.2 Service 层

```go
// FileService 新增方法
func (s *FileService) EmptyTrash(ctx context.Context, userID int64) error
```

**逻辑**：
1. 查询用户所有 `deleted_at IS NOT NULL` 的文件
2. 对每个文件递归收集所有子孙文件（复用 `FindAllDescendants`）
3. 统计物理文件引用计数（去重）
4. 在事务中：
   - 删除所有文件记录
   - 原子更新 `physical_files.ref_count`
   - 删除 `ref_count <= 0` 的物理文件和缩略图
   - 删除 `ref_count <= 0` 的 `physical_files` 记录

### 5.3 Handler 层

```go
func (h *TrashHandler) EmptyTrash(c *gin.Context)
```

**逻辑**：
1. 调用 service 清空回收站
2. 返回成功响应

---

## 6. 文件变更清单

| 文件 | 变更 |
|------|------|
| `internal/service/file.go` | 新增 `DownloadFile`, `GetThumbnail`, `StreamFolderZip`, `EmptyTrash` |
| `internal/handler/file.go` | 新增 `DownloadFile`, `GetThumbnail`, `DownloadFolder` |
| `internal/handler/trash.go` | 新增 `EmptyTrash` |
| `internal/repository/physical_file.go` | 新增 `UpdateThumbnailPath` 方法 |
| `cmd/server/main.go` | 注册新路由 |
| `go.mod` | 添加图片处理依赖（如需要） |

---

## 7. API 路由汇总

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/files/:id/download` | 下载文件 |
| GET | `/api/files/:id/thumbnail` | 获取缩略图 |
| GET | `/api/folders/:id/download` | 下载文件夹（ZIP） |
| DELETE | `/api/trash` | 清空回收站 |

---

## 8. 技术要点

### 8.1 中文文件名编码

使用 RFC 5987 格式：
```go
encodedName := url.PathEscape(filename)
c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", encodedName))
```

### 8.2 缩略图并发安全

使用排他创建锁文件：
```go
lockPath := thumbnailPath + ".lock"
f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
if err != nil {
    // 另一个进程正在生成，等待或返回默认图
}
defer os.Remove(lockPath)
defer f.Close()
```

### 8.3 ZIP 流式生成

```go
zipWriter := zip.NewWriter(c.Writer)
defer zipWriter.Close()

// 递归写入文件
filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
    // 创建 zip entry，写入内容
})
```

### 8.4 事务处理

清空回收站使用 GORM 事务：
```go
return s.db.Transaction(func(tx *gorm.DB) error {
    // 所有操作在同一事务中
})
```
