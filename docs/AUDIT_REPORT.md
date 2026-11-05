# CloudBox 全面审查报告

> 审查日期：2026-07-25
> 审查范围：Go 后端、Vue 3 前端、配置、WebDAV、AI/RAG、部署
> 审查方法：源码逐行审计 + 业务逻辑交叉验证

---

## 一、总览

| 严重程度 | 后端 | 前端 | 合计 |
|---------|------|------|------|
| **严重 (CRITICAL)** | 7 | 3 | **10** |
| **高 (HIGH)** | 13 | 8 | **21** |
| **中 (MEDIUM)** | 19 | 12 | **31** |
| **低 (LOW)** | 21 | 11 | **32** |
| **缺失功能** | - | 6 | **6** |

---

## 二、严重问题（CRITICAL）— 必须立即修复

### C-1. API 密钥泄露至 Git 历史
- **文件：** `configs/config.yaml`（commit `ac4dca3` 已移除跟踪，但仍在 Git 历史中可恢复）
- **问题：** DeepSeek 密钥 `sk-28110b6d...` 和阿里云 DashScope 密钥 `sk-58c33c84...` 已提交至版本控制。任何有权访问仓库的人均可提取并滥用这些密钥。
- **修复：** 立即轮换两个 API 密钥；使用 `git filter-repo` 清除历史；仅通过环境变量加载。

### C-2. JWT 签名密钥为已知占位符
- **文件：** `configs/config.yaml.example:24`, `internal/config/config.go:257-263`
- **问题：** `jwt.secret` 默认值为 `"change-this-in-production"`，若运维直接复制示例配置，攻击者可伪造任意用户（含 admin）的 JWT Token，完全绕过认证。
- **修复：** 启动时检测弱密钥，拒绝以已知占位符启动；强制使用 64+ 字节随机密钥。

### C-3. 默认管理员密码 `admin123` 且未服务端强制修改
- **文件：** `configs/config.yaml.example:30`, `internal/service/auth.go:118-122`, `internal/handler/middleware.go`
- **问题：** 默认管理员密码为 `admin123`。虽然登录响应设置了 `RequirePasswordChange: true`，但这仅前端提示——JWT 中不含此标记，中间件不检查，攻击者可直接使用默认凭证操作所有 admin API。
- **修复：** JWT 中加入 `pwd_changed` claim，中间件拒绝未修改密码的请求。

### C-4. 文件删除后分享链接未失效
- **文件：** `internal/service/file.go:159-179`（MoveToTrash）, `282-357`（PermanentDelete）
- **问题：** 文件删除（软删/硬删）均不撤销或清除关联的 `file_shares` 记录。软删后分享记录仍存在，文件恢复后分享自动重新生效；硬删后分享成为永久孤儿记录。
- **修复：** PermanentDelete 级联撤销所有关联分享；MoveToTrash 标记分享暂停。

### C-5. `isValidUTF8` 函数实现错误
- **文件：** `internal/service/ai.go:415-425`
- **问题：** 自定义 UTF-8 检测逻辑不正确，会接受超长编码（如 `0xC0 0x80`）和无效序列，导致非 UTF-8 编码文件（如 GBK/Shift-JIS）被误判，跳过 fallback 解码，发送给 LLM 的文本乱码。
- **修复：** 替换为标准库 `utf8.Valid(data)`。

### C-6. RAG 服务 Chat 客户端可能 nil 崩溃
- **文件：** `internal/service/rag.go:94-102, 510-511`
- **问题：** 若 `chatModel` 或 `chatBaseURL` 为空，`chatCli` 为 nil，但 `NewRAGService` 仍返回非 nil 对象。调用 `Ask` 时 `s.chatCli.ChatCompletionStream` 触发 nil 指针 panic。
- **修复：** `NewRAGService` 应检测必要配置，缺失时返回 error。

### C-7. 管理员配额更新完全不可用（前端）
- **文件：** `web/src/views/AdminUsersView.vue:266`
- **问题：** 使用原生 `axios.put()` 而非配置了 `Authorization` 拦截器的 `api` 实例，请求永远返回 401。磁盘配额管理功能完全失效。
- **修复：** 改用 `api` 实例或新增 `adminAPI.updateQuota()` 方法。

