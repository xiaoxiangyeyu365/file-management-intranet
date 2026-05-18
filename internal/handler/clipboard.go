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