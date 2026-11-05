package service

import (
	"cloudbox/internal/config"
	"cloudbox/internal/model"
	"cloudbox/internal/repository"
	"context"
	"encoding/binary"
	"encoding/json"
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
	} else {
		log.Println("[RAG] WARNING: chat config incomplete (chat_model, chat_base_url required for Ask), RAG chat disabled")
		return nil
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

	if !isRAGSupported(mimeType) {
		return "", fmt.Errorf("unsupported mime type for RAG: %s", mimeType)
	}

	if isPDFType(mimeType) {
		return extractPDFText(absPath)
	}

	if isDOCXType(mimeType) {
		return extractDOCXText(absPath)
	}

	return extractTextFile(absPath)
}

// chunkText splits text into chunks using semantic boundaries with a sliding-window fallback.
func (s *RAGService) chunkText(text string) []string {
	minSize := s.cfg.ChunkMinSize
	maxSize := s.cfg.ChunkMaxSize
	overlap := s.cfg.ChunkOverlap

	if maxSize <= 0 {
		maxSize = 500
	}
	if minSize < 0 {
		minSize = 0
	}
	if minSize > maxSize {
		minSize = maxSize
	}
	if overlap < 0 || overlap >= maxSize {
		overlap = 0
	}

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
// All size comparisons are in runes, not bytes, so CJK text is chunked at the configured size.
func chunkByParagraphs(paragraphs []string, minSize, maxSize, overlap int) []string {
	var result []string
	var current strings.Builder
	currentRunes := 0

	flush := func() {
		if current.Len() == 0 {
			return
		}
		chunk := current.String()
		if len([]rune(chunk)) >= minSize || len(result) == 0 {
			result = append(result, chunk)
		} else {
			result[len(result)-1] += "\n\n" + chunk
		}
		current.Reset()
		currentRunes = 0
	}

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		paragraphRunes := len([]rune(p))

		// A single paragraph larger than maxSize must be split on its own.
		if paragraphRunes > maxSize {
			flush()
			result = append(result, chunkSlidingWindow(p, minSize, maxSize, 0)...)
			continue
		}

		if currentRunes == 0 {
			current.WriteString(p)
			currentRunes = paragraphRunes
			continue
		}

		// Would adding this paragraph exceed maxSize?
		proposedRunes := currentRunes + 2 + paragraphRunes
		if proposedRunes > maxSize {
			flush()
			current.WriteString(p)
			currentRunes = paragraphRunes
		} else {
			current.WriteString("\n\n")
			current.WriteString(p)
			currentRunes = proposedRunes
		}
	}
	flush()

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

// ChatSource represents a citation in an AI response.
type ChatSource struct {
	ChunkID    int64   `json:"chunkId"`
	FileID     int64   `json:"fileId"`
	FileName   string  `json:"fileName"`
	Preview    string  `json:"preview"`
	Score      float64 `json:"score"`
	ChunkIndex int     `json:"chunkIndex"`
}

const (
	noRelevantAnswer   = "根据当前对话关联的文档，没有找到与该问题足够相关的内容。可以尝试换个问法，或添加更相关的文件。"
	noIndexedFilesHint = "这个对话还没有关联已索引的文件。请先在文件详情中重建索引，或添加其他已索引文件。"
)

// chatFileRef maps a physical file back to one of the user's virtual files.
type chatFileRef struct {
	ID   int64
	Name string
}

// Reindex forces re-indexing of a file (delete existing chunks, re-index).
func (s *RAGService) Reindex(ctx context.Context, physicalID int64, mimeType string) error {
	if s == nil {
		return fmt.Errorf("RAG service is not enabled")
	}

	// Delete existing chunks
	if err := s.chunkRepo.DeleteByPhysicalFileID(ctx, physicalID); err != nil {
		return fmt.Errorf("delete old chunks: %w", err)
	}
	if err := s.chunkRepo.UpdateChunkCount(ctx, physicalID, 0); err != nil {
		return fmt.Errorf("reset chunk count: %w", err)
	}

	pf, err := s.physicalRepo.FindByID(ctx, physicalID)
	if err != nil {
		return fmt.Errorf("find physical file: %w", err)
	}

	return s.indexFile(ctx, pf, mimeType)
}

// IsIndexable reports whether the RAG pipeline supports this file type.
func IsIndexable(mimeType, storagePath string) bool {
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = detectMimeTypeFromPath(storagePath)
	}
	return isRAGSupported(mimeType)
}

