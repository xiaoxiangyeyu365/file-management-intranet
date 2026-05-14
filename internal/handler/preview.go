// internal/handler/preview.go
package handler

import (
	"cloudbox/internal/service"
	"cloudbox/internal/util/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type PreviewHandler struct {
	previewService *service.PreviewService
	fileService    *service.FileService
}

func NewPreviewHandler(previewService *service.PreviewService, fileService *service.FileService) *PreviewHandler {
	return &PreviewHandler{
		previewService: previewService,
		fileService:    fileService,
	}
}

func (h *PreviewHandler) GetMetadata(c *gin.Context) {
	userID := GetUserID(c)
	fileID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid file id")
		return
	}

	metadata, err := h.previewService.GetMetadata(c.Request.Context(), userID, fileID)
	if err != nil {
		if err == service.ErrFileNotFound {
			response.NotFound(c, "file not found")
			return
		}
		if err == service.ErrNotImage {
			response.Error(c, 400, "file is not an image")
			return
		}
		response.InternalError(c, "failed to get metadata")
		return
	}

	response.Success(c, metadata)
}
