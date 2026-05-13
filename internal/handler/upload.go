// internal/handler/upload.go
package handler

import (
	"cloudbox/internal/service"
	"cloudbox/internal/util/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type UploadHandler struct {
	uploadService *service.UploadService
}

func NewUploadHandler(uploadService *service.UploadService) *UploadHandler {
	return &UploadHandler{uploadService: uploadService}
}

func (h *UploadHandler) InitUpload(c *gin.Context) {
	userID := GetUserID(c)

	var req service.InitUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	resp, err := h.uploadService.InitUpload(c.Request.Context(), userID, req)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, resp)
}

func (h *UploadHandler) UploadChunk(c *gin.Context) {
	uploadID := c.Param("uploadID")
	chunkIndex, _ := strconv.Atoi(c.Param("index"))

	err := h.uploadService.SaveChunk(c.Request.Context(), uploadID, chunkIndex, c.Request.Body)
	if err != nil {
		response.Error(c, 400, "failed to save chunk")
		return
	}

	response.Success(c, nil)
}

func (h *UploadHandler) GetProgress(c *gin.Context) {
	uploadID := c.Param("uploadID")

	chunks, err := h.uploadService.GetProgress(c.Request.Context(), uploadID)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, gin.H{
		"uploadedChunks": chunks,
	})
}

func (h *UploadHandler) CompleteUpload(c *gin.Context) {
	userID := GetUserID(c)
	uploadID := c.Param("uploadID")

	var req service.CompleteUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	file, err := h.uploadService.CompleteUpload(c.Request.Context(), userID, uploadID, req)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, file)
}

func (h *UploadHandler) CancelUpload(c *gin.Context) {
	uploadID := c.Param("uploadID")

	err := h.uploadService.CancelUpload(c.Request.Context(), uploadID)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, nil)
}
