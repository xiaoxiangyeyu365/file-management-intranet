package handler

import (
	"cloudbox/internal/model"
	"cloudbox/internal/service"
	"cloudbox/internal/util/response"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	adminService *service.AdminService
}

func NewAdminHandler(adminService *service.AdminService) *AdminHandler {
	return &AdminHandler{adminService: adminService}
}

type CreateUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Role     string `json:"role"`
}

type UpdateUserRequest struct {
	Role   string `json:"role"`
	Status string `json:"status"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}

type adminUserResponse struct {
	model.User
	UsedBytes int64 `json:"usedBytes"`
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	status := c.Query("status")

	result, err := h.adminService.ListUsers(c.Request.Context(), status)
	if err != nil {
		response.InternalError(c, "failed to list users")
		return
	}

	// Batch fetch storage usage for all users
	usageMap, err := h.adminService.GetAllUserStorageUsage(c.Request.Context())
	if err != nil {
		usageMap = make(map[int64]int64)
	}

	users := make([]adminUserResponse, 0, len(result.Users))
	for _, u := range result.Users {
		users = append(users, adminUserResponse{
			User:      u,
			UsedBytes: usageMap[u.ID],
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"users":        users,
			"total":        result.Total,
			"pendingCount": result.PendingCount,
		},
	})
}

func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	user, err := h.adminService.CreateUser(c.Request.Context(), req.Username, req.Password, req.Role)
	if err != nil {
		if err == service.ErrInvalidUsername {
			response.BadRequest(c, "username must be 3-50 alphanumeric characters")
			return
		}
		if err == service.ErrUsernameExists {
			response.BadRequest(c, "username already exists")
			return
		}
		response.InternalError(c, "failed to create user")
		return
	}

	response.Success(c, user)
}

func (h *AdminHandler) UpdateUser(c *gin.Context) {
	adminID := GetUserID(c)
	userID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	err := h.adminService.UpdateUser(c.Request.Context(), adminID, userID, req.Role, req.Status)
	if err != nil {
		if err == service.ErrCannotDemoteSelf {
			response.BadRequest(c, "cannot change your own role")
			return
		}
		response.InternalError(c, "failed to update user")
		return
	}

	response.Success(c, nil)
}

func (h *AdminHandler) ResetPassword(c *gin.Context) {
	userID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	err := h.adminService.ResetPassword(c.Request.Context(), userID, req.NewPassword)
	if err != nil {
		response.InternalError(c, "failed to reset password")
		return
	}

	response.Success(c, nil)
}

func (h *AdminHandler) DeleteUser(c *gin.Context) {
	adminID := GetUserID(c)
	userID, _ := strconv.ParseInt(c.Param("id"), 10, 64)

	result, err := h.adminService.DeleteUser(c.Request.Context(), adminID, userID)
	if err != nil {
		if err == service.ErrCannotDeleteSelf {
			response.BadRequest(c, "cannot delete yourself")
			return
		}
		response.InternalError(c, "failed to delete user")
		return
	}

	response.Success(c, result)
}

func (h *AdminHandler) UpdateUserQuota(c *gin.Context) {
	var req struct {
		DiskQuota *int64 `json:"disk_quota"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	if err := h.adminService.SetUserQuota(c.Request.Context(), id, req.DiskQuota); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, nil)
}