### C-8. ConfirmDialog 确认按钮永久卡死（前端）
- **文件：** `web/src/components/Dialogs/ConfirmDialog.vue:74-77`
- **问题：** `handleConfirm` 设置 `loading = true` 后 emit 事件，但永远不重置。对话框复用时确认按钮永久显示加载状态。
- **修复：** 由父组件通过 prop 控制 loading，或对话框关闭时自动重置。

### C-9. 批量删除提示"此操作不可恢复"但实际移至回收站（前端）
- **文件：** `web/src/views/FilesView.vue:334`
- **问题：** 批量删除提示文案为"此操作不可恢复"，但 `deleteFile()` 实际调用软删除（移至回收站）。信息误导用户。
- **修复：** 改为"删除后将移至回收站"。

### C-10. 上传 Worker 崩溃后重试空指针（前端）
- **文件：** `web/src/stores/upload.js:253`
- **问题：** `addTask()` 检查了 `if (this.worker)` 并有主线程降级方案，但 `retryUpload()` 未做检查。Worker 初始化失败后重试上传触发 `Cannot read properties of null`。
- **修复：** 添加与 `addTask()` 相同的 Worker 可用性检查和主线程降级。

---

## 三、高优先级问题（HIGH）

### H-1. 无 JWT 令牌撤销机制
- **文件：** `internal/handler/auth.go:155-158`（Logout 为空操作）
- **问题：** JWT 在 24 小时内有效，不受登出、禁用用户、密码修改影响。被盗 Token 在被禁账户后仍可使用一整天。
- **修复：** 实现 Token 黑名单，或在用户模型添加 `token_version` 列，密码修改/禁用时递增并在验证时检查。

### H-2. 角色/状态变更不反映在现有 JWT 中
- **文件：** `internal/util/crypto/jwt.go:22-28`
- **问题：** 管理员降级另一管理员后，被降级者的现有 JWT 仍携带 `role: "admin"` 直到过期（最长 24h）。被禁用的用户同理。
- **修复：** 与 H-1 的 token_version 方案一并解决。

### H-3. CORS 允许所有来源（`*`）
- **文件：** `cmd/server/main.go:100`
- **问题：** `Access-Control-Allow-Origin: *` 允许任意网站向 CloudBox API 发起跨域请求。结合 JWT-in-URL 回退，扩大 CSRF 和数据泄露风险。
- **修复：** 配置允许的域名列表，仅反射已知来源。

### H-4. JWT Token 在 URL 查询参数中传递
- **文件：** `internal/handler/middleware.go:26-28`（接受 `c.Query("token")`）, `web/src/utils/api.js`
- **问题：** Token 出现在服务器日志、浏览器历史、Referer 头、代理日志中。24 小时有效的凭据暴露于多处。
- **修复：** 移除 query parameter 回退；SSE 场景使用短期单次 Token。

### H-5. 缺失安全响应头
- **文件：** `cmd/server/main.go:94-115`
- **问题：** 未设置 `X-Content-Type-Options`、`X-Frame-Options`、`Content-Security-Policy`、`Strict-Transport-Security`、`Referrer-Policy`。可被嵌入 iframe（点击劫持）、MIME 嗅探、HTTP 降级攻击。
- **修复：** 添加安全头中间件。

### H-6. WebDAV PUT 全量缓冲到内存（DoS 风险）
- **文件：** `internal/handler/webdav.go:447-485`
- **问题：** `cloudWriteFile` 将 PUT body 全部 append 到 `[]byte`。恶意或大文件上传可耗尽 RAM 导致崩溃。
- **修复：** 流式写入临时文件，或设置最大文件大小限制。

### H-7. 上传文件名未做安全过滤
- **文件：** `internal/service/upload.go:125-128, 442`
- **问题：** 仅检查 `FileName == ""`。攻击者可上传含 `../`、null 字节、控制字符的文件名。这些名称传播到 Zip 下载（Zip Slip 风险）、WebDAV 列表、审计日志。
- **修复：** 拒绝含路径分隔符、null 字节、`.`/`..` 的文件名；应用 `filepath.Base()` 净化。

