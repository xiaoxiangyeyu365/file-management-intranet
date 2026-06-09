# File Auto-Tagging & Summarization Design

## Goal

After file upload, automatically generate a summary and tags via LLM. Store them in the database for display and search. Supports manual regeneration. Configurable per file type (text/PDF auto, image/video manual only).

## Context

CloudBox is a campus intranet file storage system. Users upload documents, images, and videos. Currently files have no semantic metadata beyond filename and size. AI-generated summaries and tags enable better search and organization, and lay groundwork for future RAG-based Q&A.

## Approach

Async pipeline after upload: extract content → call OpenAI-compatible LLM API → parse summary + tags → write to DB. Text/PDF auto-triggered; image/video manual-only due to token cost. Worker pool limits concurrent LLM calls. Summary stored at PhysicalFile level for dedup; tags per-File with initial copy from PhysicalFile.

## Configuration

New `ai` section in `configs/config.yaml`:

```yaml
ai:
  enabled: true
  base_url: "https://api.deepseek.com/v1"
  api_key: ""                                # read from env AI_API_KEY if empty
  model: "deepseek-chat"
  vision_model: ""                           # falls back to model if empty
  max_content_length: 50000
  max_concurrent: 2                          # worker pool size
  auto_document: true                       # auto-process text/PDF files
  auto_image: false                         # auto-process images (costly)
  auto_video: false                         # auto-process videos (costly)
  timeout: 30                               # LLM API timeout in seconds
  summary_prompt: "请用中文为以下文档内容生成一段简洁摘要（不超过200字）和5个关键标签。格式：\n摘要：...\n标签：标签1,标签2,标签3,标签4,标签5"
```

`AIConfig` struct in `internal/config/config.go`:

```go
type AIConfig struct {
    Enabled           bool   `yaml:"enabled"`
    BaseURL           string `yaml:"base_url"`
    APIKey            string `yaml:"api_key"`
    Model             string `yaml:"model"`
    VisionModel       string `yaml:"vision_model"`
    MaxContentLength  int    `yaml:"max_content_length"`
    MaxConcurrent     int    `yaml:"max_concurrent"`
    AutoDocument      bool   `yaml:"auto_document"`
    AutoImage         bool   `yaml:"auto_image"`
    AutoVideo         bool   `yaml:"auto_video"`
    Timeout           int    `yaml:"timeout"`
    SummaryPrompt     string `yaml:"summary_prompt"`
}
```

`api_key` falls back to `AI_API_KEY` environment variable if empty in config. `max_concurrent` defaults to 2. `vision_model` falls back to `model` if empty.

## Data Model

### `physical_files` table — add columns

```sql
ALTER TABLE physical_files ADD COLUMN summary TEXT;
ALTER TABLE physical_files ADD COLUMN summary_generated_at DATETIME NULL;
```

`PhysicalFile` struct gains `Summary string` and `SummaryGeneratedAt *time.Time` fields.

Summary is stored at the PhysicalFile level to enable dedup: two Files pointing to the same PhysicalFile share one LLM-generated summary. When a new File points to an existing PhysicalFile that already has a summary, the tags are copied from an existing File's tags for that PhysicalFile.

### `files` table — no changes

Summary is NOT stored on the `files` table. It is retrieved via JOIN on `physical_ref` when needed (the `File.Physical` field is already populated by existing code in `GetFile` and similar queries).

### New `file_tags` table

| Column | Type | Constraints |
|--------|------|-------------|
| id | BIGINT | PK, auto-increment |
| file_id | BIGINT | NOT NULL, FK → files.id |
| tag | VARCHAR(50) | NOT NULL |
| created_at | DATETIME | NOT NULL |

Unique index: `(file_id, tag)`. Index: `(tag)` for tag-based search.

`FileTag` model:

```go
type FileTag struct {
    ID        int64     `gorm:"primaryKey"`
    FileID    int64     `gorm:"not null;index"`
    Tag       string    `gorm:"size:50;not null;uniqueIndex:idx_file_tag"`
    CreatedAt time.Time `gorm:"not null"`
}
```

Tags are per-File. Initial tags are copied from another File sharing the same PhysicalFile (if one exists). After copying, each user's tags are independent — they can be modified without affecting other users' files.

Cascade: when a file is permanently deleted (`PermanentDelete`), its `file_tags` records are deleted. Soft delete (MoveToTrash) does NOT remove tags — they persist so that restoring the file also restores its tags.

## AI Pipeline

### Trigger Points

After upload completes (`CompleteUpload`, `UploadFile`, `CreateFileFromPhysical`), check `ai.enabled` and submit file to the worker pool.

### Worker Pool

Buffered channel semaphore with capacity `max_concurrent`. Tasks block when pool is full. Each task tracked with `sync.WaitGroup` for graceful shutdown.

### Dedup Logic

Before calling the LLM, check if the PhysicalFile already has a non-empty `summary`. If yes, skip the LLM call — just copy the existing summary's tags to the new File. This avoids duplicate API costs when multiple users upload the same file content.

### Content Extraction

