// internal/handler/chat.go
package handler

import (
	"cloudbox/internal/model"
	"cloudbox/internal/repository"
	"cloudbox/internal/service"
	"cloudbox/internal/util/response"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type ChatHandler struct {
	ragService  *service.RAGService
	fileService *service.FileService
}

func NewChatHandler(ragService *service.RAGService, fileService *service.FileService) *ChatHandler {
	return &ChatHandler{ragService: ragService, fileService: fileService}
}

type CreateConversationRequest struct {
	Title   string  `json:"title"`
	FileIDs []int64 `json:"fileIds"`
}

type AskRequest struct {
	Question string `json:"question" binding:"required"`
}

type AddFileRequest struct {
	FileID int64 `json:"fileId" binding:"required"`
}

// CreateConversation creates a new chat conversation.
func (h *ChatHandler) CreateConversation(c *gin.Context) {
	if h.ragService == nil {
		response.Error(c, 403, "RAG service is not enabled")
		return
	}

	var req CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body
		req = CreateConversationRequest{}
	}

	userID := GetUserID(c)

	fileIDsJSON := "[]"
	if len(req.FileIDs) > 0 {
		b, _ := json.Marshal(req.FileIDs)
		fileIDsJSON = string(b)
	}

	conv := &model.Conversation{
		UserID:    userID,
		Title:     req.Title,
		FileIDs:   fileIDsJSON,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := repository.DB.Create(conv).Error; err != nil {
		response.InternalError(c, "failed to create conversation")
		return
	}

	response.Success(c, conv)
}

// ListConversations lists user's conversations with pagination.
func (h *ChatHandler) ListConversations(c *gin.Context) {
	if h.ragService == nil {
		response.Error(c, 403, "RAG service is not enabled")
		return
	}

	userID := GetUserID(c)
	limit := 20
	offset := 0

	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	convRepo := repository.NewConversationRepository(repository.DB)
	convs, total, err := convRepo.ListByUser(c.Request.Context(), userID, limit, offset)
	if err != nil {
		response.InternalError(c, "failed to list conversations")
		return
	}

	response.Success(c, gin.H{
		"conversations": convs,
		"total":         total,
	})
}

// GetConversation returns a conversation with its messages.
func (h *ChatHandler) GetConversation(c *gin.Context) {
	if h.ragService == nil {
		response.Error(c, 403, "RAG service is not enabled")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid conversation ID")
		return
	}

	userID := GetUserID(c)
	convRepo := repository.NewConversationRepository(repository.DB)
	conv, err := convRepo.FindByIDAndUser(c.Request.Context(), id, userID)
	if err != nil {
		response.NotFound(c, "conversation not found")
		return
	}

	msgRepo := repository.NewMessageRepository(repository.DB)
	msgs, err := msgRepo.FindByConversation(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "failed to load messages")
		return
	}

	// Parse file IDs
	var fileIDs []int64
	if conv.FileIDs != "" && conv.FileIDs != "null" {
		json.Unmarshal([]byte(conv.FileIDs), &fileIDs)
	}

	response.Success(c, gin.H{
		"conversation": conv,
		"messages":     msgs,
		"fileIds":      fileIDs,
	})
}

// DeleteConversation deletes a conversation and its messages.
func (h *ChatHandler) DeleteConversation(c *gin.Context) {
	if h.ragService == nil {
		response.Error(c, 403, "RAG service is not enabled")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid conversation ID")
		return
	}

	userID := GetUserID(c)
	convRepo := repository.NewConversationRepository(repository.DB)
	conv, err := convRepo.FindByIDAndUser(c.Request.Context(), id, userID)
	if err != nil {
		response.NotFound(c, "conversation not found")
		return
	}

	msgRepo := repository.NewMessageRepository(repository.DB)
	msgRepo.DeleteByConversation(c.Request.Context(), conv.ID)
	convRepo.Delete(c.Request.Context(), conv.ID)

	response.Success(c, nil)
}