### H-8. 管理员可无限提权
- **文件：** `internal/service/admin.go:118-135`
- **问题：** 任何管理员可将任意用户设为 `"admin"`，角色值未做白名单校验。结合 C-3 默认管理员密码，攻击者可创建无限管理员账户。
- **修复：** 验证 role 值白名单（`"admin"`/`"user"`）；考虑限制管理员创建。

### H-9. WebDAV BasicAuth 无暴力破解防护
- **文件：** `internal/handler/webdav_auth.go:62-70`
- **问题：** 登录失败仅审计日志记录，无限速、无账户锁定、无指数退避。校园网络中凭证可被暴力破解。
- **修复：** WebDAV 端点添加 rate limiting。

### H-10. 前端登出未调用服务端（前端）
- **文件：** `web/src/stores/auth.js:32-37`
- **问题：** `logout()` 仅清除本地 Token，未调用 `authAPI.logout()`。JWT 在服务端仍有效直到过期。
- **修复：** 改为 async 方法，先调用服务端登出再清除本地状态。

### H-11. 移动文件对话框只显示当前目录文件夹（前端）
- **文件：** `web/src/components/Dialogs/MoveDialog.vue:71-85`
- **问题：** 文件夹树从 `filesStore.files`（当前目录内容）构建，无法看到其他目录分支。用户在 `/documents/reports` 无法将文件移动到 `/documents/invoices`。
- **修复：** 打开对话框时从后端获取完整文件夹树（需新增 API `GET /api/folders/tree`）。

### H-12. 侧边栏存储用量不随操作刷新（前端）
- **文件：** `web/src/components/Layout/AppSidebar.vue:93-98`
- **问题：** 存储用量仅 `onMounted` 获取一次。上传、删除、移动文件后显示的用量（如"2.3 GB / 10 GB"）始终过时。
- **修复：** 文件操作完成后调用 `storageAPI.getUsage()` 刷新。

### H-13. 修改密码对话框无最小长度校验（前端）
- **文件：** `web/src/components/Layout/AppHeader.vue:82-89`
- **问题：** `newPassword` 规则缺少 `{ min: 6 }` 校验，用户可将密码改为 1 个字符。注册页和强制修改密码页均有此校验。
- **修复：** 添加 `{ min: 6, message: '密码至少6位', trigger: 'blur' }` 规则。

---

## 四、中等优先级问题（MEDIUM）

### 后端

| # | 问题 | 文件 |
|---|------|------|
| M-1 | 密码最低 6 位，无复杂度要求 | `handler/auth.go:27` |
| M-2 | 用户名可通过注册接口枚举 | `handler/auth.go:98-105` |
| M-3 | 限速 100次/分钟过于宽松 | `cmd/server/main.go:154` |
| M-4 | 限速仅按 IP，不按账户 | `middleware/ratelimit.go:81-93` |
| M-5 | WebDAV auth 日志记录部分 Base64 密码 | `webdav_auth.go:26-29` |
| M-6 | 注册存在 TOCTOU 竞态条件 | `service/auth.go:67-90` |
| M-7 | 审计日志分页无上限 | `handler/admin.go:184-185` |
| M-8 | HTTP 状态码始终返回 200 | `util/response/json.go:23-28` |
| M-9 | 分享 Token 仅 8 字符（~38 位熵）+ 无限速 | `service/share.go:271-290` |
| M-10 | 审计日志 JSON 注入（未转义文件名） | `service/upload.go:149`, `file.go:154` |
| M-11 | 剪贴板 `DeleteOldestUnpinned` SQL 在 MySQL 报错 1093 | `repository/clipboard.go:76-83` |
| M-12 | AI prompt 截断按字节切割 UTF-8 | `service/ai.go:203-206` |
| M-13 | 标签解析仅分割 ASCII 逗号，不支持中文顿号 | `service/ai.go:302` |
| M-14 | RAG 检索加载全部 chunks 到内存 | `service/rag.go:525-550` |
| M-15 | RAG `buildMessages` 无 Token 预算控制 | `service/rag.go:597-633` |
| M-16 | WebDAV `Readdir` 不返回 `io.EOF` | `handler/webdav.go:405-420` |
| M-17 | WebDAV MOVE 到已存在文件夹行为非标准 | `handler/webdav.go:169-179` |
| M-18 | Chat Handler 大量代码重复（Ask/AskSSE ~120 行） | `handler/chat.go:294-414, 457-552` |
| M-19 | Chat Handler 直接访问 `repository.DB`（违反分层架构） | `handler/chat.go:120, 150` |

