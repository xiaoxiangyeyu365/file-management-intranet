# CloudBox - 内网云盘系统设计文档

## 1. 项目概述

### 1.1 背景
校园网环境下，Windows 远程桌面可以正常工作，但文件管理器的网络位置无法访问共享文件夹。需要一个运行在内网的云盘应用，实现类似共享文件夹的功能。

### 1.2 目标
- 两台电脑之间共享文件
- 支持大文件上传下载
- 简单易用，双击运行
- 预留多用户扩展空间

### 1.3 功能范围

**进阶版（当前实现）：**
- 文件上传、下载、浏览
- 文件夹创建/删除/重命名
- 文件重命名、移动
- 分片上传、断点续传、秒传
- 图片缩略图生成
- 回收站机制
- JWT 登录认证

**预留扩展：**
- 多用户、多权限级别
- 文件搜索、文件预览
- MySQL 数据库支持

---

## 2. 系统架构

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                        用户浏览器                            │
│                      http://IP:8080                         │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│                      Go Server (:8080)                      │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              Static Files (Vue 前端)                   │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                 JWT Auth Middleware                    │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                    REST API 层                         │  │
│  │  /api/auth    /api/files    /api/folders              │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                    Service 层                          │  │
│  └───────────────────────────────────────────────────────┘  │
│  ┌───────────────────────────────────────────────────────┐  │
│  │               Repository 层 (接口抽象)                 │  │
│  └───────────────────────────────────────────────────────┘  │
└────────┬─────────────────────────┬─────────────────────────┘
         │                         │
         ▼                         ▼
┌─────────────────┐      ┌─────────────────────┐
│     SQLite      │      │   文件存储目录       │
│  cloudbox.db    │      │   /files/           │
└─────────────────┘      │   /thumbnails/      │
                         │   /temp/chunks/     │
                         └─────────────────────┘
```

### 2.2 技术选型

| 层级 | 技术栈 | 说明 |
|------|--------|------|
| 后端语言 | Go | 高性能、单文件部署 |
| Web 框架 | Gin | 轻量级 HTTP 框架 |
| 数据库 | SQLite | 单文件数据库，预留 MySQL 扩展 |
| ORM | GORM | Go 常用 ORM |
| 前端框架 | Vue 3 | 组合式 API |
| 状态管理 | Pinia | Vue 3 推荐 |
| 构建工具 | Vite | 快速构建 |
| UI 组件 | Element Plus | 可选 |

---

## 3. 数据库设计

### 3.1 ER 关系图

```
┌─────────────────┐
│     users       │
├─────────────────┤
│ id (PK)         │
│ username        │
│ password_hash   │
│ role            │
│ created_at      │
└────────┬────────┘
         │
         │ 1:N
         ▼
┌─────────────────────────────────────────────┐
│                    files                     │
├─────────────────────────────────────────────┤
│ id (PK)                                      │
│ name                                         │
│ owner_id (FK) ───────────▶ users.id         │
│ physical_id (FK) ─┐                          │
│ parent_id (FK)    │                          │
│ is_folder        │                           │
│ deleted_at       │                           │
│ created_at       │                           │
│ updated_at       │                           │
└──────────────────┼───────────────────────────┘
                   │
                   │ N:1
                   ▼
┌─────────────────────────────────────────────┐
│              physical_files                  │
├─────────────────────────────────────────────┤
│ id (PK)                                      │
│ storage_path                                 │
│ md5 (UNIQUE)                                │
│ size                                         │
│ mime_type                                    │
│ ref_count                                    │
│ thumbnail_path                               │
│ created_at                                   │
└─────────────────────────────────────────────┘
```

### 3.2 表结构定义

#### users 表
```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username VARCHAR(50) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(20) DEFAULT 'user',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

#### files 表
```sql
CREATE TABLE files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name VARCHAR(255) NOT NULL,
    physical_id INTEGER REFERENCES physical_files(id),
    parent_id INTEGER REFERENCES files(id),
    owner_id INTEGER NOT NULL REFERENCES users(id),
    is_folder BOOLEAN DEFAULT FALSE,
    deleted_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 索引
CREATE INDEX idx_files_owner_parent_deleted ON files(owner_id, parent_id, deleted_at);
CREATE INDEX idx_files_owner_deleted ON files(owner_id, deleted_at);
CREATE INDEX idx_files_parent_name_deleted ON files(parent_id, name, deleted_at);
CREATE INDEX idx_files_physical_id ON files(physical_id);
```

