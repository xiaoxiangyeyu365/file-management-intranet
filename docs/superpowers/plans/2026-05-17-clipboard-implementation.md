# 云剪切板功能实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现跨设备文本剪切板同步功能

**Architecture:** 后端新增 ClipboardService 和 ClipboardHandler，使用 SQLite 存储；前端新增 ClipboardView 页面，使用轮询同步

**Tech Stack:** Go/Gin, GORM, Vue 3, Pinia

---

## 文件结构

**后端新增：**
- `internal/model/clipboard.go` - 数据模型
- `internal/repository/clipboard.go` - 数据访问层
- `internal/service/clipboard.go` - 业务逻辑层
- `internal/handler/clipboard.go` - HTTP 处理器

**后端修改：**
- `internal/repository/db.go` - 创建 clipboard_records 表和索引

**前端新增：**
- `web/src/views/ClipboardView.vue` - 主页面
- `web/src/stores/clipboard.js` - Pinia store

**前端修改：**
- `web/src/router/index.js` - 添加 /clipboard 路由
- `web/src/components/Layout/AppSidebar.vue` - 添加导航入口
- `web/src/utils/api.js` - 添加 clipboard API

---

## Task 1: 后端 - 数据模型和数据库

**Files:**
- Create: `internal/model/clipboard.go`
- Modify: `internal/repository/db.go`

### Step 1: 创建 clipboard.go 模型

创建文件 `internal/model/clipboard.go`：

```go
package model

import "time"

type ClipboardRecord struct {
    ID         int64     `gorm:"primaryKey" json:"id"`
    Content    string    `gorm:"type:text;not null" json:"content"`
    DeviceName string    `gorm:"size:100;default:'未命名设备'" json:"deviceName"`
    UserID     int64     `gorm:"not null;index" json:"userId"`
    Pinned     bool      `gorm:"default:false" json:"pinned"`
    CreatedAt  time.Time `json:"createdAt"`
}

func (ClipboardRecord) TableName() string {
    return "clipboard_records"
}

type ClipboardResponse struct {
    ID         int64     `json:"id"`
    Content    string    `json:"content"`
    DeviceName string    `json:"deviceName"`
    Pinned     bool      `json:"pinned"`
    CreatedAt  time.Time `json:"createdAt"`
}

func (c *ClipboardRecord) ToResponse() *ClipboardResponse {
    return &ClipboardResponse{
        ID:         c.ID,
        Content:    c.Content,
        DeviceName: c.DeviceName,
        Pinned:     c.Pinned,
        CreatedAt:  c.CreatedAt,
    }
}
```

### Step 2: 修改 db.go 创建表

在 `createIndexes()` 函数后添加：

```go
func createClipboardTable() error {
    var count int64
    DB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='clipboard_records'").Scan(&count)
    if count > 0 {
        return nil
    }

    sql := `CREATE TABLE clipboard_records (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        content TEXT NOT NULL,
        device_name TEXT DEFAULT '未命名设备',
        user_id INTEGER NOT NULL,
        pinned BOOLEAN DEFAULT FALSE,
        created_at DATETIME NOT NULL
    )`
    return DB.Exec(sql).Error
}

func createClipboardIndexes() error {
    if err := DB.Exec("CREATE INDEX IF NOT EXISTS idx_clipboard_user_pinned_created ON clipboard_records(user_id, pinned, created_at)").Error; err != nil {
        return err
    }
    return nil
}
```

在 `InitDB` 函数中调用（在 `createIndexes` 之后）：

```go
// Create clipboard table
if err := createClipboardTable(); err != nil {
    log.Fatalf("failed to create clipboard table: %v", err)
}
if err := createClipboardIndexes(); err != nil {
    log.Fatalf("failed to create clipboard indexes: %v", err)
}
```

### Step 3: 提交

```bash
git add internal/model/clipboard.go internal/repository/db.go
git commit -m "feat(clipboard): add ClipboardRecord model and table"
```

---

## Task 2: 后端 - Repository

**Files:**
- Create: `internal/repository/clipboard.go`

### Step 1: 创建 clipboard repository

创建文件 `internal/repository/clipboard.go`：

