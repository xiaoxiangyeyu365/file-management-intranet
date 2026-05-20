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

// ListTrash godoc
// @Summary 获取回收站列表
// @Description 获取当前用户回收站中的文件列表
// @Tags trash
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{} "回收站文件列表"
// @Router /api/trash [get]
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

// RestoreFile godoc
// @Summary 恢复文件
// @Description 从回收站恢复指定文件到原位置
// @Tags trash
// @Security BearerAuth
// @Param id path int true "文件ID"
// @Success 200 {object} map[string]interface{} "恢复成功"
// @Failure 404 {object} map[string]interface{} "文件不存在"
// @Router /api/trash/{id}/restore [post]
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

// PermanentDelete godoc
// @Summary 永久删除
// @Description 永久删除回收站中的文件，无法恢复
// @Tags trash
// @Security BearerAuth
// @Param id path int true "文件ID"
// @Success 200 {object} map[string]interface{} "删除成功"
// @Failure 404 {object} map[string]interface{} "文件不存在"
// @Router /api/trash/{id} [delete]
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

// EmptyTrash godoc
// @Summary 清空回收站
// @Description 清空当前用户回收站中的所有文件
// @Tags trash
// @Security BearerAuth
// @Success 200 {object} map[string]interface{} "清空成功"
// @Router /api/trash [delete]
func (h *TrashHandler) EmptyTrash(c *gin.Context) {
	userID := GetUserID(c)

	count, err := h.fileService.EmptyTrash(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "failed to empty trash")
		return
	}

	response.Success(c, gin.H{
		"deletedCount": count,
	})
}
