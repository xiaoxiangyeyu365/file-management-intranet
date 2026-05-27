# Multi-User Registration & Admin Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add user registration with admin approval, and an admin panel for user management with cascade file deletion.

**Architecture:** Extend existing auth system with status-based access control. New AdminService handles user CRUD with cascade file cleanup. Frontend adds register page and admin users management page.

**Tech Stack:** Go, Gin, GORM, Vue 3, Element Plus, Pinia

---

## File Structure

### Backend (Create/Modify)

| File | Action | Responsibility |
|------|--------|----------------|
| `internal/model/user.go` | Modify | Add Status, PasswordChanged fields + constants |
| `internal/config/config.go` | Modify | Add AuthConfig struct |
| `configs/config.yaml` | Modify | Add auth section |
| `internal/repository/user.go` | Modify | Add user CRUD methods (FindAll, CountByStatus, UpdateStatus, etc.) |
| `internal/repository/db.go` | Modify | Add migration for new columns |
| `internal/service/auth.go` | Modify | Add Register method, status check in Login |
| `internal/service/admin.go` | Create | Admin user management service |
| `internal/handler/middleware.go` | Modify | Add AdminMiddleware |
| `internal/handler/auth.go` | Modify | Add Register handler |
| `internal/handler/admin.go` | Create | Admin user management handlers |
| `cmd/server/main.go` | Modify | Register admin routes |

### Frontend (Create/Modify)

| File | Action | Responsibility |
|------|--------|----------------|
| `web/src/utils/api.js` | Modify | Add auth register + admin API calls |
| `web/src/stores/auth.js` | Modify | Add register action, user role |
| `web/src/stores/admin.js` | Create | Admin user management store |
| `web/src/router/index.js` | Modify | Add /register, /admin/users routes |
| `web/src/views/RegisterView.vue` | Create | Registration page |
| `web/src/views/AdminUsersView.vue` | Create | Admin user management page |
| `web/src/components/Layout/AppSidebar.vue` | Modify | Add admin nav entry |
| `web/src/views/LoginView.vue` | Modify | Add register link, status error handling |

---

## Task 1: User Model & Config Changes

**Files:**
- Modify: `internal/model/user.go`
- Modify: `internal/config/config.go`
- Modify: `configs/config.yaml`

- [ ] **Step 1: Update User model**

```go
// internal/model/user.go
package model

import "time"

const (
	UserStatusPending  = "pending"
	UserStatusApproved = "approved"
	UserStatusDisabled = "disabled"
)

type User struct {
	ID              int64     `gorm:"primaryKey" json:"id"`
	Username        string    `gorm:"uniqueIndex;size:50;not null" json:"username"`
	PasswordHash    string    `gorm:"size:255;not null" json:"-"`
	Role            string    `gorm:"size:20;default:user" json:"role"`
	Status          string    `gorm:"size:20;default:approved" json:"status"`
	PasswordChanged bool      `gorm:"default:true" json:"passwordChanged"`
	CreatedAt       time.Time `json:"createdAt"`
}

func (User) TableName() string {
	return "users"
}
```

- [ ] **Step 2: Update Config struct**

Add `AuthConfig` field to `Config` struct in `internal/config/config.go`:

```go
type AuthConfig struct {
	Registration     bool `yaml:"registration"`
	ApprovalRequired bool `yaml:"approval_required"`
}
```

Update the `Config` struct to include it:

```go
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Storage  StorageConfig  `yaml:"storage"`
	Upload   UploadConfig   `yaml:"upload"`
	JWT      JWTConfig      `yaml:"jwt"`
	Log      LogConfig      `yaml:"log"`
	Admin    AdminConfig    `yaml:"admin"`
	Auth     AuthConfig     `yaml:"auth"`
}
```

Update the default config in `Load()`:

```go
Auth: AuthConfig{
	Registration:     true,
	ApprovalRequired: true,
},
```

- [ ] **Step 3: Update config.yaml**

Add to `configs/config.yaml`:

```yaml
auth:
  registration: true
  approval_required: true
```

- [ ] **Step 4: Commit**

```bash
git add internal/model/user.go internal/config/config.go configs/config.yaml
git commit -m "feat: add user status/password-changed fields and auth config"
```

---

## Task 2: Database Migration

**Files:**
- Modify: `internal/repository/db.go`

- [ ] **Step 1: Add migration for new columns**

In `internal/repository/db.go`, after the existing `AutoMigrate` call, add migration logic for the new columns. The `AutoMigrate` for `model.User` will handle new columns automatically since GORM adds missing columns. But we need to set default values for existing rows.

Add this after the `AutoMigrate` call in `InitDB`:

```go
// Migrate all models
if err := DB.AutoMigrate(
    &model.User{},
    &model.PhysicalFile{},
); err != nil {
    log.Fatalf("failed to migrate database: %v", err)
}

// Ensure existing users have status and password_changed set
DB.Exec("UPDATE users SET status = 'approved' WHERE status IS NULL OR status = ''")
DB.Exec("UPDATE users SET password_changed = 1 WHERE password_changed IS NULL")
```

- [ ] **Step 2: Commit**

```bash
git add internal/repository/db.go
git commit -m "feat: add database migration for user status columns"
```

---

## Task 3: User Repository Extensions

**Files:**
- Modify: `internal/repository/user.go`

