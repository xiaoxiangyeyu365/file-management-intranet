// internal/handler/trash.go
package handler

import (
	"cloudbox/internal/service"
	"cloudbox/internal/util/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type TrashHandler struct {
	fileService *service.FileService
}

func NewTrashHandler(fileService *service.FileService) *TrashHandler {
	return &TrashHandler{fileService: fileService}
}

func (h *TrashHandler) ListTrash(c *gin.Context) {
	userID := GetUserID(c)

	files, err := h.fileService.ListTrash(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "failed to list trash")
		return
	}

	response.Success(c, gin.H{
		"files": files,
	})
}

func (h *TrashHandler) RestoreFile(c *gin.Context) {
	userID := GetUserID(c)
	fileID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	err := h.fileService.RestoreFile(c.Request.Context(), userID, fileID)
	if err != nil {
		if err == service.ErrFileNotFound {
			response.NotFound(c, "file not found")
			return
		}
		response.InternalError(c, "failed to restore")
		return
	}

	response.Success(c, nil)
}

func (h *TrashHandler) PermanentDelete(c *gin.Context) {
	userID := GetUserID(c)
	fileID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	err := h.fileService.PermanentDelete(c.Request.Context(), userID, fileID)
	if err != nil {
		if err == service.ErrFileNotFound {
			response.NotFound(c, "file not found")
			return
		}
		response.InternalError(c, "failed to delete permanently")
		return
	}

	response.Success(c, nil)
}
