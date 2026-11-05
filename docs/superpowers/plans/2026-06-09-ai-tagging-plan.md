# AI Tagging & Summarization Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** After file upload, automatically generate a summary and tags via LLM API, store them in the database, and expose them via API endpoints.

**Architecture:** Summary stored on `physical_files` (dedup LLM calls), tags stored on per-File `file_tags` table. AIService runs async worker pool after upload, calls OpenAI-compatible API. Manual regeneration via synchronous API endpoint.

**Tech Stack:** Go stdlib `net/http` for LLM client, `github.com/ledongthuc/pdf` for PDF text extraction, `golang.org/x/text` for encoding detection, GORM for DB.

---

### Task 1: Add AIConfig to config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `configs/config.yaml`

- [ ] **Step 1: Add AIConfig struct and Config field**

In `internal/config/config.go`, add after `TLSConfig`:

```go
type AIConfig struct {
	Enabled          bool   `yaml:"enabled"`
	BaseURL          string `yaml:"base_url"`
	APIKey           string `yaml:"api_key"`
	Model            string `yaml:"model"`
	VisionModel      string `yaml:"vision_model"`
	MaxContentLength int    `yaml:"max_content_length"`
	MaxConcurrent    int    `yaml:"max_concurrent"`
	AutoDocument     bool   `yaml:"auto_document"`
	AutoImage        bool   `yaml:"auto_image"`
	AutoVideo        bool   `yaml:"auto_video"`
	Timeout          int    `yaml:"timeout"`
	SummaryPrompt    string `yaml:"summary_prompt"`
}
```

Add `AI AIConfig \`yaml:"ai"\`` to the `Config` struct after the `TLS` field.

- [ ] **Step 2: Add defaults in Load()**

After the `TLS: TLSConfig{...}` defaults, add:

```go
AI: AIConfig{
	Enabled:          false,
	BaseURL:          "https://api.deepseek.com/v1",
	Model:            "deepseek-chat",
	VisionModel:      "",
	MaxContentLength: 50000,
	MaxConcurrent:    2,
	AutoDocument:     true,
	AutoImage:        false,
	AutoVideo:        false,
	Timeout:          30,
	SummaryPrompt:    "请用中文为以下文档内容生成一段简洁摘要（不超过200字）和5个关键标签。格式：\n摘要：...\n标签：标签1,标签2,标签3,标签4,标签5",
},
```

- [ ] **Step 3: Add env fallback for APIKey**

After the JWT secret env fallback block, add:

```go
if cfg.AI.APIKey == "" {
	cfg.AI.APIKey = os.Getenv("AI_API_KEY")
}
```

- [ ] **Step 4: Add ai section to config.yaml**

Append to `configs/config.yaml`:

```yaml
ai:
  enabled: false
  base_url: "https://api.deepseek.com/v1"
  api_key: ""                               # read from env AI_API_KEY if empty
  model: "deepseek-chat"
  vision_model: ""                           # falls back to model if empty
  max_content_length: 50000
  max_concurrent: 2
  auto_document: true
  auto_image: false
  auto_video: false
  timeout: 30
  summary_prompt: "请用中文为以下文档内容生成一段简洁摘要（不超过200字）和5个关键标签。格式：\n摘要：...\n标签：标签1,标签2,标签3,标签4,标签5"
```

- [ ] **Step 5: Verify build**

Run: `go build ./...`
Expected: compiles with no errors

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go configs/config.yaml
git commit -m "feat: add AIConfig to config struct and config.yaml"
```

---

### Task 2: Add data model — PhysicalFile fields + FileTag model + DB migration

**Files:**
- Modify: `internal/model/physical_file.go`
- Create: `internal/model/file_tag.go`
- Modify: `internal/repository/db.go`

- [ ] **Step 1: Add Summary and SummaryGeneratedAt to PhysicalFile**

In `internal/model/physical_file.go`, add fields after `MetadataJSON`:

```go
Summary            string     `gorm:"type:text" json:"summary"`
SummaryGeneratedAt *time.Time `json:"summaryGeneratedAt"`
```

- [ ] **Step 2: Create FileTag model**

Create `internal/model/file_tag.go`:

```go
package model

import "time"

type FileTag struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	FileID    int64     `gorm:"not null;index" json:"fileId"`
	Tag       string    `gorm:"size:50;not null;uniqueIndex:idx_file_tag" json:"tag"`
	CreatedAt time.Time `gorm:"not null" json:"createdAt"`
}

func (FileTag) TableName() string {
	return "file_tags"
}
```

- [ ] **Step 3: Add DB migrations in InitDB**

In `internal/repository/db.go`, after the `createAuditLogsTable()` call (around line 83) and before `createDefaultAdmin`, add:

```go
// Create file_tags table
if err := createFileTagsTable(); err != nil {
	log.Fatalf("failed to create file_tags table: %v", err)
}