- [ ] **Step 1: Add new repository methods**

Add these methods to `internal/repository/user.go`:

```go
func (r *UserRepository) FindAll(ctx context.Context, status string) ([]model.User, error) {
	var users []model.User
	query := r.db.WithContext(ctx).Order("created_at DESC")
	if status != "" {
		query = query.Where("status = ?", status)
	}
	err := query.Find(&users).Error
	return users, err
}

func (r *UserRepository) CountByStatus(ctx context.Context, status string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.User{}).Where("status = ?", status).Count(&count).Error
	return count, err
}

func (r *UserRepository) UpdateStatus(ctx context.Context, userID int64, status string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("status", status).Error
}

func (r *UserRepository) UpdateRole(ctx context.Context, userID int64, role string) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("role", role).Error
}

func (r *UserRepository) UpdatePasswordChanged(ctx context.Context, userID int64, changed bool) error {
	return r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("password_changed", changed).Error
}

func (r *UserRepository) DeleteByID(ctx context.Context, userID int64) error {
	return r.db.WithContext(ctx).Delete(&model.User{}, userID).Error
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/repository/user.go
git commit -m "feat: add user repository CRUD methods for admin management"
```

---

## Task 4: Auth Service - Register & Login Status Check

**Files:**
- Modify: `internal/service/auth.go`

- [ ] **Step 1: Add new error variables**

At the top of `internal/service/auth.go`, add:

```go
var (
	ErrInvalidCredentials  = errors.New("invalid username or password")
	ErrUserNotFound        = errors.New("user not found")
	ErrSamePassword        = errors.New("new password must be different")
	ErrAccountPending      = errors.New("account pending approval")
	ErrAccountDisabled     = errors.New("account has been disabled")
	ErrRegistrationClosed  = errors.New("registration is disabled")
	ErrInvalidUsername     = errors.New("username must be 3-50 alphanumeric characters")
	ErrUsernameExists      = errors.New("username already exists")
)
```

- [ ] **Step 2: Add Register method**

Add to `internal/service/auth.go`:

```go
func (s *AuthService) Register(ctx context.Context, username, password string) error {
	cfg := config.Get()
	if !cfg.Auth.Registration {
		return ErrRegistrationClosed
	}

	// Validate username: 3-50 chars, alphanumeric + underscore
	if len(username) < 3 || len(username) > 50 {
		return ErrInvalidUsername
	}
	for _, c := range username {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return ErrInvalidUsername
		}
	}

	// Check uniqueness
	_, err := s.userRepo.FindByUsername(ctx, username)
	if err == nil {
		return ErrUsernameExists
	}

	hash, err := crypto.HashPassword(password)
	if err != nil {
		return err
	}

	status := model.UserStatusApproved
	if cfg.Auth.ApprovalRequired {
		status = model.UserStatusPending
	}

	user := &model.User{
		Username:        username,
		PasswordHash:    hash,
		Role:            "user",
		Status:          status,
		PasswordChanged: true,
	}

	return s.userRepo.Create(ctx, user)
}
```

- [ ] **Step 3: Update Login method with status check**

Replace the existing `Login` method in `internal/service/auth.go`:

```go
func (s *AuthService) Login(ctx context.Context, username, password string) (*LoginResponse, error) {
	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if !crypto.CheckPassword(password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	// Status check
	if user.Status == model.UserStatusPending {
		return nil, ErrAccountPending
	}
	if user.Status == model.UserStatusDisabled {
		return nil, ErrAccountDisabled
	}

	// Check if password needs to be changed
	requireChange := !user.PasswordChanged
	// Also check default admin password
	cfg := config.Get()
	if password == cfg.Admin.Password {
		requireChange = true
	}

	token, err := crypto.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, err
	}

	return &LoginResponse{
		Token:                token,
		RequirePasswordChange: requireChange,
	}, nil
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/service/auth.go
git commit -m "feat: add user registration and login status check"
```

---

## Task 5: Admin Service

**Files:**
- Create: `internal/service/admin.go`

- [ ] **Step 1: Create AdminService**

Create `internal/service/admin.go`:

```go
package service

import (
	"cloudbox/internal/model"
	"cloudbox/internal/repository"
	"cloudbox/internal/util/crypto"
	"context"
	"errors"
	"log"
	"os"
)

var (
	ErrCannotDeleteSelf = errors.New("cannot delete yourself")
	ErrCannotDemoteSelf = errors.New("cannot change your own role")
)

type AdminService struct {
	userRepo      *repository.UserRepository
	fileRepo      *repository.FileRepository
	physicalRepo  *repository.PhysicalFileRepository
	clipboardRepo *repository.ClipboardRepository
	fileService   *FileService
}

func NewAdminService(
	userRepo *repository.UserRepository,
	fileRepo *repository.FileRepository,
	physicalRepo *repository.PhysicalFileRepository,
	clipboardRepo *repository.ClipboardRepository,
	fileService *FileService,
) *AdminService {
	return &AdminService{
		userRepo:      userRepo,
		fileRepo:      fileRepo,
		physicalRepo:  physicalRepo,
		clipboardRepo: clipboardRepo,
		fileService:   fileService,
	}
}

type AdminUserListResult struct {
	Users        []model.User `json:"users"`
	Total        int64        `json:"total"`
	PendingCount int64        `json:"pendingCount"`
}

func (s *AdminService) ListUsers(ctx context.Context, status string) (*AdminUserListResult, error) {
	users, err := s.userRepo.FindAll(ctx, status)
	if err != nil {
		return nil, err
	}

	total, _ := s.userRepo.CountByStatus(ctx, "")
	pending, _ := s.userRepo.CountByStatus(ctx, model.UserStatusPending)

	return &AdminUserListResult{
		Users:        users,
		Total:        total,
		PendingCount: pending,
	}, nil
}

func (s *AdminService) CreateUser(ctx context.Context, username, password, role string) (*model.User, error) {
	// Validate username
	if len(username) < 3 || len(username) > 50 {
		return nil, ErrInvalidUsername
	}
	for _, c := range username {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return nil, ErrInvalidUsername
		}
	}

	// Check uniqueness
	_, err := s.userRepo.FindByUsername(ctx, username)
	if err == nil {
		return nil, ErrUsernameExists
	}

	if role == "" {
		role = "user"
	}

	hash, err := crypto.HashPassword(password)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Username:        username,
		PasswordHash:    hash,
		Role:            role,
		Status:          model.UserStatusApproved,
		PasswordChanged: false, // Force password change on first login
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *AdminService) UpdateUser(ctx context.Context, adminID, userID int64, role, status string) error {
	if adminID == userID && role != "" {
		return ErrCannotDemoteSelf
	}

	if role != "" {
		if err := s.userRepo.UpdateRole(ctx, userID, role); err != nil {
			return err
		}
	}
	if status != "" {
		if err := s.userRepo.UpdateStatus(ctx, userID, status); err != nil {
			return err
		}
	}
	return nil
}

func (s *AdminService) ResetPassword(ctx context.Context, userID int64, newPassword string) error {
	hash, err := crypto.HashPassword(newPassword)
	if err != nil {
		return err
	}

	if err := s.userRepo.UpdatePassword(ctx, userID, hash); err != nil {
		return err
	}

	// Force password change on next login
	return s.userRepo.UpdatePasswordChanged(ctx, userID, false)
}

type DeleteUserResult struct {
	DeletedFiles   int `json:"deletedFiles"`
	DeletedFolders int `json:"deletedFolders"`
}

func (s *AdminService) DeleteUser(ctx context.Context, adminID, userID int64) (*DeleteUserResult, error) {
	if adminID == userID {
		return nil, ErrCannotDeleteSelf
	}

	// Get all files owned by this user (non-deleted and deleted)
	allFiles, err := s.fileRepo.FindAllByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := &DeleteUserResult{}
	physicalRefCount := make(map[int64]int)

	for _, f := range allFiles {
		if f.IsFolder {
			result.DeletedFolders++
		} else {
			result.DeletedFiles++
			if f.ContentRef != 0 {
				physicalRefCount[f.ContentRef]++
			}
		}
	}

	// Delete all file records
	for _, f := range allFiles {
		if err := s.fileRepo.Delete(ctx, f.ID); err != nil {
			log.Printf("warning: failed to delete file record %d: %v", f.ID, err)
		}
	}

	// Handle physical files
	for pid, count := range physicalRefCount {
		newRefCount, err := s.physicalRepo.DecrementRefCount(ctx, pid, count)
		if err != nil {
			log.Printf("warning: failed to decrement ref count for physical file %d: %v", pid, err)
			continue
		}

		if newRefCount <= 0 {
			pf, err := s.physicalRepo.FindByID(ctx, pid)
			if err != nil {
				log.Printf("warning: failed to find physical file %d: %v", pid, err)
				continue
			}

			absPath := s.fileService.GetStorage().ToAbsPath(pf.StoragePath)
			if err := removeFileIfExists(absPath); err != nil {
				log.Printf("warning: failed to delete physical file %s: %v", absPath, err)
			}

			if pf.ThumbnailPath != "" {
				removeFileIfExists(pf.ThumbnailPath)
			}

			if err := s.physicalRepo.Delete(ctx, pid); err != nil {
				log.Printf("warning: failed to delete physical file record %d: %v", pid, err)
			}
		}
	}

	// Delete clipboard records
	s.clipboardRepo.DeleteByUser(ctx, userID)

	// Delete user
	if err := s.userRepo.DeleteByID(ctx, userID); err != nil {
		return nil, err
	}

	return result, nil
}

func removeFileIfExists(path string) error {
	if _, err := os.Stat(path); err == nil {
		return os.Remove(path)
	}
	return nil
}
```

- [ ] **Step 2: Add FindAllByOwner to FileRepository**

Add to `internal/repository/file.go`:

```go
func (r *FileRepository) FindAllByOwner(ctx context.Context, ownerID int64) ([]model.File, error) {
	var files []model.File
	err := r.db.WithContext(ctx).
		Where("owner_id = ?", ownerID).
		Find(&files).Error
	return files, err
}

func (r *FileRepository) GetDB() *gorm.DB {
	return r.db
}
```

- [ ] **Step 3: Commit**

Note: `ClipboardRepository.DeleteByUser` already exists in `internal/repository/clipboard.go` — no changes needed there.

```bash
git add internal/service/admin.go internal/repository/file.go
git commit -m "feat: add admin service with user management and cascade delete"
```

---

## Task 6: Admin Middleware & Auth Handler Updates