func isRAGSupported(mimeType string) bool {
	return strings.HasPrefix(mimeType, "text/") || isPDFType(mimeType) || isDOCXType(mimeType)
}

// IndexStats summarizes the knowledge base state for the admin page.
type IndexStats struct {
	TotalPhysical      int   `json:"totalPhysical"`
	IndexedFiles       int   `json:"indexedFiles"`
	UnindexedSupported int   `json:"unindexedSupported"`
	TotalChunks        int64 `json:"totalChunks"`
	SummarizedFiles    int   `json:"summarizedFiles"`
}

// Stats returns aggregate indexing/summary counts.
func (s *RAGService) Stats(ctx context.Context) (*IndexStats, error) {
	if s == nil {
		return nil, fmt.Errorf("RAG service is not enabled")
	}

	var files []model.PhysicalFile
	if err := repository.DB.WithContext(ctx).Find(&files).Error; err != nil {
		return nil, err
	}

	stats := &IndexStats{TotalPhysical: len(files)}
	for _, pf := range files {
		stats.TotalChunks += int64(pf.ChunkCount)
		if pf.ChunkCount > 0 {
			stats.IndexedFiles++
		} else if IsIndexable(pf.MimeType, pf.StoragePath) {
			stats.UnindexedSupported++
		}
		if pf.Summary != "" {
			stats.SummarizedFiles++
		}
	}
	return stats, nil
}

// ReindexAllUnindexed queues every supported physical file that has no chunks.
// The actual embedding work runs in the background through the shared semaphore.
func (s *RAGService) ReindexAllUnindexed(ctx context.Context) (int, error) {
	if s == nil {
		return 0, fmt.Errorf("RAG service is not enabled")
	}

	var files []model.PhysicalFile
	if err := repository.DB.WithContext(ctx).Where("chunk_count = 0").Find(&files).Error; err != nil {
		return 0, err
	}

	queued := 0
	for _, pf := range files {
		if !IsIndexable(pf.MimeType, pf.StoragePath) {
			continue
		}
		pf := pf
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.sem <- struct{}{}
			defer func() { <-s.sem }()

			if err := s.Reindex(context.Background(), pf.ID, pf.MimeType); err != nil {
				log.Printf("[RAG] Bulk reindex failed for physical %d (%s): %v", pf.ID, pf.StoragePath, err)
			}
		}()
		queued++
	}

	log.Printf("[RAG] Queued %d unindexed file(s) for indexing", queued)
	return queued, nil
}

// Ask performs RAG Q&A with streaming output. Returns the full response text and sources.
func (s *RAGService) Ask(ctx context.Context, conversationID, userID int64, question string, callback StreamCallback) (string, []ChatSource, error) {
	// 1. Get conversation
	conv, err := s.convRepo.FindByIDAndUser(ctx, conversationID, userID)
	if err != nil {
		return "", nil, fmt.Errorf("conversation not found: %w", err)
	}

	// 2. Resolve file IDs to physical file IDs
	physicalIDs, fileRefs, err := s.resolvePhysicalIDs(ctx, conv, userID)
	if err != nil {
		return "", nil, fmt.Errorf("resolve file IDs: %w", err)
	}

	if len(physicalIDs) == 0 {
		if callback != nil {
			_ = callback(noIndexedFilesHint)
		}
		return noIndexedFilesHint, nil, nil
	}

	// 3. Recent history: used both for query condensation and the prompt.
	maxHistory := s.cfg.MaxHistory
	if maxHistory <= 0 {
		maxHistory = 10
	}
	recentMsgs, _ := s.msgRepo.FindRecentByConversation(ctx, conversationID, maxHistory*2)
	recentMsgs = trimCurrentQuestion(recentMsgs, question)

	searchQuery := buildSearchQuery(question, recentMsgs)
	log.Printf("[RAG] search query: %s", truncatePreview(searchQuery, 120))

	// 4. Query embedding
	queryEmbeddings, err := s.embeddingCli.Embed(ctx, []string{searchQuery})
	if err != nil {
		return "", nil, fmt.Errorf("embed query: %w", err)
	}
	queryVec := queryEmbeddings[0]

	// 5. Retrieve relevant chunks
	topChunks, err := s.retrieveChunks(ctx, physicalIDs, queryVec, s.cfg.TopK)
	if err != nil {
		return "", nil, fmt.Errorf("retrieve chunks: %w", err)
	}

	if len(topChunks) == 0 {
		if callback != nil {
			_ = callback(noRelevantAnswer)
		}
		return noRelevantAnswer, nil, nil
	}

	// 6. Build sources
	sources := make([]ChatSource, 0, len(topChunks))
	for _, tc := range topChunks {
		ref := fileRefs[tc.chunk.PhysicalFileID]
		sources = append(sources, ChatSource{
			ChunkID:    tc.chunk.ID,
			FileID:     ref.ID,
			FileName:   ref.Name,
			Preview:    truncatePreview(tc.chunk.Content, 240),
			Score:      tc.score,
			ChunkIndex: tc.chunk.ChunkIndex,
		})
	}

	// 7. Build messages
	messages := s.buildMessages(question, topChunks, fileRefs, recentMsgs)

	// 8. Stream chat completion
	fullContent, err := s.chatCli.ChatCompletionStream(ctx, messages, callback)
	if err != nil {
		return fullContent, sources, err
	}

	return fullContent, sources, nil
}

