// internal/service/auth.go
package service

import (
	"cloudbox/internal/model"
	"context"
	"errors"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserNotFound       = errors.New("user not found")
	ErrSamePassword       = errors.New("new password must be different")
	ErrAccountPending     = errors.New("account pending approval")
	ErrAccountDisabled    = errors.New("account has been disabled")
	ErrRegistrationClosed = errors.New("registration is disabled")
	ErrInvalidUsername    = errors.New("username must be 3-50 alphanumeric characters")
	ErrUsernameExists     = errors.New("username already exists")
)

type AuthService struct {
	userRepo      UserRepository
	hasher        PasswordHasher
	tokenGen      TokenGenerator
	registration  bool
	approvalReq   bool
	adminPassword string
	audit         AuditRecorder
}

func NewAuthService(
	userRepo UserRepository,
	hasher PasswordHasher,
	tokenGen TokenGenerator,
	registration bool,
	approvalReq bool,
	adminPassword string,
	audit AuditRecorder,
) *AuthService {
	return &AuthService{
		userRepo:      userRepo,
		hasher:        hasher,
		tokenGen:      tokenGen,
		registration:  registration,
		approvalReq:   approvalReq,
		adminPassword: adminPassword,
		audit:         audit,
	}
}

func (s *AuthService) Register(ctx context.Context, username, password string) error {
	if !s.registration {
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

	hash, err := s.hasher.HashPassword(password)
	if err != nil {
		return err
	}

	status := model.UserStatusApproved
	if s.approvalReq {
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

type LoginResponse struct {
	Token               string `json:"token"`
	RequirePasswordChange bool  `json:"requirePasswordChange"`
}

func (s *AuthService) Login(ctx context.Context, username, password string) (*LoginResponse, error) {
	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if !s.hasher.CheckPassword(password, user.PasswordHash) {
		s.audit.Record(ctx, "user.login_failed", "user", 0, username, "")
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
	if password == s.adminPassword {
		requireChange = true
	}

	token, err := s.tokenGen.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, err
	}

	s.audit.Record(ctx, "user.login", "user", user.ID, user.Username, "")

	return &LoginResponse{
		Token:                token,
		RequirePasswordChange: requireChange,
	}, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID int64, oldPwd, newPwd string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	if !s.hasher.CheckPassword(oldPwd, user.PasswordHash) {
		return ErrInvalidCredentials
	}

	if oldPwd == newPwd {
		return ErrSamePassword
	}

	newHash, err := s.hasher.HashPassword(newPwd)
	if err != nil {
		return err
	}

	if err := s.userRepo.UpdatePassword(ctx, userID, newHash); err != nil {
		return err
	}

	s.audit.Record(ctx, "user.change_password", "user", userID, "", "")

	return nil
}

func (s *AuthService) GetUserByID(ctx context.Context, userID int64) (*model.User, error) {
	return s.userRepo.FindByID(ctx, userID)
}