**Files:**
- Modify: `internal/handler/middleware.go`
- Modify: `internal/handler/auth.go`

- [ ] **Step 1: Add AdminMiddleware**

Add to `internal/handler/middleware.go`:

```go
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists || role.(string) != "admin" {
			response.Forbidden(c, "admin access required")
			c.Abort()
			return
		}
		c.Next()
	}
}
```

- [ ] **Step 2: Add Forbidden to response package**

Add to `internal/util/response/json.go`:

```go
func Forbidden(c *gin.Context, message string) {
	c.JSON(http.StatusForbidden, Response{
		Code:    403,
		Message: message,
	})
}
```

- [ ] **Step 3: Add Register handler**

Add to `internal/handler/auth.go`:

```go
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	err := h.authService.Register(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		if err == service.ErrRegistrationClosed {
			response.Error(c, 403, "registration is disabled")
			return
		}
		if err == service.ErrInvalidUsername {
			response.BadRequest(c, "username must be 3-50 alphanumeric characters")
			return
		}
		if err == service.ErrUsernameExists {
			response.BadRequest(c, "username already exists")
			return
		}
		response.InternalError(c, "registration failed")
		return
	}

	response.Success(c, gin.H{
		"message": "注册成功，等待管理员审批",
	})
}
```

- [ ] **Step 4: Update Login handler error messages**

Update the `Login` handler in `internal/handler/auth.go` to return specific error messages:

```go
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		if err == service.ErrAccountPending {
			response.Error(c, 401, "账号待审批，请联系管理员")
			return
		}
		if err == service.ErrAccountDisabled {
			response.Error(c, 401, "账号已被禁用")
			return
		}
		response.Error(c, 401, "用户名或密码错误")
		return
	}

	response.Success(c, resp)
}
```

- [ ] **Step 5: Commit**

```bash
git add internal/handler/middleware.go internal/handler/auth.go internal/util/response/json.go
git commit -m "feat: add admin middleware, register handler, and login status check"
```

---

## Task 7: Admin Handler

**Files:**
- Create: `internal/handler/admin.go`

- [ ] **Step 1: Create AdminHandler**

Create `internal/handler/admin.go`:

```go
package handler

import (
	"cloudbox/internal/service"
	"cloudbox/internal/util/response"
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

func (h *AdminHandler) ListUsers(c *gin.Context) {
	status := c.Query("status")

	result, err := h.adminService.ListUsers(c.Request.Context(), status)
	if err != nil {
		response.InternalError(c, "failed to list users")
		return
	}

	response.Success(c, result)
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
```

- [ ] **Step 2: Commit**

```bash
git add internal/handler/admin.go
git commit -m "feat: add admin handler for user management"
```

---

## Task 8: Route Registration

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Register admin routes**

In `cmd/server/main.go`, add after the existing service/handler initialization:

```go
// Initialize admin service and handler
adminService := service.NewAdminService(userRepo, fileRepo, physicalRepo, clipboardRepo, fileService)
adminHandler := handler.NewAdminHandler(adminService)
```

Add the admin route group after the protected routes:

```go
// Admin routes
admin := api.Group("/admin")
admin.Use(handler.JWTMiddleware(), handler.AdminMiddleware())
{
	admin.GET("/users", adminHandler.ListUsers)
	admin.POST("/users", adminHandler.CreateUser)
	admin.PUT("/users/:id", adminHandler.UpdateUser)
	admin.PUT("/users/:id/password", adminHandler.ResetPassword)
	admin.DELETE("/users/:id", adminHandler.DeleteUser)
}
```

Add the registration route in the auth group (before protected routes):

```go
auth := api.Group("/auth")
auth.Use(middleware.RateLimit(100, time.Minute))
{
	auth.POST("/login", authHandler.Login)
	auth.POST("/register", authHandler.Register)
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: register admin and registration routes"
```

---

## Task 9: Frontend API & Stores

**Files:**
- Modify: `web/src/utils/api.js`
- Modify: `web/src/stores/auth.js`
- Create: `web/src/stores/admin.js`

- [ ] **Step 1: Add API calls**

Add to `web/src/utils/api.js`:

```javascript
// Auth API - add register
export const authAPI = {
  login: (username, password) => api.post('/auth/login', { username, password }),
  register: (username, password) => api.post('/auth/register', { username, password }),
  changePassword: (oldPassword, newPassword) =>
    api.post('/auth/password', { oldPassword, newPassword }),
  logout: () => api.post('/auth/logout'),
  profile: () => api.get('/auth/profile')
}

// Admin API
export const adminAPI = {
  listUsers: (status = '') => api.get('/admin/users', { params: { status } }),
  createUser: (username, password, role = 'user') =>
    api.post('/admin/users', { username, password, role }),
  updateUser: (id, data) => api.put(`/admin/users/${id}`, data),
  resetPassword: (id, newPassword) =>
    api.put(`/admin/users/${id}/password`, { newPassword }),
  deleteUser: (id) => api.delete(`/admin/users/${id}`)
}
```

- [ ] **Step 2: Update auth store**

Update `web/src/stores/auth.js` to store user role and add register:

