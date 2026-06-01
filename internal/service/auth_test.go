package service

import (
	"context"
	"errors"
	"testing"

	"cloudbox/internal/model"
)

func TestAuthService_Register(t *testing.T) {
	tests := []struct {
		name       string
		username   string
		password   string
		regOpen    bool
		approvalReq bool
		setupMock  func(*mockUserRepo, *mockPasswordHasher)
		wantErr    error
		wantStatus string
	}{
		{
			name:     "registration_closed",
			username: "testuser",
			password: "password123",
			regOpen:  false,
			wantErr:  ErrRegistrationClosed,
		},
		{
			name:     "username_too_short",
			username: "ab",
			password: "password123",
			regOpen:  true,
			wantErr:  ErrInvalidUsername,
		},
		{
			name:     "username_too_long",
			username: "a_very_long_username_that_exceeds_fifty_characters_limit_ok",
			password: "password123",
			regOpen:  true,
			wantErr:  ErrInvalidUsername,
		},
		{
			name:     "username_invalid_chars",
			username: "user@name",
			password: "password123",
			regOpen:  true,
			wantErr:  ErrInvalidUsername,
		},
		{
			name:     "username_exists",
			username: "existinguser",
			password: "password123",
			regOpen:  true,
			setupMock: func(ur *mockUserRepo, ph *mockPasswordHasher) {
				ur.findByUsernameFn = func(ctx context.Context, username string) (*model.User, error) {
					return &model.User{ID: 1, Username: username}, nil
				}
			},
			wantErr: ErrUsernameExists,
		},
		{
			name:     "hash_error",
			username: "newuser",
			password: "password123",
			regOpen:  true,
			setupMock: func(ur *mockUserRepo, ph *mockPasswordHasher) {
				ur.findByUsernameFn = func(ctx context.Context, username string) (*model.User, error) {
					return nil, errors.New("not found")
				}
				ph.hashPasswordFn = func(password string) (string, error) {
					return "", errors.New("hash failed")
				}
			},
			wantErr: errors.New("hash failed"),
		},
		{
			name:        "success_approval_required",
			username:    "newuser",
			password:    "password123",
			regOpen:     true,
			approvalReq: true,
			setupMock: func(ur *mockUserRepo, ph *mockPasswordHasher) {
				ur.findByUsernameFn = func(ctx context.Context, username string) (*model.User, error) {
					return nil, errors.New("not found")
				}
				ph.hashPasswordFn = func(password string) (string, error) {
					return "hashed", nil
				}
				ur.createFn = func(ctx context.Context, user *model.User) error {
					if user.Status != model.UserStatusPending {
						t.Errorf("expected status pending, got %s", user.Status)
					}
					return nil
				}
			},
			wantStatus: model.UserStatusPending,
		},
		{
			name:     "success_no_approval",
			username: "newuser",
			password: "password123",
			regOpen:  true,
			setupMock: func(ur *mockUserRepo, ph *mockPasswordHasher) {
				ur.findByUsernameFn = func(ctx context.Context, username string) (*model.User, error) {
					return nil, errors.New("not found")
				}
				ph.hashPasswordFn = func(password string) (string, error) {
					return "hashed", nil
				}
				ur.createFn = func(ctx context.Context, user *model.User) error {
					if user.Status != model.UserStatusApproved {
						t.Errorf("expected status approved, got %s", user.Status)
					}
					return nil
				}
			},
			wantStatus: model.UserStatusApproved,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &mockUserRepo{}
			hasher := &mockPasswordHasher{}
			tokenGen := &mockTokenGenerator{}

			if tt.setupMock != nil {
				tt.setupMock(userRepo, hasher)
			}

			svc := NewAuthService(userRepo, hasher, tokenGen, tt.regOpen, tt.approvalReq, "admin123")
			err := svc.Register(context.Background(), tt.username, tt.password)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if tt.wantErr.Error() != err.Error() {
					t.Errorf("got error %q, want %q", err.Error(), tt.wantErr.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestAuthService_Login(t *testing.T) {
	tests := []struct {
		name       string
		username   string
		password   string
		adminPwd   string
		setupMock  func(*mockUserRepo, *mockPasswordHasher, *mockTokenGenerator)
		wantErr    error
		wantChange bool
	}{
		{
			name:     "user_not_found",
			username: "nobody",
			password: "password123",
			setupMock: func(ur *mockUserRepo, ph *mockPasswordHasher, tg *mockTokenGenerator) {
				ur.findByUsernameFn = func(ctx context.Context, username string) (*model.User, error) {
					return nil, errors.New("not found")
				}
			},
			wantErr: ErrInvalidCredentials,
		},
		{
			name:     "wrong_password",
			username: "testuser",
			password: "wrongpass",
			setupMock: func(ur *mockUserRepo, ph *mockPasswordHasher, tg *mockTokenGenerator) {
				ur.findByUsernameFn = func(ctx context.Context, username string) (*model.User, error) {
					return &model.User{ID: 1, PasswordHash: "hash", Status: model.UserStatusApproved, PasswordChanged: true}, nil
				}
				ph.checkPasswordFn = func(password, hash string) bool {
					return false
				}
			},
			wantErr: ErrInvalidCredentials,
		},
		{
			name:     "account_pending",
			username: "pendinguser",
			password: "password123",
			setupMock: func(ur *mockUserRepo, ph *mockPasswordHasher, tg *mockTokenGenerator) {
				ur.findByUsernameFn = func(ctx context.Context, username string) (*model.User, error) {
					return &model.User{ID: 1, PasswordHash: "hash", Status: model.UserStatusPending, PasswordChanged: true}, nil
				}
				ph.checkPasswordFn = func(password, hash string) bool {
					return true
				}
			},
			wantErr: ErrAccountPending,
		},
		{
			name:     "account_disabled",
			username: "disableduser",
			password: "password123",
			setupMock: func(ur *mockUserRepo, ph *mockPasswordHasher, tg *mockTokenGenerator) {
				ur.findByUsernameFn = func(ctx context.Context, username string) (*model.User, error) {
					return &model.User{ID: 1, PasswordHash: "hash", Status: model.UserStatusDisabled, PasswordChanged: true}, nil
				}
				ph.checkPasswordFn = func(password, hash string) bool {
					return true
				}
			},
			wantErr: ErrAccountDisabled,
		},
		{
			name:     "password_change_required",
			username: "newuser",
			password: "password123",
			setupMock: func(ur *mockUserRepo, ph *mockPasswordHasher, tg *mockTokenGenerator) {
				ur.findByUsernameFn = func(ctx context.Context, username string) (*model.User, error) {
					return &model.User{ID: 1, PasswordHash: "hash", Status: model.UserStatusApproved, PasswordChanged: false}, nil
				}
				ph.checkPasswordFn = func(password, hash string) bool { return true }
				tg.generateTokenFn = func(userID int64, username, role string) (string, error) {
					return "token", nil
				}
			},
			wantChange: true,
		},
		{
			name:     "admin_password_requires_change",
			username: "admin",
			password: "admin123",
			adminPwd: "admin123",
			setupMock: func(ur *mockUserRepo, ph *mockPasswordHasher, tg *mockTokenGenerator) {
				ur.findByUsernameFn = func(ctx context.Context, username string) (*model.User, error) {
					return &model.User{ID: 1, PasswordHash: "hash", Status: model.UserStatusApproved, PasswordChanged: true}, nil
				}
				ph.checkPasswordFn = func(password, hash string) bool { return true }
				tg.generateTokenFn = func(userID int64, username, role string) (string, error) {
					return "token", nil
				}
			},
			wantChange: true,
		},
		{
			name:     "success_normal",
			username: "testuser",
			password: "password123",
			setupMock: func(ur *mockUserRepo, ph *mockPasswordHasher, tg *mockTokenGenerator) {
				ur.findByUsernameFn = func(ctx context.Context, username string) (*model.User, error) {
					return &model.User{ID: 1, Username: "testuser", Role: "user", PasswordHash: "hash", Status: model.UserStatusApproved, PasswordChanged: true}, nil
				}
				ph.checkPasswordFn = func(password, hash string) bool { return true }
				tg.generateTokenFn = func(userID int64, username, role string) (string, error) {
					return "jwt-token", nil
				}
			},
			wantChange: false,
		},
		{
			name:     "token_generation_error",
			username: "testuser",
			password: "password123",
			setupMock: func(ur *mockUserRepo, ph *mockPasswordHasher, tg *mockTokenGenerator) {
				ur.findByUsernameFn = func(ctx context.Context, username string) (*model.User, error) {
					return &model.User{ID: 1, PasswordHash: "hash", Status: model.UserStatusApproved, PasswordChanged: true}, nil
				}
				ph.checkPasswordFn = func(password, hash string) bool { return true }
				tg.generateTokenFn = func(userID int64, username, role string) (string, error) {
					return "", errors.New("token failed")
				}
			},
			wantErr: errors.New("token failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &mockUserRepo{}
			hasher := &mockPasswordHasher{}
			tokenGen := &mockTokenGenerator{}
			tt.setupMock(userRepo, hasher, tokenGen)

			svc := NewAuthService(userRepo, hasher, tokenGen, true, false, tt.adminPwd)
			resp, err := svc.Login(context.Background(), tt.username, tt.password)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if tt.wantErr.Error() != err.Error() {
					t.Errorf("got error %q, want %q", err.Error(), tt.wantErr.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.RequirePasswordChange != tt.wantChange {
				t.Errorf("RequirePasswordChange = %v, want %v", resp.RequirePasswordChange, tt.wantChange)
			}
		})
	}
}

func TestAuthService_ChangePassword(t *testing.T) {
	tests := []struct {
		name      string
		oldPwd    string
		newPwd    string
		setupMock func(*mockUserRepo, *mockPasswordHasher)
		wantErr   error
	}{
		{
			name:   "user_not_found",
			oldPwd: "oldpass",
			newPwd: "newpass",
			setupMock: func(ur *mockUserRepo, ph *mockPasswordHasher) {
				ur.findByIDFn = func(ctx context.Context, id int64) (*model.User, error) {
					return nil, errors.New("not found")
				}
			},
			wantErr: ErrUserNotFound,
		},
		{
			name:   "wrong_old_password",
			oldPwd: "wrongold",
			newPwd: "newpass",
			setupMock: func(ur *mockUserRepo, ph *mockPasswordHasher) {
				ur.findByIDFn = func(ctx context.Context, id int64) (*model.User, error) {
					return &model.User{ID: 1, PasswordHash: "hash"}, nil
				}
				ph.checkPasswordFn = func(password, hash string) bool {
					return false
				}
			},
			wantErr: ErrInvalidCredentials,
		},
		{
			name:   "same_password",
			oldPwd: "samepass",
			newPwd: "samepass",
			setupMock: func(ur *mockUserRepo, ph *mockPasswordHasher) {
				ur.findByIDFn = func(ctx context.Context, id int64) (*model.User, error) {
					return &model.User{ID: 1, PasswordHash: "hash"}, nil
				}
				ph.checkPasswordFn = func(password, hash string) bool {
					return true
				}
			},
			wantErr: ErrSamePassword,
		},
		{
			name:   "hash_error",
			oldPwd: "oldpass",
			newPwd: "newpass",
			setupMock: func(ur *mockUserRepo, ph *mockPasswordHasher) {
				ur.findByIDFn = func(ctx context.Context, id int64) (*model.User, error) {
					return &model.User{ID: 1, PasswordHash: "hash"}, nil
				}
				ph.checkPasswordFn = func(password, hash string) bool {
					return true
				}
				ph.hashPasswordFn = func(password string) (string, error) {
					return "", errors.New("hash failed")
				}
			},
			wantErr: errors.New("hash failed"),
		},
		{
			name:   "success",
			oldPwd: "oldpass",
			newPwd: "newpass",
			setupMock: func(ur *mockUserRepo, ph *mockPasswordHasher) {
				ur.findByIDFn = func(ctx context.Context, id int64) (*model.User, error) {
					return &model.User{ID: 1, PasswordHash: "hash"}, nil
				}
				ph.checkPasswordFn = func(password, hash string) bool {
					return true
				}
				ph.hashPasswordFn = func(password string) (string, error) {
					return "newhash", nil
				}
				ur.updatePasswordFn = func(ctx context.Context, userID int64, passwordHash string) error {
					if passwordHash != "newhash" {
						t.Errorf("expected newhash, got %s", passwordHash)
					}
					return nil
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &mockUserRepo{}
			hasher := &mockPasswordHasher{}
			tt.setupMock(userRepo, hasher)

			svc := NewAuthService(userRepo, hasher, &mockTokenGenerator{}, true, false, "admin123")
			err := svc.ChangePassword(context.Background(), 1, tt.oldPwd, tt.newPwd)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if tt.wantErr.Error() != err.Error() {
					t.Errorf("got error %q, want %q", err.Error(), tt.wantErr.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