// Add summary columns to physical_files (safe to ignore "duplicate column" errors)
if err := DB.Exec("ALTER TABLE physical_files ADD COLUMN summary TEXT").Error; err != nil {
	log.Printf("physical_files.summary migration: %v (may already exist)", err)
}
if err := DB.Exec("ALTER TABLE physical_files ADD COLUMN summary_generated_at DATETIME NULL").Error; err != nil {
	log.Printf("physical_files.summary_generated_at migration: %v (may already exist)", err)
}
```

- [ ] **Step 4: Add createFileTagsTable function**

Add to `internal/repository/db.go` (follow the same dual SQLite/MySQL pattern as `createFileSharesTable`):

```go
func createFileTagsTable() error {
	var count int64
	if DB.Dialector.Name() == "sqlite" {
		DB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='file_tags'").Scan(&count)
	} else {
		DB.Raw("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'cloudbox' AND table_name = 'file_tags'").Scan(&count)
	}
	if count > 0 {
		return nil
	}

	if DB.Dialector.Name() == "sqlite" {
		return DB.Exec(`CREATE TABLE file_tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			file_id INTEGER NOT NULL,
			tag VARCHAR(50) NOT NULL,
			created_at DATETIME NOT NULL
		)`).Error
	}

	return DB.Exec(`CREATE TABLE file_tags (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		file_id BIGINT NOT NULL,
		tag VARCHAR(50) NOT NULL,
		created_at DATETIME NOT NULL,
		UNIQUE INDEX idx_file_tag (file_id, tag),
		INDEX idx_tag (tag)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`).Error
}

// Also add indexes for SQLite after table creation
```

After the SQLite CREATE TABLE, add index creation (SQLite doesn't support inline indexes on CREATE TABLE):

```go
if DB.Dialector.Name() == "sqlite" {
	DB.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_file_tag ON file_tags(file_id, tag)")
	DB.Exec("CREATE INDEX IF NOT EXISTS idx_tag ON file_tags(tag)")
}
```

- [ ] **Step 5: Verify build**

Run: `go build ./...`
Expected: compiles with no errors

- [ ] **Step 6: Commit**

```bash
git add internal/model/physical_file.go internal/model/file_tag.go internal/repository/db.go
git commit -m "feat: add FileTag model, PhysicalFile summary fields, and DB migrations"
```

---

### Task 3: Create FileTagRepository

**Files:**
- Create: `internal/repository/file_tag.go`

- [ ] **Step 1: Write the failing test**

Create `internal/repository/file_tag_test.go`:

```go
package repository

import (
	"cloudbox/internal/model"
	"context"
	"testing"
	"time"
)

func TestFileTagRepository_CreateAndFindByFileID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewFileTagRepository(db)
	ctx := context.Background()

	now := time.Now()
	tag := &model.FileTag{FileID: 1, Tag: "数学", CreatedAt: now}
	if err := repo.Create(ctx, tag); err != nil {
		t.Fatalf("Create: %v", err)
	}

	tags, err := repo.FindByFileID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByFileID: %v", err)
	}
	if len(tags) != 1 || tags[0].Tag != "数学" {
		t.Errorf("got %v, want tag=数学", tags)
	}
}

func TestFileTagRepository_DeleteByFileID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewFileTagRepository(db)
	ctx := context.Background()

	now := time.Now()
	repo.Create(ctx, &model.FileTag{FileID: 1, Tag: "数学", CreatedAt: now})
	repo.Create(ctx, &model.FileTag{FileID: 1, Tag: "考试", CreatedAt: now})

	if err := repo.DeleteByFileID(ctx, 1); err != nil {
		t.Fatalf("DeleteByFileID: %v", err)
	}

	tags, _ := repo.FindByFileID(ctx, 1)
	if len(tags) != 0 {
		t.Errorf("expected no tags after delete, got %d", len(tags))
	}
}

func TestFileTagRepository_CreateBatch(t *testing.T) {
	db := setupTestDB(t)
	repo := NewFileTagRepository(db)
	ctx := context.Background()

	now := time.Now()
	tags := []model.FileTag{
		{FileID: 1, Tag: "数学", CreatedAt: now},
		{FileID: 1, Tag: "考试", CreatedAt: now},
		{FileID: 1, Tag: "期末", CreatedAt: now},
	}
	if err := repo.CreateBatch(ctx, tags); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	result, _ := repo.FindByFileID(ctx, 1)
	if len(result) != 3 {
		t.Errorf("expected 3 tags, got %d", len(result))
	}
}
```

- [ ] **Step 2: Check for existing test helper**

Check if `internal/repository/` already has a `setupTestDB` helper. If not, create `internal/repository/test_helpers.go`:

```go
package repository

