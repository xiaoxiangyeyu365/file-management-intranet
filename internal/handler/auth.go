// internal/handler/auth.go
package handler

import (
	"cloudbox/internal/service"
	"cloudbox/internal/util/response"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=6"`
}

// Login godoc
// @Summary 用户登录
// @Description 使用用户名密码登录，返回 JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "登录信息"
// @Success 200 {object} map[string]interface{} "登录成功，返回 token"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Failure 401 {object} map[string]interface{} "用户名或密码错误"
// @Router /api/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		response.Error(c, 401, err.Error())
		return
	}

	response.Success(c, resp)
}

// ChangePassword godoc
// @Summary 修改密码
// @Description 修改当前用户的密码
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body ChangePasswordRequest true "密码信息"
// @Success 200 {object} map[string]interface{} "修改成功"
// @Failure 400 {object} map[string]interface{} "请求参数错误"
// @Router /api/auth/password [post]
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	userID := GetUserID(c)

	err := h.authService.ChangePassword(c.Request.Context(), userID, req.OldPassword, req.NewPassword)
	if err != nil {
		if err == service.ErrInvalidCredentials {
			response.Error(c, 400, "current password is incorrect")
			return
		}
		response.Error(c, 400, err.Error())
		return
	}

	response.Success(c, nil)
}

// Logout godoc
// @Summary 用户登出
// @Description JWT 无状态，单纯返回成功
// @Tags auth
// @Security BearerAuth
// @Success 200 {object} map[string]interface{} "登出成功"
// @Router /api/auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	// JWT is stateless, just return success
	response.Success(c, nil)
}

// GetProfile godoc
// @Summary 获取用户信息
// @Description 获取当前登录用户的基本信息
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{} "用户信息"
// @Failure 401 {object} map[string]interface{} "未授权"
// @Router /api/auth/profile [get]
func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID := GetUserID(c)

	user, err := h.authService.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		response.NotFound(c, "user not found")
		return
	}

	response.Success(c, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"role":     user.Role,
	})
}