// AddFile adds a file to an existing conversation.
func (h *ChatHandler) AddFile(c *gin.Context) {
	if h.ragService == nil {
		response.Error(c, 403, "RAG service is not enabled")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid conversation ID")
		return
	}

	var req AddFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	userID := GetUserID(c)
	convRepo := repository.NewConversationRepository(repository.DB)
	conv, err := convRepo.FindByIDAndUser(c.Request.Context(), id, userID)
	if err != nil {
		response.NotFound(c, "conversation not found")
		return
	}

	// Validate file belongs to user and is indexed
	file, err := h.fileService.GetFile(c.Request.Context(), userID, req.FileID)
	if err != nil {
		response.NotFound(c, "file not found")
		return
	}
	if file.Physical == nil || file.Physical.ChunkCount == 0 {
		response.BadRequest(c, "file has not been indexed yet")
		return
	}

	// Parse existing file IDs
	var fileIDs []int64
	if conv.FileIDs != "" && conv.FileIDs != "null" {
		json.Unmarshal([]byte(conv.FileIDs), &fileIDs)
	}

	// Check if already added
	for _, fid := range fileIDs {
		if fid == req.FileID {
			response.Success(c, conv)
			return
		}
	}

	fileIDs = append(fileIDs, req.FileID)
	b, _ := json.Marshal(fileIDs)
	conv.FileIDs = string(b)
	conv.UpdatedAt = time.Now()
	convRepo.Update(c.Request.Context(), conv)

	response.Success(c, conv)
}

// Ask sends a question and streams the answer via SSE.
func (h *ChatHandler) Ask(c *gin.Context) {
	if h.ragService == nil {
		response.Error(c, 403, "RAG service is not enabled")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid conversation ID")
		return
	}

	var req AskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "question is required")
		return
	}

	if req.Question == "" {
		response.BadRequest(c, "question is required")
		return
	}

	userID := GetUserID(c)

	// Verify conversation exists and belongs to user
	convRepo := repository.NewConversationRepository(repository.DB)
	conv, err := convRepo.FindByIDAndUser(c.Request.Context(), id, userID)
	if err != nil {
		response.NotFound(c, "conversation not found")
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Save user message first
	msgRepo := repository.NewMessageRepository(repository.DB)
	userMsg := &model.Message{
		ConversationID: id,
		Role:           "user",
		Content:        req.Question,
		CreatedAt:      time.Now(),
	}
	msgRepo.Create(c.Request.Context(), userMsg)

	// Update title if first message
	msgCount, _ := msgRepo.CountByConversation(c.Request.Context(), id)
	if msgCount <= 1 {
		title := req.Question
		runes := []rune(title)
		if len(runes) > 30 {
			title = string(runes[:30])
		}
		conv.Title = title
		conv.UpdatedAt = time.Now()
		convRepo.Update(c.Request.Context(), conv)
	}

	// Create cancel context for client disconnect
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// Monitor client disconnect
	go func() {
		<-c.Request.Context().Done()
		cancel()
	}()

	// Stream callback
	callback := func(content string) error {
		data := fmt.Sprintf("event: token\ndata: %s\n\n", mustJSON(gin.H{"content": content}))
		_, err := c.Writer.Write([]byte(data))
		if err != nil {
			return err
		}
		c.Writer.Flush()
		return nil
	}

	// Run RAG
	fullContent, sources, err := h.ragService.Ask(ctx, id, userID, req.Question, callback)

	// Save assistant message
	var sourcesJSON string
	if len(sources) > 0 {
		b, _ := json.Marshal(sources)
		sourcesJSON = string(b)
	}

	if err != nil && fullContent == "" {
		// Complete failure — send error event
		data := fmt.Sprintf("event: error\ndata: %s\n\n", mustJSON(gin.H{"error": err.Error()}))
		c.Writer.Write([]byte(data))
		c.Writer.Flush()
		return
	}

	assistantMsg := &model.Message{
		ConversationID: id,
		Role:           "assistant",
		Content:        fullContent,
		Sources:        sourcesJSON,
		CreatedAt:      time.Now(),
	}
	msgRepo.Create(c.Request.Context(), assistantMsg)

	conv.UpdatedAt = time.Now()
	convRepo.Update(c.Request.Context(), conv)

	// Send done event
	doneData := mustJSON(gin.H{
		"messageId": assistantMsg.ID,
		"sources":   sources,
	})
	c.Writer.Write([]byte(fmt.Sprintf("event: done\ndata: %s\n\n", doneData)))
	c.Writer.Flush()
}