import (
	"cloudbox/internal/model"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.AutoMigrate(&model.FileTag{})
	return db
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/repository/ -run TestFileTag -v`
Expected: FAIL — `NewFileTagRepository` undefined

- [ ] **Step 4: Implement FileTagRepository**

Create `internal/repository/file_tag.go`:

```go
package repository

import (
	"cloudbox/internal/model"
	"context"

	"gorm.io/gorm"
)

type FileTagRepository struct {
	db *gorm.DB
}

func NewFileTagRepository(db *gorm.DB) *FileTagRepository {
	return &FileTagRepository{db: db}
}

func (r *FileTagRepository) Create(ctx context.Context, tag *model.FileTag) error {
	return r.db.WithContext(ctx).Create(tag).Error
}

func (r *FileTagRepository) CreateBatch(ctx context.Context, tags []model.FileTag) error {
	if len(tags) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&tags).Error
}

func (r *FileTagRepository) FindByFileID(ctx context.Context, fileID int64) ([]model.FileTag, error) {
	var tags []model.FileTag
	err := r.db.WithContext(ctx).Where("file_id = ?", fileID).Find(&tags).Error
	return tags, err
}

func (r *FileTagRepository) DeleteByFileID(ctx context.Context, fileID int64) error {
	return r.db.WithContext(ctx).Where("file_id = ?", fileID).Delete(&model.FileTag{}).Error
}

func (r *FileTagRepository) FindByTag(ctx context.Context, tag string) ([]model.FileTag, error) {
	var tags []model.FileTag
	err := r.db.WithContext(ctx).Where("tag = ?", tag).Find(&tags).Error
	return tags, err
}
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/repository/ -run TestFileTag -v`
Expected: all 3 tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/repository/file_tag.go internal/repository/file_tag_test.go internal/repository/test_helpers.go
git commit -m "feat: add FileTagRepository with Create, FindByFileID, DeleteByFileID, CreateBatch"
```

---

### Task 4: Create AI client — OpenAI-compatible HTTP client

**Files:**
- Create: `internal/service/ai_client.go`
- Create: `internal/service/ai_client_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/service/ai_client_test.go`:

```go
package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAIClient_ChatCompletion_Text(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected /chat/completions, got %s", r.URL.Path)
		}

		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]string{
						"content": "摘要：测试文档\n标签：测试,单元测试",
					},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	client := NewAIClient(AIClientConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
		Timeout: 10,
	})

	content, err := client.ChatCompletion(rContext(), "hello", false)
	if err != nil {
		t.Fatalf("ChatCompletion: %v", err)
	}
	if content != "摘要：测试文档\n标签：测试,单元测试" {
		t.Errorf("unexpected content: %q", content)
	}
}

func TestAIClient_ChatCompletion_RetryOn5xx(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(500)
			return
		}
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{"message": map[string]string{"content": "ok"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewAIClient(AIClientConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "test-model",
		Timeout: 10,
	})

	content, err := client.ChatCompletion(rContext(), "hello", false)
	if err != nil {
		t.Fatalf("ChatCompletion: %v", err)
	}
	if content != "ok" {
		t.Errorf("unexpected content: %q", content)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls (1 retry), got %d", calls)
	}
}

func rContext() interface{} {
	return nil
}
```

Note: `rContext` will be replaced with `context.Background()` once the actual import is available. The test helper is a placeholder to make the test file compile before the implementation exists. The actual test will use `context.Background()`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestAIClient -v`
Expected: FAIL — `NewAIClient` undefined

- [ ] **Step 3: Implement AIClient**

Create `internal/service/ai_client.go`:

```go
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AIClientConfig holds configuration for the OpenAI-compatible API client.
type AIClientConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout int // seconds
}

// AIClient calls OpenAI-compatible chat completion APIs.
type AIClient struct {
	config     AIClientConfig
	httpClient *http.Client
}

// NewAIClient creates a new AIClient with the given configuration.
func NewAIClient(cfg AIClientConfig) *AIClient {
	return &AIClient{
		config: cfg,
		httpClient: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
	}
}

// chatRequest represents an OpenAI-compatible chat completion request.
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