```javascript
import { defineStore } from 'pinia'
import { authAPI } from '@/utils/api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null,
    token: localStorage.getItem('cloudbox_token') || null
  }),

  getters: {
    isLoggedIn: (state) => !!state.token,
    isAdmin: (state) => state.user?.role === 'admin'
  },

  actions: {
    async login(username, password) {
      const response = await authAPI.login(username, password)
      const data = response.data || response
      this.token = data.token
      localStorage.setItem('cloudbox_token', data.token)
      await this.fetchProfile()
      return data
    },

    async register(username, password) {
      const response = await authAPI.register(username, password)
      return response.data || response
    },

    logout() {
      this.token = null
      this.user = null
      localStorage.removeItem('cloudbox_token')
    },

    async changePassword(oldPassword, newPassword) {
      return await authAPI.changePassword(oldPassword, newPassword)
    },

    async fetchProfile() {
      if (!this.token) return null
      try {
        const response = await authAPI.profile()
        const userData = response.data || response
        this.user = {
          id: userData.id,
          username: userData.username,
          role: userData.role
        }
        return this.user
      } catch (error) {
        this.logout()
        return null
      }
    }
  }
})
```

- [ ] **Step 3: Create admin store**

Create `web/src/stores/admin.js`:

```javascript
import { defineStore } from 'pinia'
import { adminAPI } from '@/utils/api'

export const useAdminStore = defineStore('admin', {
  state: () => ({
    users: [],
    total: 0,
    pendingCount: 0,
    loading: false,
    statusFilter: ''
  }),

  actions: {
    async fetchUsers(status = '') {
      this.loading = true
      try {
        const response = await adminAPI.listUsers(status)
        const data = response.data || response
        this.users = data.users || []
        this.total = data.total || 0
        this.pendingCount = data.pendingCount || 0
        this.statusFilter = status
      } finally {
        this.loading = false
      }
    },

    async createUser(username, password, role) {
      await adminAPI.createUser(username, password, role)
      await this.fetchUsers(this.statusFilter)
    },

    async updateUser(id, data) {
      await adminAPI.updateUser(id, data)
      await this.fetchUsers(this.statusFilter)
    },

    async resetPassword(id, newPassword) {
      await adminAPI.resetPassword(id, newPassword)
    },

    async deleteUser(id) {
      await adminAPI.deleteUser(id)
      await this.fetchUsers(this.statusFilter)
    }
  }
})
```

- [ ] **Step 4: Commit**

```bash
git add web/src/utils/api.js web/src/stores/auth.js web/src/stores/admin.js
git commit -m "feat: add frontend API calls and stores for user management"
```

---

## Task 10: Frontend Routes & Sidebar

**Files:**
- Modify: `web/src/router/index.js`
- Modify: `web/src/components/Layout/AppSidebar.vue`

- [ ] **Step 1: Update router**

Update `web/src/router/index.js`:

```javascript
import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import LoginView from '@/views/LoginView.vue'
import FilesView from '@/views/FilesView.vue'
import TrashView from '@/views/TrashView.vue'
import ClipboardView from '@/views/ClipboardView.vue'
import RegisterView from '@/views/RegisterView.vue'
import AdminUsersView from '@/views/AdminUsersView.vue'

const routes = [
  {
    path: '/login',
    name: 'Login',
    component: LoginView,
    meta: { guest: true }
  },
  {
    path: '/register',
    name: 'Register',
    component: RegisterView,
    meta: { guest: true }
  },
  {
    path: '/',
    name: 'Files',
    component: FilesView,
    meta: { requiresAuth: true }
  },
  {
    path: '/trash',
    name: 'Trash',
    component: TrashView,
    meta: { requiresAuth: true }
  },
  {
    path: '/clipboard',
    name: 'Clipboard',
    component: ClipboardView,
    meta: { requiresAuth: true }
  },
  {
    path: '/admin/users',
    name: 'AdminUsers',
    component: AdminUsersView,
    meta: { requiresAuth: true, requiresAdmin: true }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to, from, next) => {
  const authStore = useAuthStore()

  if (authStore.token && !authStore.user) {
    await authStore.fetchProfile()
  }

  if (to.meta.requiresAuth && !authStore.token) {
    next('/login')
  } else if (to.meta.requiresAdmin && !authStore.isAdmin) {
    next('/')
  } else if (to.meta.guest && authStore.user) {
    next('/')
  } else {
    next()
  }
})

export default router
```

- [ ] **Step 2: Update sidebar**

Update `web/src/components/Layout/AppSidebar.vue`:

```vue
<template>
  <aside class="app-sidebar">
    <nav class="sidebar-nav">
      <router-link
        to="/"
        class="nav-item"
        :class="{ active: route.path === '/' }"
      >
        <el-icon><Folder /></el-icon>
        <span>全部文件</span>
      </router-link>

      <router-link
        to="/trash"
        class="nav-item"
        :class="{ active: route.path === '/trash' }"
      >
        <el-icon><Delete /></el-icon>
        <span>回收站</span>
      </router-link>

      <router-link
        to="/clipboard"
        class="nav-item"
        :class="{ active: route.path === '/clipboard' }"
      >
        <el-icon><Document /></el-icon>
        <span>云剪切板</span>
      </router-link>

      <router-link
        v-if="authStore.isAdmin"
        to="/admin/users"
        class="nav-item"
        :class="{ active: route.path.startsWith('/admin') }"
      >
        <el-icon><User /></el-icon>
        <span>用户管理</span>
      </router-link>
    </nav>
  </aside>
</template>

<script setup>
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { Folder, Delete, Document, User } from '@element-plus/icons-vue'

const route = useRoute()
const authStore = useAuthStore()
</script>

<style scoped lang="scss">
.app-sidebar {
  width: var(--sidebar-width);
  background: #fafafa;
  border-right: 1px solid #e4e7ed;
  padding: 16px 0;
}

.sidebar-nav {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 0 12px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  text-decoration: none;
  color: #606266;
  border-radius: 8px;
  transition: all 0.2s;

  &:hover {
    background: #f0f2f5;
    color: #409eff;
  }

  &.active {
    background: #ecf5ff;
    color: #409eff;
  }

  .el-icon {
    font-size: 20px;
  }

  span {
    font-size: 14px;
  }
}
</style>
```

