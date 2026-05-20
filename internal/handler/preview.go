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

// GetMetadata godoc
// @Summary 获取文件元数据
// @Description 获取图片文件的元数据信息
// @Tags preview
// @Produce json
// @Security BearerAuth
// @Param id path int true "文件ID"
// @Success 200 {object} map[string]interface{} "文件元数据"
// @Failure 400 {object} map[string]interface{} "不是图片文件"
// @Failure 404 {object} map[string]interface{} "文件不存在"
// @Router /api/files/{id}/metadata [get]
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
