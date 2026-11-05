# RAG 文档问答设计规格

## 概述

为 CloudBox 添加 RAG（Retrieval-Augmented Generation）文档问答功能。用户上传文档后，可对文件内容进行对话式提问，AI 基于文件内容检索并生成回答。支持单文件问答和多文件跨文档问答，流式输出，对话持久化。

## 数据模型

### 新增表：`document_chunks`

| 列 | 类型 | 说明 |
|---|---|---|
| id | BIGINT PK AUTO_INCREMENT | 自增主键 |
| physical_file_id | BIGINT NOT NULL | 关联 physical_files（去重，同内容只存一份） |
| chunk_index | INT NOT NULL | 分块序号（0-based） |
| content | TEXT NOT NULL | 分块文本 |
| embedding | BLOB | little-endian float64 原始字节数组 |
| token_count | INT | 该块 token 数（用于成本监控） |
| created_at | DATETIME NOT NULL | 创建时间 |

索引：`idx_chunks_physical` ON (physical_file_id, chunk_index)

**embedding 编码规则：**
- 存储：`math.Float64bits(f) → binary.LittleEndian.PutUint64` 逐元素写入
- 读取：按 8 字节一组 `binary.LittleEndian.Uint64 → math.Float64frombits` 解码
- 空间 = 维度 × 8 字节（如 1024 维 = 8192 字节）

### 新增表：`conversations`

| 列 | 类型 | 说明 |
|---|---|---|
| id | BIGINT PK AUTO_INCREMENT | 自增主键 |
| user_id | BIGINT NOT NULL | 所属用户 |
| title | VARCHAR(200) | 对话标题（首条问题时用问题前 30 字更新） |
| file_ids | JSON | 关联文件 ID 列表（MVP 用 JSON，后续按需抽关联表） |
| created_at | DATETIME NOT NULL | 创建时间 |
| updated_at | DATETIME NOT NULL | 更新时间 |

索引：`idx_conversations_user` ON (user_id, updated_at DESC)

### 新增表：`messages`

| 列 | 类型 | 说明 |
|---|---|---|
| id | BIGINT PK AUTO_INCREMENT | 自增主键 |
| conversation_id | BIGINT NOT NULL | 关联会话 |
| role | VARCHAR(20) NOT NULL | user / assistant（system 不持久化，动态构建） |
| content | TEXT NOT NULL | 消息内容 |
| sources | JSON | 引用来源，格式：`[{"chunk_id":123,"file_id":456,"file_name":"合同.pdf","preview":"这是被引用的文本..."}]` |
| created_at | DATETIME NOT NULL | 创建时间 |

索引：`idx_messages_conversation` ON (conversation_id, created_at)

**关键设计决策：**
- `document_chunks` 关联 `physical_file_id` 而非 `file_id`，复用去重逻辑
- `messages` 只存 user 和 assistant 角色；system（检索上下文）动态构建，不持久化
- `sources` 含 `file_name`，前端展示引用时免额外请求；注意文件名可能被重命名，展示用 `sources.file_name`，点击链接以 `file_id` 获取最新信息
- `conversations.file_ids` 用 JSON 数组，MVP 阶段够用；后续如需按文件查对话可抽关联表

### `physical_files` 表新增列

| 列 | 类型 | 说明 |
|---|---|---|
| chunk_count | INT DEFAULT 0 | 0=未分块，>0=已分块 chunk 数 |

## RAG 管道（提取 → 分块 → Embedding → 存储）

### 触发时机

文件上传完成后异步执行（`go func()` + semaphore 模式，同现有 AIService.ProcessFile），与 AI 摘要并行，互不阻塞。

### 管道步骤

1. **提取内容** — 复用 `AIService.extractContent()`，获得文本内容
2. **语义分块** — 按段落/章节边界分块：
   - PDF：按换行双空行分段落，段落内超 500 字按句号切分
   - 纯文本：按空行分段落，同上切分逻辑
   - 回退：无法按段落切分时，使用固定大小滑动窗口（max_size 字 + overlap 字重叠）
   - 规则：最小块 100 字，最大块 500 字，重叠 50 字
   - 不足 100 字的尾部合并到前一块
   - 滑动窗口边界：`i + maxSize` 不越界，最后一块从 `max(len(text)-maxSize, i)` 开始
   - 重叠块去重：前端展示引用时对相同句子去重