### 前端

| # | 问题 | 文件 |
|---|------|------|
| M-1 | ChatView `isMobile` 不响应窗口大小变化 | `ChatView.vue:153` |
| M-2 | Toolbar `viewMode` ref 与 store 不同步 | `Toolbar.vue:71` |
| M-3 | TrashView `onBeforeRouteUpdate` 冗余 | `TrashView.vue:114-117` |
| M-4 | 右键菜单超出视口（桌面端未做边界约束） | `FileGrid.vue:83-86`, `FileList.vue:100-103` |
| M-5 | 移动端选中文件时隐藏 TabBar，阻断导航 | `MobileTabBar.vue:62-63` |
| M-6 | 切换文件夹时搜索范围被静默重置 | `SearchBar.vue:66-69` |
| M-7 | 文件列表单击和双击处理重复 | `FileList.vue:166-180` |
| M-8 | 聊天输入框不支持 Shift+Enter 换行 | `ChatView.vue:92` |
| M-9 | 上传冲突对话框描述"重命名"选项但未实现 | `upload.js:141-159` |
| M-10 | 文件夹/批量下载将 ZIP 全量加载到浏览器内存 | `stores/files.js:148-185` |
| M-11 | 聊天错误消息在 UI 中显示两次 | `ChatView.vue:60-68` |
| M-12 | 路由缺少 404 兜底页面 | `router/index.js` |

---

## 五、低优先级问题（LOW）

### 后端（21 项）

| # | 问题 | 文件 |
|---|------|------|
| L-1 | Swagger UI 无需认证即可访问 | `cmd/server/main.go:273` |
| L-2 | 用户 ID 自增，可枚举 | `model/user.go:12` |
| L-3 | Context value 使用字符串 key（有冲突风险） | `handler/middleware.go:48-50` |
| L-4 | 注册响应始终说"等待审批"（即使无需审批） | `handler/auth.go:111` |
| L-5 | `UpdateUserQuota` 向客户端泄露原始错误信息 | `handler/admin.go:173` |
| L-6 | 管理员账户检查使用 `==`（非恒定时间比较） | `service/auth.go:120` |
| L-7 | 审计通道满时静默丢弃事件 | `service/audit.go` |
| L-8 | 仅 auth 路由有限速 | `cmd/server/main.go:154` |
| L-9 | 内存限速器不支持多实例 | `middleware/ratelimit.go` |
| L-10 | WebDAV Dead Properties 仅存内存（重启丢失） | `handler/webdav.go:34` |
| L-11 | WebDAV `virtualRootInfo.ModTime()` 返回零值 | `handler/webdav.go:244` |
| L-12 | HMAC 密钥未配置时每次重启分享凭证失效 | `config/config.go:265-267` |
| L-13 | 分享密码无复杂度要求 | `service/share.go:86-92` |
| L-14 | 文件夹下载计数在流式传输失败前已递增 | `service/share.go:187-193` |
| L-15 | `strconv.ParseInt` 无效输入静默返回 0 | `handler/share.go:195` |
| L-16 | 剪贴板 `len(string)` 按字节而非字符计算 | `service/clipboard.go:41` |
| L-17 | 剪贴板批量清除无审计日志 | `service/clipboard.go:98-103` |
| L-18 | Vision 客户端每次请求创建新 HTTP Client | `service/ai.go:195-200` |
| L-19 | DOCX 文本提取包含所有 XML 标签噪声 | `service/ai.go:363-369` |
| L-20 | AI Client 仅 2 次重试，无指数退避，不重试 429 | `service/ai_client.go:100-122` |
| L-21 | 配置文件权限未强制检查 | `config/config.go:247` |

### 前端（11 项）

