package handler

import (
	"cloudbox/internal/model"
	"cloudbox/internal/service"
	"cloudbox/internal/util/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ClipboardHandler struct {
	service *service.ClipboardService
}

func NewClipboardHandler(s *service.ClipboardService) *ClipboardHandler {
	return &ClipboardHandler{service: s}
}

type CreateClipboardRequest struct {
	Content    string `json:"content" binding:"required"`
	DeviceName string `json:"deviceName"`
}

type UpdatePinRequest struct {
	Pinned bool `json:"pinned"`
}

// List clipboard records
// @Summary 获取剪切板记录
// @Description 获取当前用户的云剪切板记录列表
// @Tags clipboard
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{} "剪切板记录列表"
// @Router /api/clipboard [get]
func (h *ClipboardHandler) List(c *gin.Context) {
	userID := GetUserID(c)

	records, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		response.InternalError(c, "failed to list clipboard records")
		return
	}

	responses := make([]*model.ClipboardResponse, len(records))
	for i := range records {
		responses[i] = records[i].ToResponse()
	}

	response.Success(c, gin.H{
		"records": responses,
	})
}

// Create clipboard record
// @Summary 保存到剪切板
// @Description 将文本内容保存到云剪切板
// @Tags clipboard
// @Accept json
// @Security BearerAuth
// @Param request body CreateClipboardRequest true "剪切板内容"
// @Param X-Device-Name header string false "设备名称"
// @Success 200 {object} map[string]interface{} "保存成功"
// @Failure 400 {object} map[string]interface{} "内容为空或超过限制"
// @Router /api/clipboard [post]
func (h *ClipboardHandler) Create(c *gin.Context) {
	userID := GetUserID(c)

	var req CreateClipboardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "content is required")
		return
	}

	deviceName := c.GetHeader("X-Device-Name")
	if deviceName == "" {
		deviceName = req.DeviceName
	}
	if deviceName == "" {
		deviceName = "未命名设备"
	}

	record, err := h.service.Create(c.Request.Context(), service.CreateClipboardRequest{
		Content:    req.Content,
		DeviceName: deviceName,
		UserID:     userID,
	})
	if err != nil {
		if err == service.ErrClipboardEmpty {
			response.BadRequest(c, "content is empty")
			return
		}
		if err == service.ErrClipboardTooLong {
			response.BadRequest(c, "content exceeds 10KB limit")
			return
		}
		response.InternalError(c, "failed to create clipboard record")
		return
	}

	response.Success(c, record.ToResponse())
}

// UpdatePin clipboard record
// @Summary 设置置顶
// @Description 设置或取消剪切板记录的置顶状态
// @Tags clipboard
// @Accept json
// @Security BearerAuth
// @Param id path int true "记录ID"
// @Param request body UpdatePinRequest true "置顶状态"
// @Success 200 {object} map[string]interface{} "操作成功"
// @Failure 404 {object} map[string]interface{} "记录不存在"
// @Router /api/clipboard/{id}/pin [patch]
func (h *ClipboardHandler) UpdatePin(c *gin.Context) {
	userID := GetUserID(c)
	recordID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	var req UpdatePinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	err := h.service.TogglePin(c.Request.Context(), userID, recordID, req.Pinned)
	if err != nil {
		if err == service.ErrClipboardNotFound {
			response.NotFound(c, "clipboard record not found")
			return
		}
		response.InternalError(c, "failed to update pin status")
		return
	}

	response.Success(c, nil)
}

// Delete clipboard record
// @Summary 删除记录
// @Description 删除指定的剪切板记录
// @Tags clipboard
// @Security BearerAuth
// @Param id path int true "记录ID"
// @Success 200 {object} map[string]interface{} "删除成功"
// @Failure 404 {object} map[string]interface{} "记录不存在"
// @Router /api/clipboard/{id} [delete]
func (h *ClipboardHandler) Delete(c *gin.Context) {
	userID := GetUserID(c)
	recordID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	err := h.service.Delete(c.Request.Context(), userID, recordID)
	if err != nil {
		if err == service.ErrClipboardNotFound {
			response.NotFound(c, "clipboard record not found")
			return
		}
		response.InternalError(c, "failed to delete clipboard record")
		return
	}

	response.Success(c, nil)
}

// Clear clipboard records
// @Summary 清空剪切板
// @Description 清空当前用户的剪切板记录
// @Tags clipboard
// @Security BearerAuth
// @Param onlyUnpinned query bool false "仅清空非置顶记录，默认true"
// @Success 200 {object} map[string]interface{} "清空成功"
// @Router /api/clipboard [delete]
func (h *ClipboardHandler) Clear(c *gin.Context) {
	userID := GetUserID(c)

	onlyUnpinned := c.Query("onlyUnpinned") != "false"

	var err error
	if onlyUnpinned {
		err = h.service.ClearUnpinned(c.Request.Context(), userID)
	} else {
		err = h.service.ClearAll(c.Request.Context(), userID)
	}

	if err != nil {
		response.InternalError(c, "failed to clear clipboard records")
		return
	}

	response.Success(c, nil)
}