3. **生成 Embedding** — 调用远程 Embedding API：
   - `AIClient.Embed(ctx, texts []string) ([][]float64, error)`，独立封装，不假设完全 OpenAI 兼容
   - 批量发送：每批最多 `embedding_batch_size`（默认 20）条
   - 维度动态获取：首次从 API 响应读取实际维度，与配置 `embedding_dimensions` 对比校验，不一致则 warn + 用实际维度，缓存后续复用
   - 维度缓存线程安全：使用 `sync.Once` 确保首次获取后并发调用直接复用
   - 如果模型未返回维度字段，回退到配置值并 warn
4. **存储** — 事务写入：
   - 在同一个事务内：插入所有 chunks → 更新 `physical_files.chunk_count = len(chunks)`
   - 任一步失败则回滚，chunk_count 保持 0

### 去重

同 `physical_file_id` 已有 chunks 则跳过（`SELECT COUNT(*) FROM document_chunks WHERE physical_file_id=?`）。内网并发冲突概率低，MVP 不加锁。

### 失败处理

任一步失败则 log + 跳过，不阻塞上传。`chunk_count` 保持 0 表示未处理。

## 问答流程（检索 → 生成 → 流式输出）

1. **用户提问** → 获取 conversation 关联的 file_ids → 查 files 表获取 physical_id → JOIN physical_files WHERE chunk_count > 0 过滤未处理文件 → 得到有效 physical_file_ids
2. **Query Embedding** → 对用户问题调用 Embed API 获取向量
3. **向量检索** →
   - SQL 过滤：`physical_file_id IN (...)` JOIN `physical_files WHERE chunk_count > 0`，仅加载 `id, content, embedding`
   - 应用层逐条计算余弦相似度，取 top-K
   - 跳过未处理文件（chunk_count = 0）
4. **拼接 Prompt** →
   - system: `rag.system_prompt` + 检索到的 chunks 文本
   - user: 历史对话（最近 max_history 轮，不含 system）+ 当前问题
   - 字符数粗估：chunks + history 不超 `max_content_length`，超长则截断最早的历史轮
5. **流式生成** →
   - `AIClient.ChatCompletionStream(ctx, messages, callback)`，请求加 `"stream": true`
   - 读取 SSE `data: {...}` 行，每收到 chunk 调用 callback
   - Handler 用 Gin SSE 推送到前端
   - 客户端断开时通过 `c.Request.Context().Done()` + `context.WithCancel` 取消 LLM 请求
6. **持久化** →
   - 保存 user message + assistant message（含 sources）
   - 更新 `conversation.updated_at`
   - 如果是首条用户消息，更新 `conversation.title = question[:30]`
   - 持久化在接收完整 LLM 响应后统一写入 DB（原子化）
   - 流中断（客户端断开/网络异常）时 assistant 消息不持久化（内网场景崩溃概率极低，可接受）

### 历史对话管理

- 每次问答携带最近 `max_history` 轮（默认 10 条 user+assistant 对）
- system 角色消息动态构建，不纳入历史轮数，不写入 messages 表
- 超出轮数截断最早的 user+assistant 对
- 恢复历史时：查 `role IN ('user','assistant')` 按 `created_at DESC` 取 `max_history * 2` 条，反转排序

### 性能标注

- 向量检索在万级 chunk 内可接受（< 1s）
- 数万级以上需优化（sqlite-vec 或 HNSW 索引）
- SQL 仅加载 `id, content, embedding`，已有 `idx_chunks_physical` 复合索引

## API 设计

### 对话管理

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | /api/chat/conversations | 创建对话（可选 `file_ids`、`title`） |
| GET | /api/chat/conversations | 列出用户对话（`?limit=20&offset=0`，按 updated_at 降序） |
| GET | /api/chat/conversations/:id | 获取对话详情 + 消息列表 |
| DELETE | /api/chat/conversations/:id | 删除对话及消息（应用层级联） |
| POST | /api/chat/conversations/:id/add-file | 添加文件到对话（`{"file_id": 123}`，校验归属 + chunk_count > 0） |

### 问答

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | /api/chat/conversations/:id/ask | 发送问题，SSE 流式返回回答 |

**ask 请求体：**
```json
{ "question": "这个合同的甲方是谁？" }
```