| # | 问题 | 文件 |
|---|------|------|
| L-1 | TrashView 未使用的 `useRoute` 导入 | `TrashView.vue:95` |
| L-2 | ClipboardView 未使用的 `computed`、`useAuthStore` | `ClipboardView.vue:99,101-102` |
| L-3 | ClipboardView 未使用的 `getDeviceNameKey()` | `ClipboardView.vue:118-120` |
| L-4 | ImagePreview 空的 `handleImageLoad` 函数 | `ImagePreview.vue:137-139` |
| L-5 | `main.scss` 中大量未使用的 CSS 工具类 | `styles/main.scss:44-79` |
| L-6 | `main.scss` 拖拽覆盖样式与 `FilesView.vue` 重复 | `styles/main.scss:82-105` |
| L-7 | `authAPI.logout` 定义但从未调用 | `utils/api.js:45` |
| L-8 | `formatBytes`/`formatSize` 4 处重复实现 | 多处 |
| L-9 | `formatDate` 至少 6 处不同实现 | 多处 |
| L-10 | SharesView 未使用的 `Folder` 图标 | `SharesView.vue:17` |
| L-11 | API 响应拦截器未解包标准信封结构 | `utils/api.js:26-37` |

---

## 六、缺失功能

| # | 功能 | 说明 |
|---|------|------|
| F-1 | 文本/PDF 预览 | 前端仅支持图片预览，PDF/TXT/DOCX 均触发下载。后端已有 `previewAPI` 支持 |
| F-2 | AI 摘要/标签 UI | 后端有完整 AI 摘要和标签存储，前端无任何展示/触发界面 |
| F-3 | 键盘快捷键 | 无 Delete 删除、F2 重命名、Ctrl+A 全选、Esc 取消选择 |
| F-4 | Shift 范围选择 | 仅支持切换选择，不支持 Shift+Click 范围选择 |
| F-5 | 文件列表分页 | 文件夹内容一次性全部加载，大量文件时性能和内存问题 |
| F-6 | 管理后台系统设置页 | 配额默认值、AI 配置、注册设置均只能通过 config.yaml 修改 |

---

## 七、积极发现（做得好的地方）

- **密码哈希**：使用 bcrypt DefaultCost（`password.go:6`）
- **SQL 安全**：所有数据库查询使用 GORM 参数化，未发现 SQL 注入
- **文件所有权隔离**：全程使用 `FindByIDAndOwner`（`file.go`）
- **分享凭证验证**：使用 HMAC-SHA256 + 恒定时间比较（`share.go:342`）
- **WebDAV CORS 隔离**：正确区分 WebDAV 和 API 路径（`main.go:96-99`）
- **WebDAV 路径遍历防护**：虚拟文件系统通过数据库逐段解析
- **物理文件存储**：使用数据库 ID 而非用户提供的文件名
- **TLS 私钥权限**：0600 权限写入（`generate.go:200`）
- **管理员自保护**：阻止自删除和自降级（`admin.go:119, 163`）
- **循环引用检测**：移动文件夹时检测（`file.go:205-211`）
- **前端 API 覆盖率**：100%——后端每个路由都有前端调用
- **分块上传**：Worker 线程 MD5 计算，主线程降级方案
- **SSE 流式聊天**：正确的流式传输和错误处理

---

## 八、优先修复路线

### 第一阶段：安全基线（1-2 天）
1. 轮换泄露的 API 密钥，清除 Git 历史
2. 替换 JWT 密钥，添加弱密钥启动检查
3. JWT 中添加 `token_version` + `pwd_changed` claim
4. 移除 query parameter Token 回退
5. 添加安全响应头中间件
6. 修复 C-7（管理员配额 API 调用）

### 第二阶段：核心业务修复（3-5 天）
7. 文件删除级联撤销分享
8. 修复 `isValidUTF8`
9. RAG nil 指针防护
10. 上传文件名安全过滤
11. 修复 MySQL 剪贴板 SQL
12. 修复 ConfirmDialog loading 卡死
13. 修复移动文件夹对话框（完整树）
14. WebDAV Readdir io.EOF

### 第三阶段：交互完善（5-7 天）
15. 添加 404 页面
16. 聊天自动滚动优化
17. Shift+Enter 换行
18. 存储用量刷新
19. 右键菜单边界约束
20. 统一 formatBytes/formatDate
21. AI 摘要/标签 UI

### 第四阶段：体验增强（长期）
22. 文本/PDF 预览
23. 键盘快捷键
24. 文件列表分页
25. 管理系统设置页
26. Token 撤销机制完善