type scoredChunk struct {
	chunk model.DocumentChunk
	score float64
}

// retrieveChunks finds top-K chunks by cosine similarity.
func (s *RAGService) retrieveChunks(ctx context.Context, physicalIDs []int64, queryVec []float64, topK int) ([]scoredChunk, error) {
	chunks, err := s.chunkRepo.FindByPhysicalFileIDs(ctx, physicalIDs)
	if err != nil {
		return nil, err
	}

	// Score all chunks
	scored := make([]scoredChunk, 0, len(chunks))
	for _, chunk := range chunks {
		if len(chunk.Embedding) == 0 {
			continue
		}
		emb := s.encoder.Decode(chunk.Embedding)
		score := cosineSimilarity(queryVec, emb)
		scored = append(scored, scoredChunk{chunk: chunk, score: score})
	}

	// Drop chunks that are not similar enough to the question. Without this
	// the model always receives topK chunks, even for unrelated questions.
	totalChunks := len(scored)
	scored = filterChunksByScore(scored, s.cfg.MinScore)

	// Sort by score descending
	sortChunksByScore(scored)

	// Return top K
	if len(scored) > topK {
		scored = scored[:topK]
	}

	if len(scored) > 0 {
		log.Printf("[RAG] retrieved %d/%d chunks above min_score=%.2f (top=%.3f)", len(scored), totalChunks, s.cfg.MinScore, scored[0].score)
	} else {
		log.Printf("[RAG] no chunks above min_score=%.2f (candidates=%d)", s.cfg.MinScore, totalChunks)
	}

	return scored, nil
}