```go
package repository

import (
    "cloudbox/internal/model"
    "context"

    "gorm.io/gorm"
)

type ClipboardRepository struct {
    db *gorm.DB
}

func NewClipboardRepository(db *gorm.DB) *ClipboardRepository {
    return &ClipboardRepository{db: db}
}

func (r *ClipboardRepository) Create(ctx context.Context, record *model.ClipboardRecord) error {
    return r.db.WithContext(ctx).Create(record).Error
}

func (r *ClipboardRepository) FindByUser(ctx context.Context, userID int64, limit int) ([]model.ClipboardRecord, error) {
    var records []model.ClipboardRecord
    err := r.db.WithContext(ctx).
        Where("user_id = ?", userID).
        Order("pinned DESC, created_at DESC").
        Limit(limit).
        Find(&records).Error
    return records, err
}

func (r *ClipboardRepository) FindByIDAndUser(ctx context.Context, id, userID int64) (*model.ClipboardRecord, error) {
    var record model.ClipboardRecord
    err := r.db.WithContext(ctx).
        Where("id = ? AND user_id = ?", id, userID).
        First(&record).Error
    if err != nil {
        return nil, err
    }
    return &record, nil
}

func (r *ClipboardRepository) DeleteByID(ctx context.Context, id int64) error {
    return r.db.WithContext(ctx).Delete(&model.ClipboardRecord{}, id).Error
}

func (r *ClipboardRepository) UpdatePinned(ctx context.Context, id int64, pinned bool) error {
    return r.db.WithContext(ctx).
        Model(&model.ClipboardRecord{}).
        Where("id = ?", id).
        Update("pinned", pinned).Error
}

func (r *ClipboardRepository) DeleteByUserUnpinned(ctx context.Context, userID int64) error {
    return r.db.WithContext(ctx).
        Where("user_id = ? AND pinned = ?", userID, false).
        Delete(&model.ClipboardRecord{}).Error
}

func (r *ClipboardRepository) DeleteByUser(ctx context.Context, userID int64) error {
    return r.db.WithContext(ctx).
        Where("user_id = ?", userID).
        Delete(&model.ClipboardRecord{}).Error
}

func (r *ClipboardRepository) CountUnpinned(ctx context.Context, userID int64) (int64, error) {
    var count int64
    err := r.db.WithContext(ctx).
        Model(&model.ClipboardRecord{}).
        Where("user_id = ? AND pinned = ?", userID, false).
        Count(&count).Error
    return count, err
}

func (r *ClipboardRepository) DeleteOldestUnpinned(ctx context.Context, userID int64, keepCount int) error {
    sql := `DELETE FROM clipboard_records
            WHERE id IN (
                SELECT id FROM clipboard_records
                WHERE user_id = ? AND pinned = 0
                ORDER BY created_at ASC
                LIMIT ?
            )`
    return r.db.WithContext(ctx).Exec(sql, userID, keepCount).Error
}
```

### Step 2: 提交

```bash
git add internal/repository/clipboard.go
git commit -m "feat(clipboard): add ClipboardRepository"
```

---

## Task 3: 后端 - Service

**Files:**
- Create: `internal/service/clipboard.go`

### Step 1: 创建 clipboard service

创建文件 `internal/service/clipboard.go`：

