// internal/handler/upload.go
package handler

import (
	"cloudbox/internal/service"
	"cloudbox/internal/util/response"
	"io"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type UploadHandler struct {
	uploadService *service.UploadService
}

func NewUploadHandler(uploadService *service.UploadService) *UploadHandler {
	return &UploadHandler{uploadService: uploadService}
}

// InitUpload godoc
// @Summary 初始化上传
// @Description 初始化分片上传，返回 uploadID 和分片大小
// @Tags upload
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body service.InitUploadRequest true "上传参数"
// @Success 200 {object} map[string]interface{} "初始化成功"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Router /api/upload/init [post]
func (h *UploadHandler) InitUpload(c *gin.Context) {
	userID := GetUserID(c)

	var req service.InitUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	resp, err := h.uploadService.InitUpload(c.Request.Context(), userID, req)
	if err != nil {
		if err == service.ErrQuotaExceeded {
			response.Error(c, 413, "存储空间不足，请清理文件或扩容")
			return
		}
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, resp)
}

// UploadChunk godoc
// @Summary 上传分片
// @Description 上传文件的单个分片
// @Tags upload
// @Accept multipart/form-data, application/octet-stream
// @Security BearerAuth
// @Param uploadID path string true "上传ID"
// @Param index path int true "分片索引"
// @Param chunk formData file false "分片文件"
// @Success 200 {object} map[string]interface{} "上传成功"
// @Failure 400 {object} map[string]interface{} "上传失败"
// @Router /api/upload/{uploadID}/chunk/{index} [put]
func (h *UploadHandler) UploadChunk(c *gin.Context) {
	uploadID := c.Param("uploadID")
	chunkIndex, err := strconv.Atoi(c.Param("index"))
	if err != nil || chunkIndex < 0 {
		response.BadRequest(c, "invalid chunk index")
		return
	}

	// Handle multipart/form-data (from frontend) or raw binary
	var reader io.Reader
	contentType := c.GetHeader("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Parse multipart form
		file, _, err := c.Request.FormFile("chunk")
		if err != nil {
			response.BadRequest(c, "failed to get chunk from form data")
			return
		}
		defer file.Close()
		reader = file
	} else {
		// Raw binary data
		reader = c.Request.Body
	}

	err = h.uploadService.SaveChunk(c.Request.Context(), uploadID, chunkIndex, reader)
	if err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, nil)
}

// GetProgress godoc
// @Summary 获取上传进度
// @Description 获取当前分片上传的进度
// @Tags upload
// @Produce json
// @Security BearerAuth
// @Param uploadID path string true "上传ID"
// @Success 200 {object} map[string]interface{} "上传进度"
// @Failure 404 {object} map[string]interface{} "上传不存在"
// @Router /api/upload/{uploadID}/progress [get]
func (h *UploadHandler) GetProgress(c *gin.Context) {
	uploadID := c.Param("uploadID")

	chunks, err := h.uploadService.GetProgress(c.Request.Context(), uploadID)
	if err != nil {
		if err == service.ErrUploadNotFound {
			response.NotFound(c, "upload not found")
			return
		}
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
		if err == service.ErrUploadNotFound {
			response.NotFound(c, "upload not found")
			return
		}
		if err == service.ErrChunkNotFound {
			response.Error(c, 400, "missing chunks")
			return
		}
		if err == service.ErrQuotaExceeded {
			response.Error(c, 413, "存储空间不足，请清理文件或扩容")
			return
		}
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, file)
}

func (h *UploadHandler) CancelUpload(c *gin.Context) {
	uploadID := c.Param("uploadID")

	err := h.uploadService.CancelUpload(c.Request.Context(), uploadID)
	if err != nil {
		if err == service.ErrUploadNotFound {
			response.NotFound(c, "upload not found")
			return
		}
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, nil)
}