**SSE 响应格式：**
```
event: token
data: {"content": "这是"}

event: token
data: {"content": "回答"}

event: done
data: {"messageId": 123, "sources": [...]}
```

**SSE 响应头：**
```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no
```

### 管理

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | /api/files/:id/reindex | 手动重新分块+embedding（返回 HTTP 202 `{ "status": "processing" }`，后台执行） |

## 前端交互

### 入口

- 文件详情页新增"AI 问答"按钮 → 创建单文件对话并跳转
- 侧边栏新增"AI 对话"入口 → 对话列表页

### 对话页面布局

- 左侧：对话列表（可新建、删除、切换）
- 右侧：消息区 + 底部输入框
- 顶部：关联文件标签（可添加/移除）

### 消息展示

- 用户消息：气泡右侧
- AI 消息：气泡左侧，流式逐字显示
- AI 消息下方"引用来源"折叠区：展示 sources 中的 preview 文本 + file_name 链接

### 技术方案

- Vue 3 + Element Plus
- SSE 用 `fetch` + `ReadableStream`（POST 请求需用此方式）
- Pinia store 管理对话状态和消息流

### 流式消息状态管理

- 发送问题后创建临时 assistant 消息，content 初始为空
- 随 token 事件逐步拼接 content
- done 后标记完成 + 填充 sources
- 中断/出错标记"回复中断"，允许重试

### 对话列表

- 后端返回已按 updated_at 降序
- 新消息后本地置顶或重新拉取

### 错误处理

- 网络错误、LLM 超时在消息区显示错误提示 + 重试按钮
- 重试复用当前对话，重新发送问题

### 删除确认

- 删除对话使用 Element Plus `ElMessageBox.confirm` 二次确认

## 配置

```yaml
rag:
  enabled: true
  embedding_base_url: "https://api.deepseek.com"
  embedding_api_key: ""                    # 空=复用 ai.api_key
  embedding_model: "deepseek-embedding"
  embedding_dimensions: 1024               # 校验用，实际从 API 响应获取并缓存
  chunk_min_size: 100
  chunk_max_size: 500
  chunk_overlap: 50
  embedding_batch_size: 20
  max_concurrent: 2                        # RAG 全局并发数（独立于 AI 摘要工作池）
  top_k: 5
  max_history: 10
  timeout: 30
  chat_model: ""                           # 空=复用 ai.model
  chat_base_url: ""                        # 空=复用 ai.base_url
  chat_api_key: ""                         # 空=复用 ai.api_key
  system_prompt: "你是一个文档助手。基于提供的上下文内容回答用户问题。如果上下文中没有相关信息，请明确说明。"
```

### 与现有 AI 的关系

- RAG 独立开关，不依赖 `ai.enabled`
- API key/model 可独立配置，空值时 fallback 到 `ai.*` 对应字段
- 分块管道复用 `extractContent()` 和 `detectMimeTypeFromPath()`
- Embedding 客户端独立于 AIClient（不同端点格式）
- Chat 客户端复用 AIClient 架构，新增 streaming + multi-turn 支持

### 并发提醒

RAG semaphore (`rag.max_concurrent`) 与 AI 摘要 semaphore (`ai.max_concurrent`) 独立。两者并行时总并发 LLM API 调用数可能达到两者之和。内网小规模使用通常无忧，但需注意 API 速率限制。

## 服务集成

- `NewRAGService(cfg.RAG, cfg.AI, physicalRepo, chunkRepo, convRepo, msgRepo, storage)`
- 上传完成后：`go ragService.ProcessFile(fileID, physicalID, mimeType)`，与 `aiService.ProcessFile` 并行
- Shutdown：`ragService.Shutdown()` 等待进行中任务
- `POST /api/files/:id/reindex` 复用 `ragService.ProcessFile`

### 依赖关系

- RAG enabled 但 AI disabled → 正常（摘要不生成，问答可用）
- RAG enabled 但 embedding API 不可达 → 分块成功但 embedding 失败，chunk_count=0，问答时提示"文件尚未索引"

### RAG 问答范围

当前 RAG 仅支持文档类（text/PDF）问答，不包含图片/视频视觉问答。图片/视频摘要由 AI 摘要模块处理。如需视觉问答可后续扩展。