```go
package service

import (
    "cloudbox/internal/model"
    "cloudbox/internal/repository"
    "context"
    "errors"
)

var (
    ErrClipboardEmpty    = errors.New("content is empty")
    ErrClipboardTooLong  = errors.New("content exceeds 10KB limit")
    ErrClipboardNotFound = errors.New("clipboard record not found")
)

const (
    MaxClipboardContentSize = 10240 // 10KB
    MaxClipboardRecords     = 50
)

type ClipboardService struct {
    repo *repository.ClipboardRepository
}

func NewClipboardService(repo *repository.ClipboardRepository) *ClipboardService {
    return &ClipboardService{repo: repo}
}

type CreateClipboardRequest struct {
    Content    string
    DeviceName string
    UserID     int64
}

func (s *ClipboardService) Create(ctx context.Context, req CreateClipboardRequest) (*model.ClipboardRecord, error) {
    if req.Content == "" {
        return nil, ErrClipboardEmpty
    }
    if len(req.Content) > MaxClipboardContentSize {
        return nil, ErrClipboardTooLong
    }

    deviceName := req.DeviceName
    if deviceName == "" {
        deviceName = "未命名设备"
    }

    record := &model.ClipboardRecord{
        Content:    req.Content,
        DeviceName: deviceName,
        UserID:     req.UserID,
        Pinned:     false,
    }

    if err := s.repo.Create(ctx, record); err != nil {
        return nil, err
    }

    // Auto cleanup: keep max 50 unpinned records
    count, err := s.repo.CountUnpinned(ctx, req.UserID)
    if err == nil && count > MaxClipboardRecords {
        deleteCount := count - MaxClipboardRecords + 1
        s.repo.DeleteOldestUnpinned(ctx, req.UserID, deleteCount)
    }

    return record, nil
}

func (s *ClipboardService) List(ctx context.Context, userID int64) ([]model.ClipboardRecord, error) {
    return s.repo.FindByUser(ctx, userID, MaxClipboardRecords)
}

func (s *ClipboardService) TogglePin(ctx context.Context, userID, recordID int64, pinned bool) error {
    _, err := s.repo.FindByIDAndUser(ctx, recordID, userID)
    if err != nil {
        return ErrClipboardNotFound
    }
    return s.repo.UpdatePinned(ctx, recordID, pinned)
}

func (s *ClipboardService) Delete(ctx context.Context, userID, recordID int64) error {
    _, err := s.repo.FindByIDAndUser(ctx, recordID, userID)
    if err != nil {
        return ErrClipboardNotFound
    }
    return s.repo.DeleteByID(ctx, recordID)
}

func (s *ClipboardService) ClearUnpinned(ctx context.Context, userID int64) error {
    return s.repo.DeleteByUserUnpinned(ctx, userID)
}

func (s *ClipboardService) ClearAll(ctx context.Context, userID int64) error {
    return s.repo.DeleteByUser(ctx, userID)
}
```

### Step 2: 提交

```bash
git add internal/service/clipboard.go
git commit -m "feat(clipboard): add ClipboardService"
```

---

## Task 4: 后端 - Handler 和路由

**Files:**
- Create: `internal/handler/clipboard.go`
- Modify: `cmd/server/main.go`

### Step 1: 创建 clipboard handler

创建文件 `internal/handler/clipboard.go`：

```go
package handler

import (
    "cloudbox/internal/service"
    "cloudbox/internal/util/response"
    "strconv"

    "github.com/gin-gonic/gin"
)

type ClipboardHandler struct {
    service *service.ClipboardService
}

func NewClipboardHandler(s *service.ClipboardService) *ClipboardHandler {
    return &ClipboardHandler{service: s}
}

type CreateClipboardRequest struct {
    Content string `json:"content" binding:"required"`
}

type UpdatePinRequest struct {
    Pinned bool `json:"pinned"`
}

func (h *ClipboardHandler) List(c *gin.Context) {
    userID := GetUserID(c)

    records, err := h.service.List(c.Request.Context(), userID)
    if err != nil {
        response.InternalError(c, "failed to list clipboard records")
        return
    }

    responses := make([]*service.ClipboardResponse, len(records))
    for i := range records {
        responses[i] = records[i].ToResponse()
    }

    response.Success(c, gin.H{
        "records": responses,
    })
}

func (h *ClipboardHandler) Create(c *gin.Context) {
    userID := GetUserID(c)

    var req CreateClipboardRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        response.BadRequest(c, "content is required")
        return
    }

    deviceName := c.GetHeader("X-Device-Name")
    if deviceName == "" {
        deviceName = "未命名设备"
    }

    record, err := h.service.Create(c.Request.Context(), service.CreateClipboardRequest{
        Content:    req.Content,
        DeviceName: deviceName,
        UserID:     userID,
    })
    if err != nil {
        if err == service.ErrClipboardEmpty {
            response.BadRequest(c, "content is empty")
            return
        }
        if err == service.ErrClipboardTooLong {
            response.BadRequest(c, "content exceeds 10KB limit")
            return
        }
        response.InternalError(c, "failed to create clipboard record")
        return
    }

    response.Success(c, record.ToResponse())
}

func (h *ClipboardHandler) UpdatePin(c *gin.Context) {
    userID := GetUserID(c)
    recordID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

    var req UpdatePinRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        response.BadRequest(c, "invalid request")
        return
    }

    err := h.service.TogglePin(c.Request.Context(), userID, recordID, req.Pinned)
    if err != nil {
        if err == service.ErrClipboardNotFound {
            response.NotFound(c, "clipboard record not found")
            return
        }
        response.InternalError(c, "failed to update pin status")
        return
    }

    response.Success(c, nil)
}

func (h *ClipboardHandler) Delete(c *gin.Context) {
    userID := GetUserID(c)
    recordID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

    err := h.service.Delete(c.Request.Context(), userID, recordID)
    if err != nil {
        if err == service.ErrClipboardNotFound {
            response.NotFound(c, "clipboard record not found")
            return
        }
        response.InternalError(c, "failed to delete clipboard record")
        return
    }

    response.Success(c, nil)
}

func (h *ClipboardHandler) Clear(c *gin.Context) {
    userID := GetUserID(c)

    onlyUnpinned := c.Query("onlyUnpinned") != "false"

    var err error
    if onlyUnpinned {
        err = h.service.ClearUnpinned(c.Request.Context(), userID)
    } else {
        err = h.service.ClearAll(c.Request.Context(), userID)
    }

    if err != nil {
        response.InternalError(c, "failed to clear clipboard records")
        return
    }

    response.Success(c, nil)
}
```

