// internal/service/admin.go
package service

import (
	"cloudbox/internal/model"
	"cloudbox/internal/repository"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"
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
	hasher        PasswordHasher
	audit         AuditRecorder
	auditRepo     *repository.AuditRepository
}

func NewAdminService(
	userRepo *repository.UserRepository,
	fileRepo *repository.FileRepository,
	physicalRepo *repository.PhysicalFileRepository,
	clipboardRepo *repository.ClipboardRepository,
	fileService *FileService,
	hasher PasswordHasher,
	audit AuditRecorder,
	auditRepo *repository.AuditRepository,
) *AdminService {
	return &AdminService{
		userRepo:      userRepo,
		fileRepo:      fileRepo,
		physicalRepo:  physicalRepo,
		clipboardRepo: clipboardRepo,
		fileService:   fileService,
		hasher:        hasher,
		audit:         audit,
		auditRepo:     auditRepo,
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

	hash, err := s.hasher.HashPassword(password)
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

	s.audit.Record(ctx, "admin.create_user", "user", user.ID, user.Username, fmt.Sprintf(`{"role":"%s"}`, role))

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
		s.audit.Record(ctx, "admin.update_status", "user", userID, "", fmt.Sprintf(`{"status":"%s"}`, status))
	}
	return nil
}

func (s *AdminService) ResetPassword(ctx context.Context, userID int64, newPassword string) error {
	hash, err := s.hasher.HashPassword(newPassword)
	if err != nil {
		return err
	}

	if err := s.userRepo.UpdatePassword(ctx, userID, hash); err != nil {
		return err
	}

	// Force password change on next login
	if err := s.userRepo.UpdatePasswordChanged(ctx, userID, false); err != nil {
		return err
	}

	s.audit.Record(ctx, "admin.reset_password", "user", userID, "", "")

	return nil
}

type DeleteUserResult struct {
	DeletedFiles   int `json:"deletedFiles"`
	DeletedFolders int `json:"deletedFolders"`
}

func (s *AdminService) DeleteUser(ctx context.Context, adminID, userID int64) (*DeleteUserResult, error) {
	if adminID == userID {
		return nil, ErrCannotDeleteSelf
	}

	// Get all files owned by this user
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

	s.audit.Record(ctx, "admin.delete_user", "user", userID, "", fmt.Sprintf(`{"deletedFiles":%d,"deletedFolders":%d}`, result.DeletedFiles, result.DeletedFolders))

	return result, nil
}

func removeFileIfExists(path string) error {
	if _, err := os.Stat(path); err == nil {
		return os.Remove(path)
	}
	return nil
}

func (s *AdminService) SetUserQuota(ctx context.Context, userID int64, quota *int64) error {
	if err := s.userRepo.SetQuota(ctx, userID, quota); err != nil {
		return err
	}

	quotaStr := "global"
	if quota != nil {
		if *quota == 0 {
			quotaStr = "unlimited"
		} else {
			quotaStr = fmt.Sprintf("%d", *quota)
		}
	}
	s.audit.Record(ctx, "admin.set_quota", "user", userID, "", fmt.Sprintf(`{"quota":"%s"}`, quotaStr))

	return nil
}

func (s *AdminService) GetAllUserStorageUsage(ctx context.Context) (map[int64]int64, error) {
	return s.physicalRepo.CalculateAllUserStorageUsage(ctx)
}

func (s *AdminService) ListAuditLogs(ctx context.Context, action string, userID int64, targetType string, keyword string, startDate, endDate *time.Time, page, pageSize int) ([]model.AuditLog, int64, error) {
	return s.auditRepo.FindWithFilter(ctx, action, userID, targetType, keyword, startDate, endDate, page, pageSize)
}
