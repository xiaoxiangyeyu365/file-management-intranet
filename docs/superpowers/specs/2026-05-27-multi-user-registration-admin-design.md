# Multi-User Registration & Admin Management Design

## 1. Overview

Add user registration with admin approval flow, and an admin panel for user management (CRUD, role/status control, password reset). Ensures proper access control with admin-only middleware and cascade file deletion on user removal.

## 2. User Model Changes

### 2.1 Database Schema

`users` table additions:

```sql
ALTER TABLE users ADD COLUMN status VARCHAR(20) DEFAULT 'approved';
ALTER TABLE users ADD COLUMN password_changed BOOLEAN DEFAULT true;
```

### 2.2 Status Constants

```go
// internal/model/user.go
const (
    UserStatusPending  = "pending"
    UserStatusApproved = "approved"
    UserStatusDisabled = "disabled"
)
```

### 2.3 Updated User Struct

```go
type User struct {
    ID              int64     `json:"id"`
    Username        string    `json:"username"`
    PasswordHash    string    `json:"-"`
    Role            string    `json:"role"`
    Status          string    `json:"status"`
    PasswordChanged bool      `json:"passwordChanged"`
    CreatedAt       time.Time `json:"createdAt"`
}
```

## 3. Configuration

```yaml
auth:
  registration: true        # Allow public registration (false = admin-only creation)
  approval_required: true   # Public registrations require admin approval (false = auto-approved)
```

Config struct addition:

```go
type AuthConfig struct {
    Registration      bool `yaml:"registration"`
    ApprovalRequired  bool `yaml:"approval_required"`
}
```

## 4. API Design

### 4.1 Public Registration

**Endpoint:** `POST /api/auth/register`

**Request:**
```json
{
  "username": "user1",
  "password": "password123"
}
```

**Validation:**
- Username: 3-50 chars, alphanumeric + underscore only
- Password: minimum 6 characters
- Username uniqueness check

**Response (success):**
```json
{
  "code": 0,
  "message": "注册成功，等待管理员审批"
}
```

**Response (registration disabled):**
```json
{
  "code": 403,
  "message": "注册功能已关闭"
}
```

**Logic:**
- Check `auth.registration` config; return 403 if disabled
- Create user with `status = "pending"` if `approval_required`, else `"approved"`
- Do NOT return token (user cannot login until approved)
- `password_changed = true` for self-registered users

### 4.2 Login Status Check

**Endpoint:** `POST /api/auth/login` (modify existing)

Add status check after password validation:

```go
if user.Status == model.UserStatusPending {
    return nil, ErrAccountPending  // "账号待审批，请联系管理员"
}
if user.Status == model.UserStatusDisabled {
    return nil, ErrAccountDisabled // "账号已被禁用"
}
```

Also check `password_changed` field to set `requirePasswordChange` response flag.

### 4.3 Admin User Management

All admin endpoints require `JWTMiddleware` + `AdminMiddleware`.

#### 4.3.1 List Users

**Endpoint:** `GET /api/admin/users`

**Query params:**
- `status` (optional): filter by status (pending/approved/disabled)

**Response:**
```json
{
  "code": 0,
  "data": {
    "users": [
      {
        "id": 1,
        "username": "admin",
        "role": "admin",
        "status": "approved",
        "passwordChanged": true,
        "createdAt": "2026-05-27T10:00:00Z"
      }
    ],
    "total": 5,
    "pendingCount": 2
  }
}
```

#### 4.3.2 Create User

**Endpoint:** `POST /api/admin/users`

**Request:**
```json
{
  "username": "user1",
  "password": "default123",
  "role": "user"
}
```

**Logic:**
- Create user with `status = "approved"` (admin-created users skip approval)
- Set `password_changed = false` (forces password change on first login)
- Hash password with bcrypt

#### 4.3.3 Update User

**Endpoint:** `PUT /api/admin/users/:id`

**Request:**
```json
{
  "role": "admin",
  "status": "approved"
}
```