### Step 2: 修改 main.go 注册路由

在 main.go 中：

1. 添加导入（如果尚未导入）：
```go
"cloudbox/internal/model"
```

2. 在初始化服务部分添加：
```go
clipboardRepo := repository.NewClipboardRepository(db)
clipboardService := service.NewClipboardService(clipboardRepo)
clipboardHandler := handler.NewClipboardHandler(clipboardService)
```

3. 在 protected 路由组中添加：
```go
// Clipboard
protected.GET("/clipboard", clipboardHandler.List)
protected.POST("/clipboard", clipboardHandler.Create)
protected.PATCH("/clipboard/:id/pin", clipboardHandler.UpdatePin)
protected.DELETE("/clipboard/:id", clipboardHandler.Delete)
protected.DELETE("/clipboard", clipboardHandler.Clear)
```

### Step 3: 提交

```bash
git add internal/handler/clipboard.go cmd/server/main.go
git commit -m "feat(clipboard): add ClipboardHandler and routes"
```

---

## Task 5: 前端 - API 和 Store

**Files:**
- Modify: `web/src/utils/api.js`
- Create: `web/src/stores/clipboard.js`

### Step 1: 修改 api.js 添加 clipboard API

在 `web/src/utils/api.js` 文件末尾添加：

```javascript
// Clipboard API
export const clipboardAPI = {
  list: () => api.get('/clipboard'),
  create: (content) => api.post('/clipboard', { content }),
  togglePin: (id, pinned) => api.patch(`/clipboard/${id}/pin`, { pinned }),
  delete: (id) => api.delete(`/clipboard/${id}`),
  clear: (onlyUnpinned = true) => api.delete('/clipboard', {
    params: { onlyUnpinned: onlyUnpinned.toString() }
  })
}
```

### Step 2: 创建 clipboard store

创建文件 `web/src/stores/clipboard.js`：

