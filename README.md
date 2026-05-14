

# CloudBox 内网云盘系统

CloudBox 是一款用 Go 语言开发的内网云盘系统，支持文件上传、下载、预览、搜索以及回收站管理等核心云存储功能。

## 功能特性

### 文件管理
- **文件浏览**：按文件夹层级查看和管理文件
- **文件夹创建**：支持创建多层级文件夹
- **文件重命名**：灵活的文件/文件夹重命名
- **文件移动**：支持批量移动文件到指定文件夹
- **文件删除**：软删除至回收站

### 文件上传
- **分片上传**：支持大文件分片断点续传
- **MD5 校验**：上传时进行 MD5 校验，支持秒传功能
- **进度追踪**：实时获取上传进度

### 文件下载
- **单文件下载**：直接下载文件，支持中文文件名
- **文件夹打包下载**：将整个文件夹打包为 ZIP 下载
- **缩略图获取**：获取图片文件的缩略图

### 回收站
- **查看回收站**：列出所有已删除文件
- **恢复文件**：从回收站恢复文件
- **永久删除**：彻底删除文件，释放存储空间
- **清空回收站**：一键清空所有回收站文件

### 搜索与预览
- **文件搜索**：按文件名关键词搜索，支持递归搜索子文件夹
- **图片预览**：获取图片 EXIF 元数据（分辨率、拍摄信息等）

### 用户认证
- **JWT 认证**：基于 Token 的安全认证
- **密码管理**：支持修改密码

## 技术栈

| 分类 | 技术 |
|------|------|
| 后端框架 | Go + Gin |
| 数据库 | SQLite + GORM |
| 认证 | JWT |
| 存储 | 本地文件系统 |

## 快速开始

### 环境要求

- Go 1.20+
- SQLite3

### 构建运行

```bash
# 完整构建（前端 + 后端）
make build

# 仅构建后端
make build-backend

# 仅构建前端
make build-frontend

# 开发模式运行
make run
```

### 配置说明

配置文件位于 `configs/config.yaml`，主要配置项：

```yaml
server:
  port: 8080        # 服务端口
  mode: debug      # 运行模式

database:
  path: ./data.db   # 数据库路径

storage:
  root: ./storage  # 存储根目录
  temp: ./temp     # 临时文件目录

jwt:
  secret: ""       # JWT 密钥（自动生成）
  expiry: 168h     # Token 有效期

admin:
  username: admin # 管理员用户名
  password: admin  # 管理员默认密码
```

首次运行会自动创建默认管理员账号，登录后建议立即修改密码。

## API 接口

### 认证接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/auth/login | 用户登录 |
| POST | /api/auth/change-password | 修改密码 |
| POST | /api/auth/logout | 用户登出 |
| GET | /api/auth/profile | 获取用户信息 |

### 文件接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/files | 获取文件列表 |
| GET | /api/files/:id | 获取文件详情 |
| POST | /api/files | 创建文件夹 |
| PUT | /api/files/:id/rename | 重命名 |
| PUT | /api/files/:id/move | 移动文件 |
| DELETE | /api/files/:id | 删除文件 |
| GET | /api/files/:id/download | 下载文件 |
| GET | /api/files/:id/thumbnail | 获取缩略图 |
| GET | /api/files/search | 搜索文件 |
| GET | /api/folders/:id/download | 文件夹打包下载 |

### 上传接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/upload/init | 初始化上传 |
| POST | /api/upload/chunk | 上传分片 |
| GET | /api/upload/progress | 获取上传进度 |
| POST | /api/upload/complete | 完成上传 |
| DELETE | /api/upload/cancel | 取消上传 |

### 回收站接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/trash | 查看回收站 |
| POST | /api/trash/:id/restore | 恢复文件 |
| DELETE | /api/trash/:id | 永久删除 |
| DELETE | /api/trash | 清空回收站 |

## 项目结构

```
.
├── cmd/server/           # 程序入口
├── configs/             # 配置文件
├── internal/
│   ├── config/          # 配置加载
│   ├── handler/        # HTTP 处理器
│   ├── model/          # 数据模型
│   ├── repository/     # 数据访问层
│   ├── service/        # 业务逻辑层
│   └── util/           # 工具函数
│       ├── crypto/     # 加密相关
│       ├── response/   # 响应封装
│       └── storage/    # 存储管理
└── Makefile            # 构建脚本
```

## 许可证

MIT License