**Logic:**
- Cannot modify own role (admin can't demote themselves)
- Used for: approve (pending→approved), disable (approved→disabled), re-enable (disabled→approved), change role

#### 4.3.4 Reset Password

**Endpoint:** `PUT /api/admin/users/:id/password`

**Request:**
```json
{
  "newPassword": "newpass123"
}
```

**Logic:**
- Hash new password with bcrypt
- Set `password_changed = false` (forces change on next login)
- Return success

#### 4.3.5 Delete User

**Endpoint:** `DELETE /api/admin/users/:id`

**Logic (in transaction):**
1. Cannot delete self
2. Query all files owned by user (`owner_id = userID`)
3. For each file, apply permanent delete logic:
   - Recursively find all descendants
   - Count physical file references
   - Delete file records
   - Decrement `physical_files.ref_count` atomically
   - If `ref_count <= 0`: delete physical file + thumbnail + physical_files record
4. Delete user's `clipboard_records`
5. Delete `users` record
6. Return `{ "deletedFiles": N, "deletedFolders": M }`

**Response:**
```json
{
  "code": 0,
  "data": {
    "deletedFiles": 15,
    "deletedFolders": 3
  }
}
```

## 5. Middleware

### 5.1 AdminMiddleware

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

### 5.2 Route Registration

```go
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

## 6. Service Layer

### 6.1 AuthService Changes

```go
// New error variables
var (
    ErrAccountPending    = errors.New("account pending approval")
    ErrAccountDisabled   = errors.New("account has been disabled")
    ErrRegistrationClosed = errors.New("registration is disabled")
    ErrInvalidUsername   = errors.New("username must be 3-50 alphanumeric characters")
)
```

### 6.2 AdminService (new)

```go
type AdminService struct {
    userRepo     *repository.UserRepository
    fileService  *FileService
}

func (s *AdminService) ListUsers(ctx context.Context, status string) ([]model.User, int64, error)
func (s *AdminService) CreateUser(ctx context.Context, username, password, role string) (*model.User, error)
func (s *AdminService) UpdateUser(ctx context.Context, userID int64, role, status string) error
func (s *AdminService) ResetPassword(ctx context.Context, userID int64, newPassword string) error
func (s *AdminService) DeleteUser(ctx context.Context, userID int64) (deletedFiles, deletedFolders int, err error)
```

### 6.3 UserRepository Changes

```go
func (r *UserRepository) FindAll(ctx context.Context, status string) ([]model.User, error)
func (r *UserRepository) CountByStatus(ctx context.Context, status string) (int64, error)
func (r *UserRepository) UpdateStatus(ctx context.Context, userID int64, status string) error
func (r *UserRepository) UpdateRole(ctx context.Context, userID int64, role string) error
func (r *UserRepository) UpdatePasswordChanged(ctx context.Context, userID int64, changed bool) error
func (r *UserRepository) DeleteByID(ctx context.Context, userID int64) error
```

## 7. Frontend Design

### 7.1 Registration Page

**Route:** `/register`

**Layout:**
```
┌─────────────────────────────────┐
│         CloudBox 注册            │
│                                 │
│  用户名: [________________]     │
│  密  码: [________________]     │
│  确认密码: [________________]    │
│                                 │
│         [注册]                  │
│                                 │
│  已有账号？去登录                │
└─────────────────────────────────┘
```

**Post-registration:** Show "注册成功，等待管理员审批" message, do NOT redirect to login.

### 7.2 Admin Users Page

**Route:** `/admin/users`

**Layout:**
```
┌─────────────────────────────────────────────────────┐
│ AppHeader                                           │
├─────────┬───────────────────────────────────────────┤
│         │ 用户管理                        [创建用户] │
│         ├───────────────────────────────────────────┤
│ 侧边栏   │ [全部] [待审批(2)] [已禁用]              │
│         ├───────────────────────────────────────────┤
│ 全部文件 │ ☐  用户名  角色   状态    创建时间  操作  │
│ 回收站   │ ☐  admin   管理员  ✅正常  5/27     -    │
│ 云剪切板 │ ☐  user1   普通   🟡待审  5/27  审批 禁用│
│ 用户管理 │ ☐  user2   普通   ✅正常  5/26  改密 删除│
│         │                                          │
│         │ [批量审批]  共 3 个用户                    │
└─────────┴───────────────────────────────────────────┘
```

### 7.3 Sidebar Changes

AppSidebar.vue: Add "用户管理" entry, visible only when `user.role === 'admin'`.

### 7.4 Login Page Changes

- Add "注册账号" link (visible only when `auth.registration` config is true)
- Login error handling: show specific message for pending/disabled accounts

### 7.5 Delete Confirmation

```
┌─────────────────────────────────────────┐
│ ⚠️ 确认删除用户                          │
│                                         │
│ 删除用户 "user1" 将同时永久删除其所有    │
│ 文件（共 15 个文件，3 个文件夹），且      │
│ 不可恢复。                               │
│                                         │
│       [取消]    [确认删除]               │
└─────────────────────────────────────────┘
```

## 8. File Change List

| File | Change | Description |
|------|--------|-------------|
| `internal/model/user.go` | Modify | Add Status, PasswordChanged fields + constants |
| `internal/config/config.go` | Modify | Add AuthConfig struct |
| `configs/config.yaml` | Modify | Add auth section |
| `internal/handler/middleware.go` | Modify | Add AdminMiddleware |
| `internal/handler/auth.go` | Modify | Add Register handler, login status check |
| `internal/handler/admin.go` | Create | Admin user management handlers |
| `internal/service/auth.go` | Modify | Add Register method, status check in Login |
| `internal/service/admin.go` | Create | Admin user management service |
| `internal/repository/user.go` | Modify | Add user CRUD methods |
| `internal/repository/db.go` | Modify | Add migration for new columns |
| `cmd/server/main.go` | Modify | Register admin routes |
| `web/src/router/index.js` | Modify | Add /register, /admin/users routes |
| `web/src/views/RegisterView.vue` | Create | Registration page |
| `web/src/views/AdminUsersView.vue` | Create | Admin user management page |
| `web/src/stores/auth.js` | Modify | Add register action |
| `web/src/stores/admin.js` | Create | Admin user management store |
| `web/src/utils/api.js` | Modify | Add admin API calls |
| `web/src/components/Layout/AppSidebar.vue` | Modify | Add admin nav entry |
| `web/src/views/LoginView.vue` | Modify | Add register link, status error handling |