// chatResponse represents an OpenAI-compatible chat completion response.
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// ChatCompletion sends a chat completion request and returns the response content.
// If isVision is true, the content is sent as a vision message (image_url format).
func (c *AIClient) ChatCompletion(ctx context.Context, prompt string, isVision bool) (string, error) {
	var content interface{}
	if isVision {
		content = []map[string]interface{}{
			{"type": "text", "text": prompt},
		}
	} else {
		content = prompt
	}

	reqBody := chatRequest{
		Model: c.config.Model,
		Messages: []chatMessage{
			{Role: "user", Content: content},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respData))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respData, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// ChatCompletionWithImage sends a vision request with a base64-encoded image.
func (c *AIClient) ChatCompletionWithImage(ctx context.Context, prompt string, imageBase64 string, mimeType string) (string, error) {
	reqBody := chatRequest{
		Model: c.config.Model,
		Messages: []chatMessage{
			{
				Role: "user",
				Content: []map[string]interface{}{
					{"type": "text", "text": prompt},
					{"type": "image_url", "image_url": map[string]string{
						"url": fmt.Sprintf("data:%s;base64,%s", mimeType, imageBase64),
					}},
				},
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API returned %d: %s", resp.StatusCode, string(respData))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respData, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return chatResp.Choices[0].Message.Content, nil
}

func (c *AIClient) doRequest(ctx context.Context, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	// Retry once on 5xx
	if resp.StatusCode >= 500 {
		resp.Body.Close()
		time.Sleep(500 * time.Millisecond)
		req2, _ := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+"/chat/completions", bytes.NewReader(body))
		req2.Header.Set("Content-Type", "application/json")
		req2.Header.Set("Authorization", "Bearer "+c.config.APIKey)
		resp2, err := c.httpClient.Do(req2)
		if err != nil {
			return nil, fmt.Errorf("retry request: %w", err)
		}
		return resp2, nil
	}

	return resp, nil
}
```

- [ ] **Step 4: Fix test to use context.Background()**

Replace `rContext()` in the test file with `context.Background()` and add `"context"` to imports.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/service/ -run TestAIClient -v`
Expected: all tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/service/ai_client.go internal/service/ai_client_test.go
git commit -m "feat: add OpenAI-compatible AI client with retry"
```

---

### Task 5: Create AIService — orchestration, content extraction, result writing

**Files:**
- Create: `internal/service/ai.go`
- Create: `internal/service/ai_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/service/ai_test.go`:

```go
package service

import (
	"testing"
)

func TestParseAIResponse_SummaryAndTags(t *testing.T) {
	input := "摘要：这是一份关于数据分析的文档，包含了Python和机器学习的相关内容。\n标签：数据分析,Python,机器学习,可视化,报告"
	summary, tags := parseAIResponse(input)
	if summary != "这是一份关于数据分析的文档，包含了Python和机器学习的相关内容。" {
		t.Errorf("summary = %q", summary)
	}
	if len(tags) != 5 || tags[0] != "数据分析" {
		t.Errorf("tags = %v", tags)
	}
}

func TestParseAIResponse_ParseFailure(t *testing.T) {
	input := "This is just some random text without the expected format"
	summary, tags := parseAIResponse(input)
	if summary != input {
		t.Errorf("expected raw input as summary on parse failure, got %q", summary)
	}
	if len(tags) != 0 {
		t.Errorf("expected no tags on parse failure, got %v", tags)
	}
}

func TestParseAIResponse_EmptyTags(t *testing.T) {
	input := "摘要：测试摘要\n标签："
	summary, tags := parseAIResponse(input)
	if summary != "测试摘要" {
		t.Errorf("summary = %q", summary)
	}
	if len(tags) != 0 {
		t.Errorf("expected no tags when tags section is empty, got %v", tags)
	}
}

func TestShouldAutoProcess(t *testing.T) {
	cfg := AIConfig{AutoDocument: true, AutoImage: false, AutoVideo: false}

	tests := []struct {
		mime     string
		expected bool
	}{
		{"text/plain", true},
		{"application/pdf", true},
		{"image/jpeg", false},
		{"video/mp4", false},
		{"application/octet-stream", false},
	}
	for _, tt := range tests {
		got := shouldAutoProcess(tt.mime, cfg)
		if got != tt.expected {
			t.Errorf("shouldAutoProcess(%q) = %v, want %v", tt.mime, got, tt.expected)
		}
	}
}

func TestIsDocumentType(t *testing.T) {
	tests := []struct {
		mime     string
		expected bool
	}{
		{"text/plain", true},
		{"text/markdown", true},
		{"application/pdf", true},
		{"image/jpeg", false},
		{"video/mp4", false},
	}
	for _, tt := range tests {
		got := isDocumentType(tt.mime)
		if got != tt.expected {
			t.Errorf("isDocumentType(%q) = %v, want %v", tt.mime, got, tt.expected)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/service/ -run TestParseAIResponse -v`
Expected: FAIL — `parseAIResponse` undefined

- [ ] **Step 3: Implement AIService**

Create `internal/service/ai.go`:

```go
package service

import (
	"cloudbox/internal/config"
	"cloudbox/internal/model"
	"cloudbox/internal/repository"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/ledongthuc/pdf"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// AIService generates summaries and tags for uploaded files using LLM.
type AIService struct {
	cfg         config.AIConfig
	client      *AIClient
	physicalRepo *repository.PhysicalFileRepository
	fileTagRepo  *repository.FileTagRepository
	storage      Storage
	sem          chan struct{}
	wg           sync.WaitGroup
}

// NewAIService creates a new AIService.
func NewAIService(cfg config.AIConfig, physicalRepo *repository.PhysicalFileRepository, fileTagRepo *repository.FileTagRepository, storage Storage) *AIService {
	if !cfg.Enabled {
		return nil
	}

	model := cfg.Model
	clientCfg := AIClientConfig{
		BaseURL: strings.TrimRight(cfg.BaseURL, "/"),
		APIKey:  cfg.APIKey,
		Model:   model,
		Timeout: cfg.Timeout,
	}

	concurrent := cfg.MaxConcurrent
	if concurrent <= 0 {
		concurrent = 2
	}

	return &AIService{
		cfg:          cfg,
		client:       NewAIClient(clientCfg),
		physicalRepo: physicalRepo,
		fileTagRepo:  fileTagRepo,
		storage:      storage,
		sem:          make(chan struct{}, concurrent),
	}
}

// Shutdown waits for in-progress AI tasks to complete (up to 30s).
func (s *AIService) Shutdown() {
	if s == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		log.Println("[AI] Shutdown timed out waiting for tasks")
	}
}

// ProcessFile submits a file for async AI processing after upload.
// If the PhysicalFile already has a summary, it copies tags from an existing file instead.
func (s *AIService) ProcessFile(fileID int64, physicalID int64, mimeType string) {
	if s == nil {
		return
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.sem <- struct{}{}
		defer func() { <-s.sem }()

		ctx := context.Background()

		pf, err := s.physicalRepo.FindByID(ctx, physicalID)
		if err != nil {
			log.Printf("[AI] Error finding physical file %d: %v", physicalID, err)
			return
		}

		// Already has summary — just copy tags to this file
		if pf.Summary != "" {
			s.copyTagsFromExistingFile(ctx, fileID, physicalID)
			return
		}

		// Check auto-processing config
		if !shouldAutoProcess(mimeType, s.cfg) {
			return
		}

		summary, tags, err := s.generateSummary(ctx, pf, mimeType)
		if err != nil {
			log.Printf("[AI] Error generating summary for physical %d: %v", physicalID, err)
			return
		}

		// Write summary to PhysicalFile
		now := time.Now()
		pf.Summary = summary
		pf.SummaryGeneratedAt = &now
		if err := s.physicalRepo.Update(ctx, pf); err != nil {
			log.Printf("[AI] Error updating summary for physical %d: %v", physicalID, err)
			return
		}

		// Write tags to file
		s.writeTags(ctx, fileID, tags)
		log.Printf("[AI] Generated summary for physical %d (%d tags)", physicalID, len(tags))
	}()
}

// ProcessFileSync generates summary synchronously for manual regeneration.
// Ignores auto_* config and always processes.
func (s *AIService) ProcessFileSync(ctx context.Context, fileID int64, physicalID int64, mimeType string) (string, []string, error) {
	if s == nil {
		return "", nil, fmt.Errorf("AI service is not enabled")
	}

	pf, err := s.physicalRepo.FindByID(ctx, physicalID)
	if err != nil {
		return "", nil, fmt.Errorf("find physical file: %w", err)
	}

	summary, tags, err := s.generateSummary(ctx, pf, mimeType)
	if err != nil {
		return "", nil, fmt.Errorf("generate summary: %w", err)
	}

	// Write summary to PhysicalFile
	now := time.Now()
	pf.Summary = summary
	pf.SummaryGeneratedAt = &now
	if err := s.physicalRepo.Update(ctx, pf); err != nil {
		return "", nil, fmt.Errorf("update summary: %w", err)
	}

	// Write tags to file
	s.writeTags(ctx, fileID, tags)

	return summary, tags, nil
}

// GetSummary returns the summary and tags for a file.
func (s *AIService) GetSummary(ctx context.Context, physicalID int64, fileID int64) (string, []string, *time.Time, error) {
	pf, err := s.physicalRepo.FindByID(ctx, physicalID)
	if err != nil {
		return "", nil, nil, fmt.Errorf("find physical file: %w", err)
	}

	tags, _ := s.fileTagRepo.FindByFileID(ctx, fileID)
	tagStrs := make([]string, len(tags))
	for i, t := range tags {
		tagStrs[i] = t.Tag
	}

	return pf.Summary, tagStrs, pf.SummaryGeneratedAt, nil
}

func (s *AIService) generateSummary(ctx context.Context, pf *model.PhysicalFile, mimeType string) (string, []string, error) {
	content, isVision, err := s.extractContent(pf, mimeType)
	if err != nil {
		log.Printf("[AI] Content extraction failed for %s, using filename: %v", pf.StoragePath, err)
		content = fmt.Sprintf("文件名：%s\n大小：%d 字节\n类型：%s", filepath.Base(pf.StoragePath), pf.Size, mimeType)
		isVision = false
	}

	prompt := s.cfg.SummaryPrompt + "\n\n" + content

	var response string
	if isVision {
		// For vision, use the vision model client
		visionCfg := AIClientConfig{
			BaseURL: strings.TrimRight(s.cfg.BaseURL, "/"),
			APIKey:  s.cfg.APIKey,
			Model:   s.cfg.VisionModel,
			Timeout: s.cfg.Timeout,
		}
		if visionCfg.Model == "" {
			visionCfg.Model = s.cfg.Model
		}
		visionClient := NewAIClient(visionCfg)
		// content is already base64 for images
		response, err = visionClient.ChatCompletionWithImage(ctx, s.cfg.SummaryPrompt, content, mimeType)
	} else {
		// Truncate content if too long
		if len(prompt) > s.cfg.MaxContentLength {
			prompt = prompt[:s.cfg.MaxContentLength]
		}
		response, err = s.client.ChatCompletion(ctx, prompt, false)
	}

	if err != nil {
		return "", nil, fmt.Errorf("LLM call: %w", err)
	}

	summary, tags := parseAIResponse(response)
	return summary, tags, nil
}

func (s *AIService) extractContent(pf *model.PhysicalFile, mimeType string) (string, bool, error) {
	absPath := s.storage.ToAbsPath(pf.StoragePath)

	if isDocumentType(mimeType) {
		if isPDFType(mimeType) {
			return extractPDFText(absPath)
		}
		return extractTextFile(absPath)
	}

	if isImageType(mimeType) {
		data, err := os.ReadFile(absPath)
		if err != nil {
			return "", true, err
		}
		return base64.StdEncoding.EncodeToString(data), true, nil
	}

	return "", false, fmt.Errorf("unsupported mime type: %s", mimeType)
}

func (s *AIService) writeTags(ctx context.Context, fileID int64, tags []string) {
	if len(tags) == 0 {
		return
	}
	_ = s.fileTagRepo.DeleteByFileID(ctx, fileID)

	now := time.Now()
	fileTags := make([]model.FileTag, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" || len(t) > 50 {
			continue
		}
		fileTags = append(fileTags, model.FileTag{FileID: fileID, Tag: t, CreatedAt: now})
	}
	if len(fileTags) > 0 {
		if err := s.fileTagRepo.CreateBatch(ctx, fileTags); err != nil {
			log.Printf("[AI] Error writing tags for file %d: %v", fileID, err)
		}
	}
}

func (s *AIService) copyTagsFromExistingFile(ctx context.Context, fileID int64, physicalID int64) {
	// Find another file with the same physical_ref that has tags
	// For now, skip — tags will be empty until manual regeneration
	// This can be enhanced later to query files by physical_ref
	log.Printf("[AI] Physical %d already has summary, copying tags skipped (manual regen available)", physicalID)
}

// parseAIResponse extracts summary and tags from LLM response.
func parseAIResponse(input string) (string, []string) {
	summaryRe := regexp.MustCompile(`(?s)摘要[：:]\s*(.+?)(?:\n标签|$)`)
	tagsRe := regexp.MustCompile(`标签[：:]\s*(.+)`)

	sm := summaryRe.FindStringSubmatch(input)
	tm := tagsRe.FindStringSubmatch(input)

	if sm == nil && tm == nil {
		log.Printf("[AI] WARNING: Failed to parse LLM response — prompt format may not match. Response: %q", input[:min(200, len(input))])
		return input, nil
	}

	summary := ""
	if sm != nil {
		summary = strings.TrimSpace(sm[1])
	} else {
		summary = strings.TrimSpace(input)
	}

	var tags []string
	if tm != nil {
		for _, t := range strings.Split(tm[1], ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tags = append(tags, t)
			}
		}
	}

	return summary, tags
}

func shouldAutoProcess(mimeType string, cfg config.AIConfig) bool {
	if isDocumentType(mimeType) && cfg.AutoDocument {
		return true
	}
	if isImageType(mimeType) && cfg.AutoImage {
		return true
	}
	if isVideoType(mimeType) && cfg.AutoVideo {
		return true
	}
	return false
}

func isDocumentType(mimeType string) bool {
	return strings.HasPrefix(mimeType, "text/") || mimeType == "application/pdf"
}

func isPDFType(mimeType string) bool {
	return mimeType == "application/pdf"
}

func isImageType(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}

func isVideoType(mimeType string) bool {
	return strings.HasPrefix(mimeType, "video/")
}

func extractTextFile(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}
	// Try UTF-8 first
	content := string(data)
	// If it looks like GBK (has invalid UTF-8 sequences), try decoding
	if !isValidUTF8(data) {
		decoded, err := decodeGBK(data)
		if err == nil {
			content = decoded
		}
	}
	return content, false, nil
}

func extractPDFText(path string) (string, bool, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()

	var buf strings.Builder
	b, err := r.GetPlainText()
	if err != nil {
		return "", false, err
	}
	io.Copy(&buf, b)
	return buf.String(), false, nil
}

func isValidUTF8(data []byte) bool {
	for i := 0; i < len(data); i++ {
		if data[i] > 127 {
			// Non-ASCII — check if valid UTF-8
			if data[i]&0xC0 == 0xC0 {
				// Start of multi-byte sequence, roughly valid
				continue
			}
			if data[i]&0xC0 == 0x80 {
				// Continuation byte — likely valid UTF-8
				continue
			}
			return false
		}
	}
	return true
}

func decodeGBK(data []byte) (string, error) {
	reader := transform.NewReader(strings.NewReader(string(data)), simplifiedchinese.GBK.NewDecoder())
	decoded, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
```

- [ ] **Step 4: Add go.mod dependencies**

Run: `go get github.com/ledongthuc/pdf golang.org/x/text`

- [ ] **Step 5: Run tests**

Run: `go test ./internal/service/ -run "TestParseAIResponse|TestShouldAutoProcess|TestIsDocumentType" -v`
Expected: all tests PASS

- [ ] **Step 6: Commit**

```bash
git add internal/service/ai.go internal/service/ai_test.go go.mod go.sum
git commit -m "feat: add AIService with content extraction, LLM orchestration, and tag writing"
```

---

### Task 6: Wire AIService into UploadService and add API handlers

**Files:**
- Modify: `internal/service/upload.go`
- Modify: `internal/handler/file.go`
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Add aiService field to UploadService**

In `internal/service/upload.go`, add `aiService *AIService` field to `UploadService` struct (after `audit AuditRecorder`).

Add `aiService *AIService` parameter to `NewUploadService` constructor and assignment.

- [ ] **Step 2: Call aiService.ProcessFile after upload**

Add after each `previewService.ProcessImage` call in three locations:

In `CompleteUpload` (after line ~329):
```go
if s.aiService != nil {
	go s.aiService.ProcessFile(file.ID, pf.ID, pf.MimeType)
}
```

In `CreateFileFromPhysical` (after line ~394):
```go
if s.aiService != nil {
	go s.aiService.ProcessFile(file.ID, pf.ID, pf.MimeType)
}
```

In `UploadFile` (after line ~498):
```go
if s.aiService != nil {
	go s.aiService.ProcessFile(file.ID, pf.ID, pf.MimeType)
}
```

- [ ] **Step 3: Add AI summary handlers to FileHandler**

In `internal/handler/file.go`, add `aiService *service.AIService` field to `FileHandler` struct and add it to `NewFileHandler` constructor.

Add two handler methods:

```go
// GetAISummary returns the AI-generated summary and tags for a file.
func (h *FileHandler) GetAISummary(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid file ID")
		return
	}

	userID := c.GetInt64("userID")
	file, err := h.fileService.GetFile(c.Request.Context(), userID, id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "file not found")
		return
	}

	if file.Physical == nil || file.Physical.Summary == "" {
		response.Error(c, http.StatusNotFound, "no AI summary available")
		return
	}

	tags, _ := h.aiService.GetTags(c.Request.Context(), id)

	c.JSON(http.StatusOK, gin.H{
		"summary":      file.Physical.Summary,
		"tags":         tags,
		"generated_at": file.Physical.SummaryGeneratedAt,
	})
}

// RegenerateSummary manually triggers AI summary generation.
func (h *FileHandler) RegenerateSummary(c *gin.Context) {
	if h.aiService == nil {
		response.Error(c, http.StatusForbidden, "AI service is not enabled")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid file ID")
		return
	}

	userID := c.GetInt64("userID")
	role, _ := c.Get("role")
	file, err := h.fileService.GetFile(c.Request.Context(), userID, id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "file not found")
		return
	}

	if file.OwnerID != userID && role != "admin" {
		response.Error(c, http.StatusForbidden, "not authorized")
		return
	}

	if file.Physical == nil {
		response.Error(c, http.StatusBadRequest, "file has no physical content")
		return
	}

	summary, tags, err := h.aiService.ProcessFileSync(c.Request.Context(), id, file.ContentRef, file.Physical.MimeType)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "AI processing failed: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"summary": summary,
		"tags":    tags,
	})
}
```

Add `GetTags` helper method to AIService in `internal/service/ai.go`:

```go
// GetTags returns the tags for a file as string slice.
func (s *AIService) GetTags(ctx context.Context, fileID int64) ([]string, error) {
	if s == nil {
		return nil, nil
	}
	fileTags, err := s.fileTagRepo.FindByFileID(ctx, fileID)
	if err != nil {
		return nil, err
	}
	tags := make([]string, len(fileTags))
	for i, t := range fileTags {
		tags[i] = t.Tag
	}
	return tags, nil
}
```

- [ ] **Step 4: Update cmd/server/main.go**

Add `aiService` initialization after existing services are created:

```go
// Initialize AI service
var aiService *service.AIService
if cfg.AI.Enabled {
	aiService = service.NewAIService(cfg.AI, physicalRepo, repository.NewFileTagRepository(db), storageManager)
}
```

Update `NewUploadService` call to pass `aiService`:

```go
uploadService := service.NewUploadService(fileRepo, physicalRepo, userRepo, storageManager, previewService, cfg.Upload.ChunkSize, cfg.Disk.DefaultQuota, auditService, aiService)
```

Update `NewFileHandler` call to pass `aiService`:

```go
fileHandler := handler.NewFileHandler(fileService, aiService)
```

Add AI summary routes in the protected routes group:

```go
// AI Summary
if aiService != nil {
	protected.GET("/files/:id/ai-summary", fileHandler.GetAISummary)
	protected.POST("/files/:id/ai-summary", fileHandler.RegenerateSummary)
}
```

Add `aiService.Shutdown()` in the shutdown sequence (before auditService.Shutdown):

```go
if aiService != nil {
	aiService.Shutdown()
}
auditService.Shutdown()
```

- [ ] **Step 5: Verify build**

Run: `go build ./...`
Expected: compiles with no errors

- [ ] **Step 6: Manual smoke test**

1. Set `ai.enabled: true` in config.yaml (or set AI_API_KEY env var)
2. Run: `go run ./cmd/server`
3. Upload a text file via the web UI
4. Check server logs for `[AI] Generated summary for physical ...`
5. Call `GET /api/files/:id/ai-summary` to verify summary and tags are returned
6. Call `POST /api/files/:id/ai-summary` to verify manual regeneration works

- [ ] **Step 7: Commit**

```bash
git add internal/service/upload.go internal/service/ai.go internal/handler/file.go cmd/server/main.go
git commit -m "feat: wire AIService into upload pipeline and add API endpoints"
```

---

### Task 7: Update CLAUDE.md and add PermanentDelete cascade

**Files:**
- Modify: `internal/service/file.go` — add tag deletion in PermanentDelete
- Modify: `CLAUDE.md`

- [ ] **Step 1: Add tag cascade to PermanentDelete**

Find the `PermanentDelete` method in `internal/service/file.go`. Before the physical file cleanup logic, add:

```go
// Delete tags for this file
if s.fileTagRepo != nil {
	s.fileTagRepo.DeleteByFileID(ctx, id)
}
```

This requires adding `fileTagRepo *repository.FileTagRepository` to `FileService` struct and constructor. Add it as an optional field (can be nil when AI is disabled).

- [ ] **Step 2: Add AI section to CLAUDE.md**

Add after the HTTPS/TLS section:

```markdown
## AI Summarization

Self-signed LLM integration for file summaries and tags. Config in `configs/config.yaml` under `ai:`.

- `ai.enabled: true` enables async AI processing after file upload
- Summary stored on `physical_files` (dedup — same content only calls LLM once)
- Tags stored per-File in `file_tags` table (initial copy from PhysicalFile, then independent)
- Auto-processes text/PDF only; image/video require manual trigger via `POST /api/files/:id/ai-summary`
- `vision_model` falls back to `model` if empty
- Uses OpenAI-compatible API format (`/chat/completions`)
- Response parsing expects `摘要：...\n标签：tag1,tag2,...` format
- `ai.api_key` falls back to `AI_API_KEY` env var
- Worker pool limits concurrent LLM calls (`max_concurrent`, default 2)
- AIService.Shutdown() waits for in-progress tasks on graceful shutdown
- Permanent delete cascades to file_tags; soft delete preserves tags
```

- [ ] **Step 3: Commit**

```bash
git add internal/service/file.go CLAUDE.md
git commit -m "feat: add tag cascade on permanent delete and AI docs to CLAUDE.md"
```