#### physical_files 表
```sql
CREATE TABLE physical_files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    storage_path VARCHAR(500) NOT NULL UNIQUE,
    md5 VARCHAR(32) NOT NULL UNIQUE,
    size BIGINT NOT NULL,
    mime_type VARCHAR(100),
    ref_count INTEGER DEFAULT 1,
    thumbnail_path VARCHAR(500),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_physical_files_md5 ON physical_files(md5);
```

---

## 4. API 设计

### 4.1 认证 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/login` | 登录 |
| POST | `/api/auth/password` | 修改密码 |
| POST | `/api/auth/logout` | 退出登录 |

### 4.2 文件 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/files?folderId={id}` | 列出文件夹内容 |
| GET | `/api/files/{id}` | 获取文件详情 |
| GET | `/api/files/lookup?parentId={id}&name={name}` | 按名称查找 |
| PUT | `/api/files/{id}` | 重命名 |
| DELETE | `/api/files/{id}` | 移入回收站 |
| PATCH | `/api/files/move` | 移动文件 |
| GET | `/api/files/{id}/download` | 下载文件 |
| GET | `/api/files/{id}/thumbnail` | 获取缩略图 |

### 4.3 文件夹 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/folders` | 创建文件夹 |
| GET | `/api/folders/{id}/download` | 下载文件夹（ZIP） |

### 4.4 上传 API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/upload/init` | 初始化上传（秒传检查） |
| PUT | `/api/upload/{uploadID}/chunk/{index}` | 上传分片 |
| GET | `/api/upload/{uploadID}/progress` | 查询进度 |
| POST | `/api/upload/{uploadID}/complete` | 完成上传 |
| DELETE | `/api/upload/{uploadID}` | 取消上传 |

### 4.5 回收站 API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/trash` | 回收站列表 |
| POST | `/api/trash/{id}/restore` | 恢复文件 |
| DELETE | `/api/trash/{id}` | 永久删除 |
| DELETE | `/api/trash` | 清空回收站 |

---

## 5. 文件上传设计

### 5.1 上传流程

```
Step 1: 前端计算 MD5（Web Worker）
        │
        ▼
Step 2: POST /api/upload/init
        │
        ├─ 秒传命中 → 返回 { instant: true, file }
        │
        └─ 需要上传 → 返回 { uploadID, chunkSize, chunksAlreadyDone }
                │
                ▼
Step 3: 分片并发上传（max=3）
        │
        ▼
Step 4: POST /api/upload/{uploadID}/complete
        │
        ▼
Step 5: 异步生成缩略图
```

### 5.2 uploadID 生成策略

```
uploadID = MD5(fileContent) + "_" + userID
临时目录: /temp/chunks/{uploadID}/
```

### 5.3 秒传机制

1. 根据 MD5 查找 `physical_files` 表
2. 若存在：创建新 `files` 记录，`physical_files.ref_count += 1`
3. 若不存在：进入分片上传流程

### 5.4 断点续传

- 前端 localStorage 存储 `pendingUploads`
- 初始化时返回已上传分片列表
- 前端跳过已完成分片

---

## 6. 安全设计

### 6.1 JWT 认证

- 登录成功返回 JWT Token
- Token 存储于 localStorage
- 所有 API 请求携带 `Authorization: Bearer <token>`
- 中间件校验 Token 有效性

### 6.2 用户隔离

- 所有文件操作自动过滤 `owner_id`
- 移动/删除前校验文件所有权
- 预留多用户扩展接口

### 6.3 敏感操作确认

| 操作 | 确认方式 |
|------|----------|
| 删除文件/文件夹 | 前端弹窗二次确认 |
| 批量删除 | 显示数量，确认 |
| 下载文件夹 | 提示打包下载 |

### 6.4 默认密码强制修改

- 首次登录检测默认密码
- 强制修改后方可使用

---

## 7. 核心业务逻辑

