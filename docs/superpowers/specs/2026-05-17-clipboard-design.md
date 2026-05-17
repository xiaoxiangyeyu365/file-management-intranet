# 云剪切板功能设计规格

> **目标：** 实现跨设备文本剪切板同步功能

## 功能概述

云剪切板允许用户在不同设备间同步文本内容。一台设备粘贴内容并保存后，其他登录同一账号的设备可以立即看到并取用。

## 数据模型

### 数据库表：clipboard_records

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER | 主键，自增 |
| content | TEXT | 剪切板内容（纯文本） |
| device_name | TEXT | 来源设备名称，默认 "未命名设备" |
| user_id | INTEGER | 所有者 ID（外键） |
| pinned | BOOLEAN | 是否置顶，默认 false |
| created_at | DATETIME | 创建时间 |

- `device_name` 字段默认值为 "未命名设备"，避免 NULL 值
- `content` 字段在 Go 层校验，最大 10KB (10240 字节)

### 索引

- `(user_id, pinned, created_at)` - 查询用户记录时使用

## API 设计

### 获取记录列表

```
GET /api/clipboard
Authorization: Bearer <token>

Response:
{
  "code": 0,
  "data": {
    "records": [
      {
        "id": 1,
        "content": "Hello World",
        "deviceName": "办公室电脑",
        "pinned": false,
        "createdAt": "2026-05-17T21:00:00+08:00"
      }
    ]
  }
}
```

- 返回当前用户的所有记录，最多 50 条
- 置顶记录优先显示在前
- 服务端只返回 `createdAt`，不计算 `isNew`
- `isNew` 由前端判断：(Date.now() - new Date(record.createdAt).getTime()) < 5000

### 创建记录

```
POST /api/clipboard
Authorization: Bearer <token>
Content-Type: application/json

{
  "content": "要保存的文本内容"
}

Response:
{
  "code": 0,
  "data": {
    "id": 1,
    "content": "要保存的文本内容",
    "deviceName": "办公室电脑",
    "pinned": false,
    "createdAt": "2026-05-17T21:00:00+08:00"
  }
}
```

- 自动记录设备名称（首次需要用户输入，之后记住在 localStorage）
- 在同一事务内：先插入新记录，再检查总数是否超过 50，若超限则删除最早的非置顶记录

### 置顶/取消置顶

```
PATCH /api/clipboard/:id/pin
Authorization: Bearer <token>
Content-Type: application/json

{
  "pinned": true
}

Response:
{
  "code": 0
}
```

### 删除单条

```
DELETE /api/clipboard/:id
Authorization: Bearer <token>

Response:
{
  "code": 0
}
```

### 清空全部

```
DELETE /api/clipboard?onlyUnpinned=true
Authorization: Bearer <token>

Response:
{
  "code": 0
}
```

- `onlyUnpinned=true` 时：仅删除非置顶记录（默认）
- `onlyUnpinned=false` 或不传：删除所有记录（包括置顶）

## 前端设计

### 路由

- `/clipboard` - 云剪切板页面（需要登录）

### 侧边栏入口

在 AppSidebar 中新增：
```vue
<router-link to="/clipboard" class="nav-item">
  <el-icon><Document /></el-icon>
  <span>云剪切板</span>
</router-link>
```

### ClipboardView.vue 页面布局

```
┌─────────────────────────────────────────────────┐
│ AppHeader                                        │
├─────────┬───────────────────────────────────────┤
│         │ 云剪切板                    [清空]    │
│         ├───────────────────────────────────────┤
│ 侧边栏   │ ┌─────────────────────────────────┐  │
│         │ │  文本输入框                    │  │
│ 全部文件 │ │                               │  │
│ 回收站   │ │  [保存]                       │  │
│ 云剪切板 │ └─────────────────────────────────┘  │
│         │                                       │
│         │ 设备名称：__________________ [保存]   │
│         ├───────────────────────────────────────┤
│         │ 📌 置顶内容                    [取消] │
│         │    内容预览...                  │
│         │    来自: 办公室电脑  2小时前         │
│         │                                       │
│         │ ✨ NEW 内容2                    [置顶] │
│         │    内容预览...                  │
│         │    来自: 笔记本  5分钟前           │
│         │                                       │
│         │ 内容3                            [置顶] │
│         │    内容预览...                  │
│         │    来自: 手机  昨天               │
└─────────┴───────────────────────────────────────┘
```

### 交互逻辑

1. **保存内容**
   - 用户在文本框输入内容，点击保存
   - 首次保存时弹出设备名称设置对话框
   - 保存成功后显示成功提示，内容出现在列表顶部
   - 输入框下方显示剩余字符数（10240 上限）

2. **查看历史**
   - 列表按时间倒序排列，置顶记录始终在最前
   - 前端判断：createdAt 距今 5 秒内显示 NEW 标签

3. **复制内容**
   - 点击记录自动复制到浏览器剪贴板
   - 显示"已复制到剪贴板"提示

4. **置顶/取消**
   - 点击置顶按钮切换状态
   - 置顶记录不受自动清理影响

5. **删除**
   - 单条删除：点击删除按钮，确认后删除
   - 清空全部：点击清空按钮，确认后删除非置顶记录，提示"置顶记录将保留"

6. **自动同步**
   - 使用 `setInterval` 每 5 秒轮询 `/api/clipboard`
   - 轮询在组件 `onUnmounted` 时清除
   - 若用户正在输入框中编辑，不刷新列表（避免页面跳动）
   - 新记录追加到列表顶部，不重置滚动位置

## 设备名称管理

- 存储在 localStorage，key 格式：`cloudbox_device_name_{userID}`
- 首次创建记录时检查是否存在，不存在则弹出设置对话框
- 切换账号时自动重置为新账号的设备名称
- 用户可以在页面设置区域修改设备名称

## 自动清理规则

1. 用户最多保留 50 条记录
2. 在创建新记录的同一事务内检查并清理
3. 超限时删除最早创建的非置顶记录
4. 置顶记录永远不会被自动删除

## 错误处理

| 场景 | 前端提示 | 后端处理 |
|------|----------|----------|
| 内容为空 | "请输入内容" | 返回 400 |
| 内容超 10KB | "内容过长（最多 10240 字符）" | 返回 400 |
| 网络错误 | 显示重试按钮 | 返回 500 |
| 未设置设备名 | 弹出设备名称设置对话框 | - |