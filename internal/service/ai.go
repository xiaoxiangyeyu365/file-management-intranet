package service

import (
	"archive/zip"
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
	cfg          config.AIConfig
	client       *AIClient
	physicalRepo *repository.PhysicalFileRepository
	fileTagRepo  *repository.FileTagRepository
	storage      Storage
	sem          chan struct{}
	wg           sync.WaitGroup
}

// NewAIService creates a new AIService. Returns nil if AI is not enabled.
func NewAIService(cfg config.AIConfig, physicalRepo *repository.PhysicalFileRepository, fileTagRepo *repository.FileTagRepository, storage Storage) *AIService {
	if !cfg.Enabled {
		return nil
	}

	concurrent := cfg.MaxConcurrent
	if concurrent <= 0 {
		concurrent = 2
	}

	return &AIService{
		cfg:          cfg,
		client:       NewAIClient(AIClientConfig{BaseURL: strings.TrimRight(cfg.BaseURL, "/"), APIKey: cfg.APIKey, Model: cfg.Model, Timeout: cfg.Timeout}),
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

		if pf.Summary != "" {
			log.Printf("[AI] Physical %d already has summary, skipping", physicalID)
			return
		}

		if !shouldAutoProcess(mimeType, s.cfg) {
			return
		}

		summary, tags, err := s.generateSummary(ctx, pf, mimeType)
		if err != nil {
			log.Printf("[AI] Error generating summary for physical %d: %v", physicalID, err)
			return
		}

		now := time.Now()
		pf.Summary = summary
		pf.SummaryGeneratedAt = &now
		if err := s.physicalRepo.Update(ctx, pf); err != nil {
			log.Printf("[AI] Error updating summary for physical %d: %v", physicalID, err)
			return
		}

		s.writeTags(ctx, fileID, tags)
		log.Printf("[AI] Generated summary for physical %d (%d tags)", physicalID, len(tags))
	}()
}

// ProcessFileSync generates summary synchronously for manual regeneration.
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

	now := time.Now()
	pf.Summary = summary
	pf.SummaryGeneratedAt = &now
	if err := s.physicalRepo.Update(ctx, pf); err != nil {
		return "", nil, fmt.Errorf("update summary: %w", err)
	}

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

func (s *AIService) generateSummary(ctx context.Context, pf *model.PhysicalFile, mimeType string) (string, []string, error) {
	content, isVision, err := s.extractContent(pf, mimeType)
	if err != nil {
		log.Printf("[AI] Content extraction failed for %s, using filename: %v", pf.StoragePath, err)
		content = fmt.Sprintf("文件名：%s\n大小：%d 字节\n类型：%s", filepath.Base(pf.StoragePath), pf.Size, mimeType)
		isVision = false
	}

	var response string
	if isVision {
		visionModel := s.cfg.VisionModel
		if visionModel == "" {
			visionModel = s.cfg.Model
		}
		visionClient := NewAIClient(AIClientConfig{
			BaseURL: strings.TrimRight(s.cfg.BaseURL, "/"),
			APIKey:  s.cfg.APIKey,
			Model:   visionModel,
			Timeout: s.cfg.Timeout,
		})
		response, err = visionClient.ChatCompletionWithImage(ctx, s.cfg.SummaryPrompt, content, mimeType)
	} else {
		prompt := s.cfg.SummaryPrompt + "\n\n" + content
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

	// Fallback: detect MIME type from extension if it's generic
	if mimeType == "application/octet-stream" || mimeType == "" {
		detected := detectMimeTypeFromPath(absPath)
		if detected != "application/octet-stream" {
			mimeType = detected
			log.Printf("[AI] Detected MIME type from extension: %s -> %s", pf.StoragePath, mimeType)
		}
	}

	if isDocumentType(mimeType) {
		if isPDFType(mimeType) {
			text, err := extractPDFText(absPath)
			return text, false, err
		}
		if isDOCXType(mimeType) {
			text, err := extractDOCXText(absPath)
			return text, false, err
		}
		text, err := extractTextFile(absPath)
		return text, false, err
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

// parseAIResponse extracts summary and tags from LLM response.
func parseAIResponse(input string) (string, []string) {
	summaryRe := regexp.MustCompile(`(?s)摘要[：:]\s*(.+?)(?:\n标签|$)`)
	tagsRe := regexp.MustCompile(`标签[：:]\s*(.+)`)

	sm := summaryRe.FindStringSubmatch(input)
	tm := tagsRe.FindStringSubmatch(input)

	if sm == nil && tm == nil {
		preview := input
		if len(preview) > 200 {
			preview = preview[:200]
		}
		log.Printf("[AI] WARNING: Failed to parse LLM response — prompt format may not match. Response: %q", preview)
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
	return strings.HasPrefix(mimeType, "text/") || mimeType == "application/pdf" ||
		mimeType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document" ||
		mimeType == "application/msword"
}

func isPDFType(mimeType string) bool {
	return mimeType == "application/pdf"
}

func isDOCXType(mimeType string) bool {
	return mimeType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
}

func extractDOCXText(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer r.Close()

	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			data, err := io.ReadAll(rc)
			if err != nil {
				return "", err
			}

			// Extract text between <w:t> tags
			content := string(data)
			var buf strings.Builder
			re := regexp.MustCompile(`>([^<]+)<`)
			for _, match := range re.FindAllStringSubmatch(content, -1) {
				text := strings.TrimSpace(match[1])
				if text != "" {
					buf.WriteString(text)
				}
			}
			return buf.String(), nil
		}
	}

	return "", fmt.Errorf("word/document.xml not found in docx")
}

func isVideoType(mimeType string) bool {
	return strings.HasPrefix(mimeType, "video/")
}

func extractTextFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := string(data)
	if !isValidUTF8(data) {
		decoded, err := decodeGBK(data)
		if err == nil {
			content = decoded
		}
	}
	return content, nil
}

func extractPDFText(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf strings.Builder
	b, err := r.GetPlainText()
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(&buf, b); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func isValidUTF8(data []byte) bool {
	for i := 0; i < len(data); i++ {
		if data[i] > 127 {
			if data[i]&0xC0 == 0xC0 || data[i]&0xC0 == 0x80 {
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

// detectMimeTypeFromPath detects MIME type from file extension.
func detectMimeTypeFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	mimeTypes := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".mp4":  "video/mp4",
		".mp3":  "audio/mpeg",
		".zip":  "application/zip",
		".txt":  "text/plain",
		".md":   "text/markdown",
		".json": "application/json",
		".csv":  "text/csv",
		".log":  "text/plain",
		".xml":  "application/xml",
		".yaml": "text/yaml",
		".yml":  "text/yaml",
		".ini":  "text/plain",
		".cfg":  "text/plain",
		".conf": "text/plain",
		".sh":   "application/x-sh",
		".py":   "text/x-python",
		".js":   "application/javascript",
		".html": "text/html",
		".htm":  "text/html",
		".css":  "text/css",
	}
	if mt, ok := mimeTypes[ext]; ok {
		return mt
	}
	return "application/octet-stream"
}