func filterChunksByScore(chunks []scoredChunk, minScore float64) []scoredChunk {
	if minScore <= 0 {
		return chunks
	}
	filtered := make([]scoredChunk, 0, len(chunks))
	for _, c := range chunks {
		if c.score >= minScore {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func sortChunksByScore(chunks []scoredChunk) {
	// Insertion sort (topK is small)
	for i := 1; i < len(chunks); i++ {
		key := chunks[i]
		j := i - 1
		for j >= 0 && chunks[j].score < key.score {
			chunks[j+1] = chunks[j]
			j--
		}
		chunks[j+1] = key
	}
}

// resolvePhysicalIDs converts conversation file_ids to physical file IDs and
// keeps a mapping back to the user's virtual file for citations.
func (s *RAGService) resolvePhysicalIDs(ctx context.Context, conv *model.Conversation, userID int64) ([]int64, map[int64]chatFileRef, error) {
	if conv.Scope == "all" {
		files, err := s.fileRepo.FindAllByOwner(ctx, userID)
		if err != nil {
			return nil, nil, fmt.Errorf("list indexed files: %w", err)
		}

		physicalIDs := make([]int64, 0, len(files))
		fileRefs := make(map[int64]chatFileRef)
		for _, file := range files {
			if file.IsFolder || file.Physical == nil || file.Physical.ChunkCount == 0 {
				continue
			}
			physicalIDs = append(physicalIDs, file.ContentRef)
			if _, exists := fileRefs[file.ContentRef]; !exists {
				fileRefs[file.ContentRef] = chatFileRef{ID: file.ID, Name: file.Name}
			}
		}
		return physicalIDs, fileRefs, nil
	}

	var fileIDs []int64
	if conv.FileIDs != "" && conv.FileIDs != "null" {
		if err := json.Unmarshal([]byte(conv.FileIDs), &fileIDs); err != nil {
			return nil, nil, fmt.Errorf("parse file_ids: %w", err)
		}
	}

	if len(fileIDs) == 0 {
		return nil, nil, nil
	}

	physicalIDs := make([]int64, 0, len(fileIDs))
	fileRefs := make(map[int64]chatFileRef) // physicalID -> virtual file
	for _, fid := range fileIDs {
		file, err := s.fileRepo.FindByIDAndOwner(ctx, fid, userID)
		if err != nil {
			continue // skip inaccessible files
		}
		if file.Physical == nil || file.Physical.ChunkCount == 0 {
			continue // skip unindexed files
		}
		physicalIDs = append(physicalIDs, file.ContentRef)
		if _, exists := fileRefs[file.ContentRef]; !exists {
			fileRefs[file.ContentRef] = chatFileRef{ID: file.ID, Name: file.Name}
		}
	}

	return physicalIDs, fileRefs, nil
}

// buildMessages constructs the LLM message array for RAG.
func (s *RAGService) buildMessages(question string, topChunks []scoredChunk, fileRefs map[int64]chatFileRef, recentMsgs []model.Message) []chatMessage {
	var messages []chatMessage

	// System message with retrieved context
	var contextBuilder strings.Builder
	contextBuilder.WriteString(s.cfg.SystemPrompt)
	contextBuilder.WriteString("\n\n---\n以下是与问题相关的文档内容：\n\n")
	for i, tc := range topChunks {
		ref := fileRefs[tc.chunk.PhysicalFileID]
		if ref.Name != "" {
			contextBuilder.WriteString(fmt.Sprintf("[文档片段 %d | 文件: %s]\n%s\n\n", i+1, ref.Name, tc.chunk.Content))
		} else {
			contextBuilder.WriteString(fmt.Sprintf("[文档片段 %d]\n%s\n\n", i+1, tc.chunk.Content))
		}
	}
	contextBuilder.WriteString("\n引用要求：请在回答中用到资料时，于相关句子后用 [1]、[2] 标注对应的文档片段编号；只能引用上面出现过的编号，不要编造。")
	contextBuilder.WriteString("如果资料不足以回答，请明确说明缺少什么，不要凭空推测。回答使用 Markdown 组织。\n")

	messages = append(messages, chatMessage{
		Role:    "system",
		Content: contextBuilder.String(),
	})

	// Recent conversation history
	for _, msg := range recentMsgs {
		messages = append(messages, chatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Current question
	messages = append(messages, chatMessage{
		Role:    "user",
		Content: question,
	})

	return messages
}

// trimCurrentQuestion removes the just-saved user message from the history
// slice so it is not sent to the model twice.
func trimCurrentQuestion(msgs []model.Message, question string) []model.Message {
	if n := len(msgs); n > 0 &&
		msgs[n-1].Role == "user" &&
		strings.TrimSpace(msgs[n-1].Content) == strings.TrimSpace(question) {
		return msgs[:n-1]
	}
	return msgs
}

// buildSearchQuery adds just enough conversation context for follow-up
// questions ("那第二个呢") without biasing unrelated new topics.
func buildSearchQuery(question string, recent []model.Message) string {
	if !needsConversationContext(question) {
		return question
	}

	previous := ""
	for i := len(recent) - 1; i >= 0; i-- {
		if recent[i].Role != "user" {
			continue
		}
		content := strings.TrimSpace(recent[i].Content)
		if content == "" || content == question {
			continue
		}
		previous = content
		break
	}
	if previous == "" {
		return question
	}

	combined := previous + " " + question
	runes := []rune(combined)
	if len(runes) > 400 {
		combined = string(runes[len(runes)-400:])
	}
	return combined
}

func needsConversationContext(question string) bool {
	if len([]rune(question)) <= 30 {
		return true
	}
	markers := []string{
		"它", "他", "她", "这个", "那个", "这些", "那些", "上述", "前面", "刚才",
		"继续", "再", "第二", "第三", "呢",
	}
	for _, m := range markers {
		if strings.Contains(question, m) {
			return true
		}
	}
	return false
}

func truncatePreview(text string, maxLen int) string {
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen]) + "..."
}