- [ ] **Step 3: Commit**

```bash
git add web/src/router/index.js web/src/components/Layout/AppSidebar.vue
git commit -m "feat: add routes and sidebar entry for registration and admin"
```

---

## Task 11: Register View

**Files:**
- Create: `web/src/views/RegisterView.vue`

- [ ] **Step 1: Create RegisterView**

Create `web/src/views/RegisterView.vue`:

```vue
<template>
  <div class="register-container">
    <div class="register-box">
      <div class="register-header">
        <h1>CloudBox</h1>
        <p>注册新账号</p>
      </div>

      <div v-if="registered" class="success-message">
        <el-alert
          type="success"
          title="注册成功，等待管理员审批"
          description="管理员审批后即可登录，请耐心等待。"
          show-icon
          :closable="false"
        />
        <el-button
          type="primary"
          style="width: 100%; margin-top: 16px"
          @click="router.push('/login')"
        >
          返回登录
        </el-button>
      </div>

      <el-form
        v-else
        ref="formRef"
        :model="form"
        :rules="rules"
        @submit.prevent="handleRegister"
      >
        <el-form-item prop="username">
          <el-input
            v-model="form.username"
            placeholder="用户名（3-50个字符）"
            prefix-icon="User"
            size="large"
          />
        </el-form-item>

        <el-form-item prop="password">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="密码（至少6位）"
            prefix-icon="Lock"
            size="large"
            show-password
          />
        </el-form-item>

        <el-form-item prop="confirmPassword">
          <el-input
            v-model="form.confirmPassword"
            type="password"
            placeholder="确认密码"
            prefix-icon="Lock"
            size="large"
            show-password
            @keyup.enter="handleRegister"
          />
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="loading"
            style="width: 100%"
            @click="handleRegister"
          >
            注册
          </el-button>
        </el-form-item>
      </el-form>

      <div class="register-footer">
        <span>已有账号？</span>
        <router-link to="/login">去登录</router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'

const router = useRouter()
const authStore = useAuthStore()

const formRef = ref(null)
const loading = ref(false)
const registered = ref(false)

const form = reactive({
  username: '',
  password: '',
  confirmPassword: ''
})

const validateConfirmPassword = (rule, value, callback) => {
  if (value !== form.password) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const rules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 50, message: '用户名长度为3-50个字符', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_]+$/, message: '用户名只能包含字母、数字和下划线', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码至少6位', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    { validator: validateConfirmPassword, trigger: 'blur' }
  ]
}

async function handleRegister() {
  if (!formRef.value) return

  try {
    await formRef.value.validate()
  } catch {
    return
  }

  loading.value = true

  try {
    await authStore.register(form.username, form.password)
    registered.value = true
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '注册失败')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped lang="scss">
.register-container {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.register-box {
  width: 360px;
  padding: 40px;
  background: white;
  border-radius: 12px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
}

.register-header {
  text-align: center;
  margin-bottom: 32px;

  h1 {
    font-size: 28px;
    font-weight: 600;
    color: #333;
    margin-bottom: 8px;
  }

  p {
    color: #666;
    font-size: 14px;
  }
}

.register-footer {
  text-align: center;
  margin-top: 16px;
  font-size: 14px;
  color: #666;

  a {
    color: #409eff;
    text-decoration: none;
    margin-left: 4px;

    &:hover {
      text-decoration: underline;
    }
  }
}
</style>
```

- [ ] **Step 2: Commit**

```bash
git add web/src/views/RegisterView.vue
git commit -m "feat: add registration page"
```

---

## Task 12: Login View Updates

**Files:**
- Modify: `web/src/views/LoginView.vue`

- [ ] **Step 1: Add register link and status error handling**

Update `web/src/views/LoginView.vue`:

```vue
<template>
  <div class="login-container">
    <div class="login-box">
      <div class="login-header">
        <h1>CloudBox</h1>
        <p>内网云存储</p>
      </div>

      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        @submit.prevent="handleLogin"
      >
        <el-form-item prop="username">
          <el-input
            v-model="form.username"
            placeholder="用户名"
            prefix-icon="User"
            size="large"
          />
        </el-form-item>

        <el-form-item prop="password">
          <el-input
            v-model="form.password"
            type="password"
            placeholder="密码"
            prefix-icon="Lock"
            size="large"
            show-password
            @keyup.enter="handleLogin"
          />
        </el-form-item>

        <el-form-item>
          <el-button
            type="primary"
            size="large"
            :loading="loading"
            style="width: 100%"
            @click="handleLogin"
          >
            登录
          </el-button>
        </el-form-item>
      </el-form>

      <el-alert
        v-if="error"
        type="error"
        :title="error"
        show-icon
        :closable="false"
        style="margin-top: 16px"
      />

      <div class="login-footer">
        <router-link to="/register">注册新账号</router-link>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { ElMessage } from 'element-plus'

const router = useRouter()
const authStore = useAuthStore()

const formRef = ref(null)
const loading = ref(false)
const error = ref('')

const form = reactive({
  username: '',
  password: ''
})

const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
}

async function handleLogin() {
  if (!formRef.value) return

  try {
    await formRef.value.validate()
  } catch {
    return
  }

  loading.value = true
  error.value = ''

  try {
    const data = await authStore.login(form.username, form.password)
    if (data.requirePasswordChange) {
      // TODO: show change password dialog
      ElMessage.warning('请修改默认密码')
    }
    router.push('/')
  } catch (err) {
    error.value = err.response?.data?.message || '登录失败，请检查用户名和密码'
  } finally {
    loading.value = false
  }
}
</script>

<style scoped lang="scss">
.login-container {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.login-box {
  width: 360px;
  padding: 40px;
  background: white;
  border-radius: 12px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.2);
}

.login-header {
  text-align: center;
  margin-bottom: 32px;

  h1 {
    font-size: 28px;
    font-weight: 600;
    color: #333;
    margin-bottom: 8px;
  }

  p {
    color: #666;
    font-size: 14px;
  }
}

.login-footer {
  text-align: center;
  margin-top: 16px;
  font-size: 14px;

  a {
    color: #409eff;
    text-decoration: none;

    &:hover {
      text-decoration: underline;
    }
  }
}
</style>
```

- [ ] **Step 2: Commit**

```bash
git add web/src/views/LoginView.vue
git commit -m "feat: add register link and status error handling to login page"
```

---

## Task 13: Admin Users View

**Files:**
- Create: `web/src/views/AdminUsersView.vue`

- [ ] **Step 1: Create AdminUsersView**

Create `web/src/views/AdminUsersView.vue`:

