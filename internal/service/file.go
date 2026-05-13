// internal/service/file.go
package service

import (
	"cloudbox/internal/model"
	"cloudbox/internal/repository"
	"cloudbox/internal/util/storage"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"
)

var (
	ErrFileNotFound      = errors.New("file not found")
	ErrForbidden         = errors.New("access denied")
	ErrNameConflict      = errors.New("file name already exists")
	ErrCircularReference = errors.New("cannot move to a subfolder")
	ErrInvalidTarget     = errors.New("invalid target folder")
)

type FileService struct {
	fileRepo     *repository.FileRepository
	physicalRepo *repository.PhysicalFileRepository
	storage      *storage.StorageManager
}

func NewFileService(
	fileRepo *repository.FileRepository,
	physicalRepo *repository.PhysicalFileRepository,
	storage *storage.StorageManager,
) *FileService {
	return &FileService{
		fileRepo:     fileRepo,
		physicalRepo: physicalRepo,
		storage:      storage,
	}
}

func (s *FileService) ListFiles(ctx context.Context, userID, folderID int64) ([]model.File, error) {
	return s.fileRepo.FindByParentAndOwner(ctx, folderID, userID, false)
}

func (s *FileService) GetFile(ctx context.Context, userID, fileID int64) (*model.File, error) {
	file, err := s.fileRepo.FindByIDAndOwner(ctx, fileID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("failed to get file: %w", err)
	}
	return file, nil
}

func (s *FileService) CreateFolder(ctx context.Context, userID, parentID int64, name string) (*model.File, error) {
	// Check name conflict
	if s.fileRepo.ExistsByName(ctx, userID, parentID, name) {
		return nil, ErrNameConflict
	}

	folder := &model.File{
		Name:      name,
		ParentID:  repository.NullInt64(parentID),
		OwnerID:   userID,
		IsFolder:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.fileRepo.Create(ctx, folder); err != nil {
		return nil, err
	}

	return folder, nil
}

func (s *FileService) Rename(ctx context.Context, userID, fileID int64, newName string) error {
	file, err := s.fileRepo.FindByIDAndOwner(ctx, fileID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFileNotFound
		}
		return fmt.Errorf("failed to find file for rename: %w", err)
	}

	// Check name conflict (exclude self)
	// Note: ExistsByName doesn't exclude, but rename to same name is ok
	if file.Name == newName {
		return nil
	}

	if s.fileRepo.ExistsByName(ctx, userID, NullInt64ToInt(file.ParentID), newName) {
		return ErrNameConflict
	}

	file.Name = newName
	file.UpdatedAt = time.Now()

	return s.fileRepo.Update(ctx, file)
}

func (s *FileService) MoveToTrash(ctx context.Context, userID, fileID int64) error {
	file, err := s.fileRepo.FindByIDAndOwner(ctx, fileID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFileNotFound
		}
		return fmt.Errorf("failed to find file for trash: %w", err)
	}

	return s.fileRepo.SoftDelete(ctx, file.ID)
}

func (s *FileService) MoveFiles(ctx context.Context, userID int64, fileIDs []int64, targetFolderID int64) error {
	// Validate target folder
	target, err := s.fileRepo.FindByIDAndOwner(ctx, targetFolderID, userID)
	if err != nil || !target.IsFolder || target.DeletedAt.Valid {
		return ErrInvalidTarget
	}

	// Validate each file
	for _, fileID := range fileIDs {
		file, err := s.fileRepo.FindByIDAndOwner(ctx, fileID, userID)
		if err != nil {
			return ErrFileNotFound
		}

		// Check circular reference
		isAncestor, err := s.fileRepo.IsAncestor(ctx, fileID, targetFolderID)
		if err != nil || isAncestor {
			return ErrCircularReference
		}

		// Check name conflict
		if s.fileRepo.ExistsByName(ctx, userID, targetFolderID, file.Name) {
			return ErrNameConflict
		}
	}

	return s.fileRepo.BatchUpdateParent(ctx, fileIDs, targetFolderID)
}

func (s *FileService) ListTrash(ctx context.Context, userID int64) ([]model.File, error) {
	return s.fileRepo.FindTrash(ctx, userID)
}

func (s *FileService) RestoreFile(ctx context.Context, userID, fileID int64) error {
	file, err := s.fileRepo.FindByIDAndOwner(ctx, fileID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFileNotFound
		}
		return fmt.Errorf("failed to find file for restore: %w", err)
	}

	var newParentID *int64
	newName := file.Name

	// Check if original parent exists
	if file.ParentID.Valid {
		parent, err := s.fileRepo.FindByID(ctx, file.ParentID.Int64)
		if err != nil || parent.DeletedAt.Valid {
			// Parent doesn't exist or is deleted, restore to root
			newParentID = nil
		} else {
			pid := file.ParentID.Int64
			newParentID = &pid
		}
	}

	// Check name conflict
	var parentID int64
	if newParentID != nil {
		parentID = *newParentID
	}

	if s.fileRepo.ExistsByName(ctx, userID, parentID, file.Name) {
		// Auto rename
		ext := filepath.Ext(file.Name)
		base := file.Name[:len(file.Name)-len(ext)]
		newName = fmt.Sprintf("%s (恢复)_%s%s", base, time.Now().Format("20060102_150405"), ext)
	}

	return s.fileRepo.Restore(ctx, fileID, newParentID, newName)
}

func (s *FileService) PermanentDelete(ctx context.Context, userID, fileID int64) error {
	// TODO: Wrap all operations in a database transaction for atomicity.
	// The current repository pattern doesn't expose transactions. Consider adding
	// a BeginTx method or using a transaction callback pattern.

	file, err := s.fileRepo.FindByIDAndOwner(ctx, fileID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFileNotFound
		}
		return fmt.Errorf("failed to find file for permanent delete: %w", err)
	}

	// Get all descendants
	descendants, err := s.fileRepo.FindAllDescendants(ctx, fileID)
	if err != nil {
		return err
	}

	allFiles := append(descendants, *file)

	// Count physical file deletions
	physicalRefCount := make(map[int64]int)

	for _, f := range allFiles {
		if !f.IsFolder && f.PhysicalID.Valid {
			physicalRefCount[f.PhysicalID.Int64]++
		}
	}

	// Delete file records first
	for _, f := range allFiles {
		if err := s.fileRepo.Delete(ctx, f.ID); err != nil {
			return err
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

			// Delete physical file
			absPath := s.storage.ToAbsPath(pf.StoragePath)
			if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
				log.Printf("warning: failed to delete physical file %s: %v", absPath, err)
			}

			// Delete thumbnail
			if pf.ThumbnailPath != "" {
				if err := os.Remove(pf.ThumbnailPath); err != nil && !os.IsNotExist(err) {
					log.Printf("warning: failed to delete thumbnail %s: %v", pf.ThumbnailPath, err)
				}
			}

			// Delete physical file record
			if err := s.physicalRepo.Delete(ctx, pid); err != nil {
				log.Printf("warning: failed to delete physical file record %d: %v", pid, err)
			}
		}
	}

	return nil
}

// Helper function
func NullInt64ToInt(nullInt sql.NullInt64) int64 {
	if nullInt.Valid {
		return nullInt.Int64
	}
	return 0
}
