package service

import (
	"cloudbox/internal/config"
	"cloudbox/internal/model"
	"cloudbox/internal/repository"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// RAGService handles document chunking, embedding, and indexing for RAG.
type RAGService struct {
	cfg          config.RAGConfig
	aiCfg        config.AIConfig
	embeddingCli *EmbeddingClient
	chatCli      *AIClient
	physicalRepo *repository.PhysicalFileRepository
	chunkRepo    *repository.ChunkRepository
	convRepo     *repository.ConversationRepository
	msgRepo      *repository.MessageRepository
	fileRepo     FileRepository
	storage      Storage
	encoder      *EmbeddingEncoder
	sem          chan struct{}
	wg           sync.WaitGroup
}

// EmbeddingEncoder converts float64 embeddings to/from binary blobs for DB storage.
type EmbeddingEncoder struct {
	dims int
}

// NewRAGService creates a new RAGService. Returns nil if RAG is not enabled or config is incomplete.
func NewRAGService(
	cfg config.RAGConfig,
	aiCfg config.AIConfig,
	physicalRepo *repository.PhysicalFileRepository,
	chunkRepo *repository.ChunkRepository,
	convRepo *repository.ConversationRepository,
	msgRepo *repository.MessageRepository,
	fileRepo FileRepository,
	storage Storage,
) *RAGService {
	if !cfg.Enabled {
		return nil
	}

	// Resolve embedding config with AI config fallback
	embeddingBaseURL := cfg.EmbeddingBaseURL
	if embeddingBaseURL == "" {
		embeddingBaseURL = aiCfg.BaseURL
	}
	embeddingAPIKey := cfg.EmbeddingAPIKey
	if embeddingAPIKey == "" {
		embeddingAPIKey = aiCfg.APIKey
	}

	if embeddingBaseURL == "" || embeddingAPIKey == "" || cfg.EmbeddingModel == "" {
		log.Println("[RAG] WARNING: embedding config incomplete (need base_url, api_key, model), RAG disabled")
		return nil
	}

	embeddingCli := NewEmbeddingClient(
		strings.TrimRight(embeddingBaseURL, "/"),
		embeddingAPIKey,
		cfg.EmbeddingModel,
		cfg.Timeout,
		cfg.EmbeddingDimensions,
	)

	// Resolve chat config with AI config fallback
	chatModel := cfg.ChatModel
	if chatModel == "" {
		chatModel = aiCfg.Model
	}
	chatBaseURL := cfg.ChatBaseURL
	if chatBaseURL == "" {
		chatBaseURL = aiCfg.BaseURL
	}
	chatAPIKey := cfg.ChatAPIKey
	if chatAPIKey == "" {
		chatAPIKey = aiCfg.APIKey
	}

	var chatCli *AIClient
	if chatModel != "" && chatBaseURL != "" {
		chatCli = NewAIClient(AIClientConfig{
			BaseURL: strings.TrimRight(chatBaseURL, "/"),
			APIKey:  chatAPIKey,
			Model:   chatModel,
			Timeout: cfg.Timeout,
		})
	}

	concurrent := cfg.MaxConcurrent
	if concurrent <= 0 {
		concurrent = 2
	}

	return &RAGService{
		cfg:          cfg,
		aiCfg:        aiCfg,
		embeddingCli: embeddingCli,
		chatCli:      chatCli,
		physicalRepo: physicalRepo,
		chunkRepo:    chunkRepo,
		convRepo:     convRepo,
		msgRepo:      msgRepo,
		fileRepo:     fileRepo,
		storage:      storage,
		encoder:      &EmbeddingEncoder{dims: embeddingCli.Dimensions()},
		sem:          make(chan struct{}, concurrent),
	}
}

// Shutdown waits for in-progress RAG tasks to complete (up to 30s).
func (s *RAGService) Shutdown() {
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
		log.Println("[RAG] Shutdown timed out waiting for tasks")
	}
}

// ProcessFile submits a file for async RAG indexing after upload.
func (s *RAGService) ProcessFile(physicalID int64, mimeType string) {
	if s == nil {
		return
	}

	if !isDocumentType(mimeType) {
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
			log.Printf("[RAG] Error finding physical file %d: %v", physicalID, err)
			return
		}

		if pf.ChunkCount > 0 {
			log.Printf("[RAG] Physical %d already indexed (%d chunks), skipping", physicalID, pf.ChunkCount)
			return
		}

		if err := s.indexFile(ctx, pf, mimeType); err != nil {
			log.Printf("[RAG] Error indexing physical %d: %v", physicalID, err)
		}
	}()
}

// indexFile extracts content from a physical file, chunks it, embeds it, and stores the chunks.
func (s *RAGService) indexFile(ctx context.Context, pf *model.PhysicalFile, mimeType string) error {
	text, err := s.extractContent(pf, mimeType)
	if err != nil {
		return fmt.Errorf("extract content: %w", err)
	}

	chunks := s.chunkText(text)
	if len(chunks) == 0 {
		log.Printf("[RAG] No chunks produced for physical %d", pf.ID)
		return nil
	}

	// Batch embed
	batchSize := s.cfg.EmbeddingBatchSize
	if batchSize <= 0 {
		batchSize = 20
	}

	var allEmbeddings [][]float64
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[i:end]

		embeddings, err := s.embeddingCli.Embed(ctx, batch)
		if err != nil {
			return fmt.Errorf("embed batch %d-%d: %w", i, end, err)
		}
		allEmbeddings = append(allEmbeddings, embeddings...)
	}

	now := time.Now()
	dbChunks := make([]model.DocumentChunk, len(chunks))
	for i, chunk := range chunks {
		dbChunks[i] = model.DocumentChunk{
			PhysicalFileID: pf.ID,
			ChunkIndex:     i,
			Content:        chunk,
			Embedding:      s.encoder.Encode(allEmbeddings[i]),
			TokenCount:     estimateTokens(chunk),
			CreatedAt:      now,
		}
	}

	return repository.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&dbChunks).Error; err != nil {
			return fmt.Errorf("insert chunks: %w", err)
		}
		if err := tx.Model(&model.PhysicalFile{}).Where("id = ?", pf.ID).Update("chunk_count", len(chunks)).Error; err != nil {
			return fmt.Errorf("update chunk_count: %w", err)
		}
		log.Printf("[RAG] Indexed physical %d: %d chunks", pf.ID, len(chunks))
		return nil
	})
}