```vue
<template>
  <div class="admin-users-view">
    <div class="page-header">
      <h2>用户管理</h2>
      <el-button type="primary" @click="showCreateDialog = true">
        创建用户
      </el-button>
    </div>

    <div class="filter-tabs">
      <el-radio-group v-model="statusFilter" @change="fetchUsers">
        <el-radio-button label="">全部</el-radio-button>
        <el-radio-button label="pending">
          待审批
          <el-badge v-if="adminStore.pendingCount > 0" :value="adminStore.pendingCount" class="tab-badge" />
        </el-radio-button>
        <el-radio-button label="approved">正常</el-radio-button>
        <el-radio-button label="disabled">已禁用</el-radio-button>
      </el-radio-group>
    </div>

    <el-table :data="adminStore.users" v-loading="adminStore.loading" stripe>
      <el-table-column prop="username" label="用户名" width="180" />
      <el-table-column prop="role" label="角色" width="120">
        <template #default="{ row }">
          <el-tag :type="row.role === 'admin' ? 'danger' : 'info'" size="small">
            {{ row.role === 'admin' ? '管理员' : '普通用户' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="status" label="状态" width="120">
        <template #default="{ row }">
          <el-tag :type="statusTagType(row.status)" size="small">
            {{ statusLabel(row.status) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="passwordChanged" label="密码状态" width="120">
        <template #default="{ row }">
          <el-tag :type="row.passwordChanged ? 'success' : 'warning'" size="small">
            {{ row.passwordChanged ? '已修改' : '默认密码' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="createdAt" label="创建时间" width="180">
        <template #default="{ row }">
          {{ formatDate(row.createdAt) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" min-width="240" fixed="right">
        <template #default="{ row }">
          <el-button
            v-if="row.status === 'pending'"
            type="success"
            size="small"
            @click="handleApprove(row)"
          >
            审批
          </el-button>
          <el-button
            v-if="row.status === 'approved'"
            type="warning"
            size="small"
            @click="handleDisable(row)"
          >
            禁用
          </el-button>
          <el-button
            v-if="row.status === 'disabled'"
            type="success"
            size="small"
            @click="handleEnable(row)"
          >
            启用
          </el-button>
          <el-button
            size="small"
            @click="showResetDialog(row)"
          >
            重置密码
          </el-button>
          <el-button
            v-if="row.id !== currentUserId"
            type="danger"
            size="small"
            @click="handleDelete(row)"
          >
            删除
          </el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Create User Dialog -->
    <el-dialog v-model="showCreateDialog" title="创建用户" width="400">
      <el-form ref="createFormRef" :model="createForm" :rules="createRules">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="createForm.username" placeholder="3-50个字符" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="createForm.password" type="password" placeholder="至少6位" show-password />
        </el-form-item>
        <el-form-item label="角色" prop="role">
          <el-select v-model="createForm.role" style="width: 100%">
            <el-option label="普通用户" value="user" />
            <el-option label="管理员" value="admin" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreateDialog = false">取消</el-button>
        <el-button type="primary" :loading="createLoading" @click="handleCreate">创建</el-button>
      </template>
    </el-dialog>

    <!-- Reset Password Dialog -->
    <el-dialog v-model="showResetDialogVisible" title="重置密码" width="400">
      <el-form ref="resetFormRef" :model="resetForm" :rules="resetRules">
        <el-form-item label="新密码" prop="newPassword">
          <el-input v-model="resetForm.newPassword" type="password" placeholder="至少6位" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showResetDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="resetLoading" @click="handleResetPassword">确认</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useAdminStore } from '@/stores/admin'
import { useAuthStore } from '@/stores/auth'
import { ElMessage, ElMessageBox } from 'element-plus'

const adminStore = useAdminStore()
const authStore = useAuthStore()

const statusFilter = ref('')
const currentUserId = computed(() => authStore.user?.id)

// Create dialog
const showCreateDialog = ref(false)
const createLoading = ref(false)
const createFormRef = ref(null)
const createForm = reactive({
  username: '',
  password: '',
  role: 'user'
})
const createRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 50, message: '长度为3-50个字符', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_]+$/, message: '只能包含字母、数字和下划线', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, message: '密码至少6位', trigger: 'blur' }
  ]
}

// Reset dialog
const showResetDialogVisible = ref(false)
const resetLoading = ref(false)
const resetFormRef = ref(null)
const resetTargetUser = ref(null)
const resetForm = reactive({ newPassword: '' })
const resetRules = {
  newPassword: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, message: '密码至少6位', trigger: 'blur' }
  ]
}

function fetchUsers() {
  adminStore.fetchUsers(statusFilter.value)
}

function statusTagType(status) {
  switch (status) {
    case 'approved': return 'success'
    case 'pending': return 'warning'
    case 'disabled': return 'danger'
    default: return 'info'
  }
}

function statusLabel(status) {
  switch (status) {
    case 'approved': return '正常'
    case 'pending': return '待审批'
    case 'disabled': return '已禁用'
    default: return status
  }
}

function formatDate(dateStr) {
  if (!dateStr) return ''
  const d = new Date(dateStr)
  return d.toLocaleString('zh-CN')
}

async function handleApprove(user) {
  await adminStore.updateUser(user.id, { status: 'approved' })
  ElMessage.success('已审批通过')
}

async function handleDisable(user) {
  await adminStore.updateUser(user.id, { status: 'disabled' })
  ElMessage.success('已禁用')
}

async function handleEnable(user) {
  await adminStore.updateUser(user.id, { status: 'approved' })
  ElMessage.success('已启用')
}

function showResetDialog(user) {
  resetTargetUser.value = user
  resetForm.newPassword = ''
  showResetDialogVisible.value = true
}

async function handleResetPassword() {
  if (!resetFormRef.value) return
  try {
    await resetFormRef.value.validate()
  } catch { return }

  resetLoading.value = true
  try {
    await adminStore.resetPassword(resetTargetUser.value.id, resetForm.newPassword)
    showResetDialogVisible.value = false
    ElMessage.success('密码已重置，用户下次登录需修改密码')
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '重置失败')
  } finally {
    resetLoading.value = false
  }
}

async function handleCreate() {
  if (!createFormRef.value) return
  try {
    await createFormRef.value.validate()
  } catch { return }

  createLoading.value = true
  try {
    await adminStore.createUser(createForm.username, createForm.password, createForm.role)
    showCreateDialog.value = false
    createForm.username = ''
    createForm.password = ''
    createForm.role = 'user'
    ElMessage.success('用户创建成功')
  } catch (err) {
    ElMessage.error(err.response?.data?.message || '创建失败')
  } finally {
    createLoading.value = false
  }
}

async function handleDelete(user) {
  try {
    await ElMessageBox.confirm(
      `删除用户 "${user.username}" 将同时永久删除其所有文件，且不可恢复。是否继续？`,
      '确认删除用户',
      { confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning' }
    )

    const result = await adminStore.deleteUser(user.id)
    const data = result?.data || result
    ElMessage.success(`已删除用户，共删除 ${data?.deletedFiles || 0} 个文件，${data?.deletedFolders || 0} 个文件夹`)
  } catch (err) {
    if (err !== 'cancel') {
      ElMessage.error(err.response?.data?.message || '删除失败')
    }
  }
}

onMounted(() => {
  fetchUsers()
})
</script>

<style scoped lang="scss">
.admin-users-view {
  padding: 24px;
  height: 100%;
  overflow-y: auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;

  h2 {
    margin: 0;
    font-size: 20px;
    color: #303133;
  }
}

.filter-tabs {
  margin-bottom: 20px;
}

.tab-badge {
  margin-left: 8px;
}
</style>
```

- [ ] **Step 2: Commit**

```bash
git add web/src/views/AdminUsersView.vue
git commit -m "feat: add admin users management page"
```

---

## Task 14: Build & Verify

**Files:**
- None (verification only)

- [ ] **Step 1: Build backend**

```bash
cd /d/goProjects/file-management-intranet
go build ./cmd/server/
```

Expected: Build succeeds with no errors.

- [ ] **Step 2: Build frontend**

```bash
cd /d/goProjects/file-management-intranet/web
npm run build
```

Expected: Build succeeds, output in `web/dist/`.

- [ ] **Step 3: Copy built frontend to static**

```bash
cp -r /d/goProjects/file-management-intranet/web/dist/* /d/goProjects/file-management-intranet/static/
```

- [ ] **Step 4: Final commit with built assets**

```bash
git add static/
git commit -m "chore: rebuild frontend with registration and admin features"
```