// ReindexFile triggers re-indexing of a file for RAG.
func (h *ChatHandler) ReindexFile(c *gin.Context) {
	if h.ragService == nil {
		response.Error(c, 403, "RAG service is not enabled")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid file ID")
		return
	}

	userID := GetUserID(c)
	role, _ := c.Get("role")
	file, err := h.fileService.GetFile(c.Request.Context(), userID, id)
	if err != nil {
		response.NotFound(c, "file not found")
		return
	}

	if file.OwnerID != userID && role != "admin" {
		response.Forbidden(c, "not authorized")
		return
	}

	if file.Physical == nil {
		response.BadRequest(c, "file has no physical content")
		return
	}

	go func() {
		if err := h.ragService.Reindex(context.Background(), file.ContentRef, file.Physical.MimeType); err != nil {
			log.Printf("[RAG] Reindex failed for file %d (physical %d): %v", id, file.ContentRef, err)
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{"status": "processing"})
}

// AskSSE is a GET endpoint for SSE streaming (compatible with EventSource).
func (h *ChatHandler) AskSSE(c *gin.Context) {
	if h.ragService == nil {
		c.SSEvent("error", gin.H{"error": "RAG service is not enabled"})
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.SSEvent("error", gin.H{"error": "invalid conversation ID"})
		return
	}

	question := c.Query("q")
	if question == "" {
		c.SSEvent("error", gin.H{"error": "question is required"})
		return
	}

	userID := GetUserID(c)

	convRepo := repository.NewConversationRepository(repository.DB)
	conv, err := convRepo.FindByIDAndUser(c.Request.Context(), id, userID)
	if err != nil {
		c.SSEvent("error", gin.H{"error": "conversation not found"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	msgRepo := repository.NewMessageRepository(repository.DB)
	userMsg := &model.Message{
		ConversationID: id,
		Role:           "user",
		Content:        question,
		CreatedAt:      time.Now(),
	}
	msgRepo.Create(c.Request.Context(), userMsg)

	msgCount, _ := msgRepo.CountByConversation(c.Request.Context(), id)
	if msgCount <= 1 {
		title := question
		runes := []rune(title)
		if len(runes) > 30 {
			title = string(runes[:30])
		}
		conv.Title = title
		conv.UpdatedAt = time.Now()
		convRepo.Update(c.Request.Context(), conv)
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	go func() {
		<-c.Request.Context().Done()
		cancel()
	}()

	callback := func(content string) error {
		fmt.Fprintf(c.Writer, "data: %s\n\n", mustJSON(gin.H{"content": content}))
		c.Writer.Flush()
		return nil
	}

	fullContent, sources, err := h.ragService.Ask(ctx, id, userID, question, callback)

	var sourcesJSON string
	if len(sources) > 0 {
		b, _ := json.Marshal(sources)
		sourcesJSON = string(b)
	}

	if err != nil && fullContent == "" {
		fmt.Fprintf(c.Writer, "data: %s\n\n", mustJSON(gin.H{"error": err.Error()}))
		c.Writer.Flush()
		return
	}

	assistantMsg := &model.Message{
		ConversationID: id,
		Role:           "assistant",
		Content:        fullContent,
		Sources:        sourcesJSON,
		CreatedAt:      time.Now(),
	}
	msgRepo.Create(c.Request.Context(), assistantMsg)

	conv.UpdatedAt = time.Now()
	convRepo.Update(c.Request.Context(), conv)

	fmt.Fprintf(c.Writer, "data: %s\n\n", mustJSON(gin.H{"messageId": assistantMsg.ID, "sources": sources}))
	c.Writer.Flush()
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