// extractContent extracts text content from a physical file for RAG indexing.
func (s *RAGService) extractContent(pf *model.PhysicalFile, mimeType string) (string, error) {
	absPath := s.storage.ToAbsPath(pf.StoragePath)

	// Fallback: detect MIME type from extension if it's generic
	if mimeType == "application/octet-stream" || mimeType == "" {
		detected := detectMimeTypeFromPath(absPath)
		if detected != "application/octet-stream" {
			mimeType = detected
		}
	}

	if !isDocumentType(mimeType) {
		return "", fmt.Errorf("unsupported mime type for RAG: %s", mimeType)
	}

	if isPDFType(mimeType) {
		return extractPDFText(absPath)
	}

	return extractTextFile(absPath)
}

// chunkText splits text into chunks using semantic boundaries with a sliding-window fallback.
func (s *RAGService) chunkText(text string) []string {
	minSize := s.cfg.ChunkMinSize
	maxSize := s.cfg.ChunkMaxSize
	overlap := s.cfg.ChunkOverlap

	// Try paragraph splitting
	chunks := chunkByParagraphs(strings.Split(text, "\n\n"), minSize, maxSize, overlap)
	if len(chunks) > 1 {
		return chunks
	}

	// Try line splitting
	chunks = chunkByParagraphs(strings.Split(text, "\n"), minSize, maxSize, overlap)
	if len(chunks) > 1 {
		return chunks
	}

	return chunkSlidingWindow(text, minSize, maxSize, overlap)
}

// chunkByParagraphs merges small paragraphs into chunks respecting min/max size constraints.
func chunkByParagraphs(paragraphs []string, minSize, maxSize, overlap int) []string {
	var result []string
	var current strings.Builder

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		paragraphRunes := len([]rune(p))

		if current.Len() == 0 {
			current.WriteString(p)
			continue
		}

		// Would adding this paragraph exceed maxSize?
		proposedRunes := current.Len() + 1 + paragraphRunes
		if proposedRunes > maxSize {
			// Flush current chunk
			chunk := current.String()
			if len([]rune(chunk)) >= minSize || len(result) == 0 {
				result = append(result, chunk)
			} else if len(result) > 0 {
				result[len(result)-1] += "\n\n" + chunk
			}
			current.Reset()
			current.WriteString(p)
		} else {
			current.WriteString("\n\n")
			current.WriteString(p)
		}
	}

	if current.Len() > 0 {
		chunk := current.String()
		if len([]rune(chunk)) < minSize && len(result) > 0 {
			result[len(result)-1] += "\n\n" + chunk
		} else {
			result = append(result, chunk)
		}
	}

	// Apply overlap: prepend end of previous chunk to each subsequent chunk
	if overlap > 0 && len(result) > 1 {
		for i := 1; i < len(result); i++ {
			prev := result[i-1]
			prevRunes := []rune(prev)
			if len(prevRunes) > overlap {
				overlapText := string(prevRunes[len(prevRunes)-overlap:])
				result[i] = overlapText + "\n\n" + result[i]
			}
		}
	}

	return result
}

// chunkSlidingWindow splits text using a rune-based sliding window with overlap.
func chunkSlidingWindow(text string, minSize, maxSize, overlap int) []string {
	runes := []rune(text)
	totalLen := len(runes)

	if totalLen == 0 {
		return nil
	}

	if totalLen <= maxSize {
		return []string{text}
	}

	step := maxSize - overlap
	if step <= 0 {
		step = maxSize
	}

	var result []string
	for i := 0; i < totalLen; i += step {
		end := i + maxSize
		if end > totalLen {
			end = totalLen
		}
		result = append(result, string(runes[i:end]))
		if end == totalLen {
			break
		}
	}

	// Merge last chunk into previous if it's too small
	if len(result) > 1 {
		lastRunes := len([]rune(result[len(result)-1]))
		if lastRunes < minSize {
			prev := result[len(result)-2]
			result[len(result)-2] = prev + "\n\n" + result[len(result)-1]
			result = result[:len(result)-1]
		}
	}

	return result
}

// Encode converts a float64 embedding vector to a binary blob for database storage.
func (e *EmbeddingEncoder) Encode(vec []float64) []byte {
	buf := make([]byte, len(vec)*8)
	for i, v := range vec {
		binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(v))
	}
	return buf
}

// Decode converts a binary blob back to a float64 embedding vector.
func (e *EmbeddingEncoder) Decode(buf []byte) []float64 {
	if len(buf) == 0 || len(buf)%8 != 0 {
		return nil
	}
	vec := make([]float64, len(buf)/8)
	for i := range vec {
		vec[i] = math.Float64frombits(binary.LittleEndian.Uint64(buf[i*8:]))
	}
	return vec
}

// cosineSimilarity computes cosine similarity between two vectors.
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

// estimateTokens returns a rough token count for the given text (approx. 1.5 tokens per CJK rune).
func estimateTokens(text string) int {
	return len([]rune(text)) * 3 / 2
}
