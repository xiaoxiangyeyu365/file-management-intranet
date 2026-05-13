// internal/service/auth.go
package service

import (
	"cloudbox/internal/config"
	"cloudbox/internal/model"
	"cloudbox/internal/repository"
	"cloudbox/internal/util/crypto"
	"context"
	"errors"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserNotFound       = errors.New("user not found")
	ErrSamePassword       = errors.New("new password must be different")
)

type AuthService struct {
	userRepo *repository.UserRepository
}

func NewAuthService(userRepo *repository.UserRepository) *AuthService {
	return &AuthService{userRepo: userRepo}
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

	if !crypto.CheckPassword(password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	// Check if using default password
	cfg := config.Get()
	requireChange := password == cfg.Admin.Password

	token, err := crypto.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		return nil, err
	}

	return &LoginResponse{
		Token:               token,
		RequirePasswordChange: requireChange,
	}, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID int64, oldPwd, newPwd string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	if !crypto.CheckPassword(oldPwd, user.PasswordHash) {
		return ErrInvalidCredentials
	}

	if oldPwd == newPwd {
		return ErrSamePassword
	}

	newHash, err := crypto.HashPassword(newPwd)
	if err != nil {
		return err
	}

	return s.userRepo.UpdatePassword(ctx, userID, newHash)
}

func (s *AuthService) GetUserByID(ctx context.Context, userID int64) (*model.User, error) {
	return s.userRepo.FindByID(ctx, userID)
}