### 7.1 移动文件校验

1. 目标文件夹存在且未删除
2. 目标文件夹属于当前用户
3. 目标文件夹不是自己或子文件夹（循环引用）
4. 目标文件夹下无同名文件

### 7.2 恢复文件逻辑

1. 检查原始父文件夹是否存在
2. 若不存在 → 恢复到根目录
3. 检查同名冲突 → 自动重命名

### 7.3 永久删除逻辑

```
1. 递归获取所有子孙节点
2. 使用 Map 统计物理文件删除次数（去重）
3. 原子更新 ref_count
4. ref_count <= 0 时删除物理文件和缩略图
5. 全部操作在事务中完成
```

---

## 8. 项目目录结构

```
cloudbox/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── config/
│   ├── handler/
│   ├── service/
│   ├── repository/
│   ├── model/
│   └── util/
├── web/
│   ├── src/
│   │   ├── api/
│   │   ├── components/
│   │   ├── views/
│   │   ├── stores/
│   │   ├── utils/
│   │   └── worker/
│   └── vite.config.js
├── configs/
│   └── config.yaml
├── embed.go
├── go.mod
└── Makefile
```

---

## 9. 配置设计

```yaml
# config.yaml
server:
  host: "0.0.0.0"
  port: 8080

database:
  type: "sqlite"
  path: "./data/cloudbox.db"

storage:
  root: "./data/files"
  temp: "./data/temp"
  thumbnails: "./data/thumbnails"

upload:
  chunk_size: 5242880        # 5MB
  max_concurrent: 3
  temp_expire: 24h

jwt:
  secret: ""                 # 留空从环境变量读取
  expire: 24h

log:
  level: "info"
  file: "./data/logs/cloudbox.log"

admin:
  username: "admin"
  password: "admin123"       # 首次登录强制修改
```

---

## 10. 前端设计

### 10.1 页面结构

- 登录页
- 主页面（文件管理）
- 回收站页面
- 修改密码页面

### 10.2 核心组件

- FileList：文件列表
- Breadcrumb：面包屑导航
- UploadPanel：上传面板
- ConfirmDialog：确认弹窗
- ImageViewer：图片预览
- Thumbnail：缩略图

### 10.3 上传队列

- Web Worker 计算 MD5
- 并发池控制（max=3）
- 分片失败自动重试
- 进度实时显示

---

## 11. CLI 工具设计

### 11.1 命令列表

```bash
cloudbox login       # 登录
cloudbox logout      # 退出
cloudbox passwd      # 修改密码
cloudbox ls [path]   # 列出文件
cloudbox upload <local> [remote]  # 上传
cloudbox download <remote> [local] # 下载
cloudbox rm <path>   # 删除
cloudbox mkdir <path> # 创建文件夹
```

### 11.2 配置存储

- 路径：`~/.cloudbox/config.json`
- 权限：`0600`
- 存储：服务器地址、Token

---

## 12. 部署

### 12.1 构建命令

```bash
# 完整构建
make build

# 仅构建后端
make build-server

# 仅构建前端
make build-frontend

# 开发模式
make dev
```

### 12.2 运行

```bash
# Windows
cloudbox.exe

# Linux/macOS
./cloudbox
```

### 12.3 首次运行

1. 自动创建数据目录
2. 自动初始化数据库
3. 创建默认管理员账号
4. 浏览器访问 `http://IP:8080`
5. 使用默认密码登录
6. 强制修改密码

---

## 13. 扩展预留

### 13.1 多用户支持

- Repository 接口已抽象
- `owner_id` 字段已预留
- 角色权限字段已预留

### 13.2 MySQL 支持

- 配置文件预留 `database.type`
- Repository 接口可切换实现

### 13.3 文件搜索

- 可增加全文索引
- 可集成 Elasticsearch

---

## 14. 技术风险与对策

| 风险 | 对策 |
|------|------|
| 大文件上传超时 | 分片上传、断点续传 |
| 并发引用计数竞态 | 原子 SQL 更新 |
| 磁盘空间不足 | 配额限制（预留） |
| 临时文件堆积 | 定时清理任务 |
| 秒传哈希冲突 | MD5 + 文件大小双重校验 |