```javascript
import { defineStore } from 'pinia'
import { clipboardAPI } from '@/utils/api'
import { ElMessage } from 'element-plus'

const MAX_CONTENT_SIZE = 10240

export const useClipboardStore = defineStore('clipboard', {
  state: () => ({
    records: [],
    loading: false,
    pollingInterval: null
  }),

  getters: {
    pinnedRecords: (state) => state.records.filter(r => r.pinned),
    unpinnedRecords: (state) => state.records.filter(r => !r.pinned)
  },

  actions: {
    isNew(record) {
      const createdTime = new Date(record.createdAt).getTime()
      return Date.now() - createdTime < 5000
    },

    async fetchRecords() {
      this.loading = true
      try {
        const response = await clipboardAPI.list()
        const data = response?.data || response
        this.records = data.records || []
      } catch (err) {
        console.error('Failed to fetch clipboard records:', err)
      } finally {
        this.loading = false
      }
    },

    async createRecord(content) {
      if (!content || content.trim() === '') {
        ElMessage.error('请输入内容')
        return null
      }
      if (content.length > MAX_CONTENT_SIZE) {
        ElMessage.error('内容过长（最多 10240 字符）')
        return null
      }

      try {
        const response = await clipboardAPI.create(content)
        const data = response?.data || response
        this.records.unshift(data)
        if (this.records.length > 50) {
          this.records = this.records.slice(0, 50)
        }
        ElMessage.success('已保存到云剪切板')
        return data
      } catch (err) {
        ElMessage.error('保存失败')
        return null
      }
    },

    async togglePin(record) {
      try {
        await clipboardAPI.togglePin(record.id, !record.pinned)
        record.pinned = !record.pinned
      } catch (err) {
        ElMessage.error('操作失败')
      }
    },

    async deleteRecord(record) {
      try {
        await clipboardAPI.delete(record.id)
        this.records = this.records.filter(r => r.id !== record.id)
        ElMessage.success('已删除')
      } catch (err) {
        ElMessage.error('删除失败')
      }
    },

    async clearAll(onlyUnpinned = true) {
      try {
        await clipboardAPI.clear(onlyUnpinned)
        if (onlyUnpinned) {
          this.records = this.records.filter(r => r.pinned)
        } else {
          this.records = []
        }
        ElMessage.success('已清空')
      } catch (err) {
        ElMessage.error('清空失败')
      }
    },

    startPolling(intervalMs = 5000) {
      this.stopPolling()
      this.pollingInterval = setInterval(() => {
        this.fetchRecords()
      }, intervalMs)
    },

    stopPolling() {
      if (this.pollingInterval) {
        clearInterval(this.pollingInterval)
        this.pollingInterval = null
      }
    }
  }
})
```

### Step 3: 提交

```bash
git add web/src/utils/api.js web/src/stores/clipboard.js
git commit -m "feat(clipboard): add clipboard API and store"
```

---

## Task 6: 前端 - 路由和侧边栏

**Files:**
- Modify: `web/src/router/index.js`
- Modify: `web/src/components/Layout/AppSidebar.vue`

### Step 1: 修改 router/index.js

添加导入和路由：

```javascript
import ClipboardView from '@/views/ClipboardView.vue'
```

在 routes 数组中添加：

```javascript
{
  path: '/clipboard',
  name: 'Clipboard',
  component: ClipboardView,
  meta: { requiresAuth: true }
}
```

### Step 2: 修改 AppSidebar.vue

添加导航项（在回收站路由之后）：

```vue
<router-link
  to="/clipboard"
  class="nav-item"
  :class="{ active: route.path === '/clipboard' }"
>
  <el-icon><Document /></el-icon>
  <span>云剪切板</span>
</router-link>
```

添加导入：

```javascript
import { Folder, Delete, Document } from '@element-plus/icons-vue'
```

### Step 3: 提交

```bash
git add web/src/router/index.js web/src/components/Layout/AppSidebar.vue
git commit -m "feat(clipboard): add clipboard route and sidebar entry"
```

---

## Task 7: 前端 - ClipboardView 页面

**Files:**
- Create: `web/src/views/ClipboardView.vue`

### Step 1: 创建 ClipboardView.vue

创建文件 `web/src/views/ClipboardView.vue`，完整代码见计划文档。

关键功能：
- 文本输入框 + 保存按钮
- 设备名称设置
- 记录列表（置顶优先）
- 点击复制
- 置顶/取消置顶
- 删除
- 清空

### Step 2: 提交

```bash
git add web/src/views/ClipboardView.vue
git commit -m "feat(clipboard): add ClipboardView page"
```

---

## Task 8: 测试和构建

### Step 1: 构建后端

```bash
cd E:/fileManagementIntranet
go build -o cloudbox.exe ./cmd/server
```

### Step 2: 构建前端

```bash
cd E:/fileManagementIntranet/web
npm run build
```

### Step 3: 测试 API

使用 curl 测试各个端点。

### Step 4: 提交

```bash
git add -A
git commit -m "feat(clipboard): complete clipboard feature implementation"
```