| File Type | Auto? | Extraction Method |
|-----------|-------|-------------------|
| Text (txt/md/csv/json/log) | Yes (if auto_document) | Read file content, auto-detect encoding with `golang.org/x/text` (GBK→UTF-8) |
| PDF | Yes (if auto_document) | `github.com/ledongthuc/pdf` extract text; fallback to filename |
| Word/PPT/Excel | No | Filename + extension + size only (no Office parsing lib in MVP) |
| Image (jpg/png/gif) | No (manual only) | Base64 encode → vision API with `image_url` message type |
| Video | No (manual only) | Filename + size only (no ffmpeg dependency) |
| Other | No | Filename + extension + size only |

Tag format relies on prompt constraint. No post-processing normalization in MVP — if LLM output is messy, adjust the prompt or add normalization later.

### LLM Call

Standard OpenAI Chat Completions format: `POST {base_url}/chat/completions`.

- Text/PDF: `{"role": "user", "content": "{summary_prompt}\n\n{extracted_text}"}`
- Image: `{"role": "user", "content": [{"type": "text", "text": "{summary_prompt}"}, {"type": "image_url", "image_url": {"url": "data:image/jpeg;base64,..."}}]}`
- Model selection: text files use `model`, image/video use `vision_model` (falls back to `model` if empty)
- Timeout: configurable, default 30s
- Retry: 1 retry with exponential backoff on transient errors (5xx, timeout)

### Response Parsing

LLM response expected format:
```
摘要：这是一份关于xxx的文档...
标签：数据分析,Python,机器学习,可视化,报告
```

Parse with regex: extract summary after `摘要：`, extract comma-separated tags after `标签：`. On parse failure, store the full response as summary with no tags, and log a WARNING suggesting the prompt format may not match.

### Writing Results

1. Update `physical_files.summary` and `physical_files.summary_generated_at` for the PhysicalFile
2. Delete existing tags for the File from `file_tags`
3. Insert new tags into `file_tags`

All in a transaction to ensure consistency.

### Error Handling

- LLM call failure → log warning, skip (don't block upload)
- Content extraction failure → fall back to filename-only generation
- Parse failure → store raw response as summary, no tags, log WARNING about prompt format
- Worker pool full → task queues (blocking submit with context timeout)
- Vision model does not support image → API error returned, log warning, skip

### WebDAV Override

When WebDAV PUT overwrites a file, the old File is soft-deleted and a new File + new PhysicalFile are created. The new PhysicalFile has no summary, so the auto-pipeline will process it. The old PhysicalFile's summary stays with the soft-deleted file (available on restore). No special handling needed.

## Manual Regeneration API

### POST /api/files/:id/ai-summary

- Requires JWT auth + file ownership (owner_id match) or admin role
- Returns 403 if `ai.enabled` is false
- Ignores `auto_*` config — always processes regardless of file type
- Does NOT check for existing summary — always calls LLM and overwrites
- Synchronous: waits for LLM response before returning
- Returns `{"summary": "...", "tags": ["tag1", "tag2"]}` on success
- Returns 404 if file not found
- Returns 403 if not owner and not admin

### GET /api/files/:id/ai-summary

- Requires JWT auth + file ownership or admin
- Returns existing summary and tags: `{"summary": "...", "tags": ["tag1", "tag2"], "generated_at": "2026-06-09T12:00:00Z"}`
- Returns 404 if no AI summary has been generated yet
- Summary is read from `file.Physical.Summary` (joined via `physical_ref`)

## Graceful Shutdown

AIService has a `Shutdown()` method. It uses `sync.WaitGroup` to track in-progress AI tasks. On shutdown, it waits for all tasks to complete (with a 30s timeout), then returns. Called in `main.go` before `auditService.Shutdown()`.

## Code Organization

### New Files

| Path | Responsibility |
|------|---------------|
| `internal/service/ai.go` | `AIService` — worker pool, orchestration, content extraction, result writing, Shutdown() |
| `internal/service/ai_client.go` | `AIClient` — OpenAI-compatible HTTP client with retry |
| `internal/model/file_tag.go` | `FileTag` struct |
| `internal/repository/file_tag.go` | `FileTagRepository` — CRUD + delete-by-file-id + find-by-tag + find-by-physical-ref |

### Modified Files

| Path | Change |
|------|--------|
| `internal/model/physical_file.go` | Add `Summary` and `SummaryGeneratedAt` fields |
| `internal/repository/db.go` | `createFileTagsTable()` + `physical_files` column migrations (summary, summary_generated_at) |
| `internal/service/upload.go` | Call `aiService.ProcessFile()` after upload (same pattern as previewService.ProcessImage) |
| `internal/handler/file.go` | Add `GetAISummary` + `RegenerateSummary` handlers |
| `cmd/server/main.go` | Init AIService, register new routes, call aiService.Shutdown() on exit |
| `internal/config/config.go` | Add `AIConfig` struct |
| `configs/config.yaml` | Add `ai` section |

## What This Does NOT Cover

- Frontend display of summaries/tags (separate UI task)
- Tag-based file search/filtering API (can be added later)
- Embedding vector generation for RAG (future milestone)
- Office document text extraction (MVP uses filename only)
- Video keyframe extraction (MVP uses filename only)
- Batch AI processing of existing files (manual per-file only)
- Per-user or global AI call rate limiting (future improvement)
- Tag post-processing normalization (rely on prompt constraint in MVP)
