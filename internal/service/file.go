// internal/service/file.go
package service

import (
	"archive/zip"
	"cloudbox/internal/model"
	"cloudbox/internal/repository"
	"cloudbox/internal/util/storage"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
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
	ErrIsFolder          = errors.New("cannot download a folder")
	ErrNoPhysicalContent = errors.New("file has no physical content")
	ErrNotImage          = errors.New("file is not an image")
)

const thumbnailSize = 200
const maxLockAge = 30 * time.Second

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

func (s *FileService) FindByName(ctx context.Context, userID, parentID int64, name string) (*model.File, error) {
	file, err := s.fileRepo.FindByNameAndParent(ctx, userID, parentID, name)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("failed to find file by name: %w", err)
	}
	return file, nil
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
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrInvalidTarget
		}
		return fmt.Errorf("failed to find target folder: %w", err)
	}
	if !target.IsFolder || target.DeletedAt.Valid {
		return ErrInvalidTarget
	}

	// Validate each file
	for _, fileID := range fileIDs {
		file, err := s.fileRepo.FindByIDAndOwner(ctx, fileID, userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFileNotFound
			}
			return fmt.Errorf("failed to find file: %w", err)
		}

		// Check circular reference
		isAncestor, err := s.fileRepo.IsAncestor(ctx, fileID, targetFolderID)
		if err != nil {
			log.Printf("failed to check circular reference: %v", err)
			return fmt.Errorf("failed to check circular reference: %w", err)
		}
		if isAncestor {
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
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				log.Printf("failed to find parent folder: %v", err)
			}
			// Parent doesn't exist or error, restore to root
			newParentID = nil
		} else if parent.DeletedAt.Valid {
			// Parent is deleted, restore to root
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

// DownloadFile returns file and physical file info for download
func (s *FileService) DownloadFile(ctx context.Context, userID, fileID int64) (*model.File, *model.PhysicalFile, error) {
	file, err := s.fileRepo.FindByIDAndOwner(ctx, fileID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrFileNotFound
		}
		return nil, nil, fmt.Errorf("failed to find file: %w", err)
	}

	if file.IsFolder {
		return nil, nil, ErrIsFolder
	}

	if !file.PhysicalID.Valid {
		return nil, nil, ErrNoPhysicalContent
	}

	pf, err := s.physicalRepo.FindByID(ctx, file.PhysicalID.Int64)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find physical file: %w", err)
	}

	return file, pf, nil
}

func (s *FileService) GetStorage() *storage.StorageManager {
	return s.storage
}

// Helper function
func NullInt64ToInt(nullInt sql.NullInt64) int64 {
	if nullInt.Valid {
		return nullInt.Int64
	}
	return 0
}

// GetThumbnail returns thumbnail path, generating if needed
func (s *FileService) GetThumbnail(ctx context.Context, userID, fileID int64) (string, error) {
	file, err := s.fileRepo.FindByIDAndOwner(ctx, fileID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrFileNotFound
		}
		return "", fmt.Errorf("failed to find file: %w", err)
	}

	if file.IsFolder || !file.PhysicalID.Valid {
		return "", ErrNotImage
	}

	pf, err := s.physicalRepo.FindByID(ctx, file.PhysicalID.Int64)
	if err != nil {
		return "", fmt.Errorf("failed to find physical file: %w", err)
	}

	// Check if image type
	if !isImageType(pf.MimeType) {
		return "", ErrNotImage
	}

	thumbnailPath := s.storage.ThumbnailPath(pf.ID)

	// Check if thumbnail already exists
	if _, err := os.Stat(thumbnailPath); err == nil {
		return thumbnailPath, nil
	}

	// Generate thumbnail with file lock
	if err := s.generateThumbnail(ctx, pf, thumbnailPath); err != nil {
		return "", err
	}

	return thumbnailPath, nil
}

func isImageType(mimeType string) bool {
	imageTypes := []string{"image/jpeg", "image/png", "image/gif"}
	for _, t := range imageTypes {
		if mimeType == t {
			return true
		}
	}
	return false
}

func (s *FileService) generateThumbnail(ctx context.Context, pf *model.PhysicalFile, thumbnailPath string) error {
	// Use file lock for concurrency safety
	lockPath := thumbnailPath + ".lock"

	// Retry loop for lock acquisition
	for i := 0; i < 10; i++ {
		f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			defer os.Remove(lockPath)
			defer f.Close()
			break
		}

		// Check for stale lock (process crashed while holding lock)
		if info, statErr := os.Stat(lockPath); statErr == nil {
			if time.Since(info.ModTime()) > maxLockAge {
				os.Remove(lockPath) // Clean up stale lock
				continue
			}
		}

		// Lock exists, check if thumbnail was generated
		if _, err := os.Stat(thumbnailPath); err == nil {
			return nil // Thumbnail was generated by another process
		}

		// Wait and retry
		if i < 9 {
			time.Sleep(100 * time.Millisecond)
		} else {
			return fmt.Errorf("failed to acquire lock after retries: %w", err)
		}
	}

	// Decode original image
	absPath := s.storage.ToAbsPath(pf.StoragePath)
	imgFile, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("failed to open image file %s (physical_id=%d): %w", pf.StoragePath, pf.ID, err)
	}
	defer imgFile.Close()

	var img image.Image
	switch pf.MimeType {
	case "image/png":
		img, err = png.Decode(imgFile)
	case "image/gif":
		img, err = gif.Decode(imgFile)
	default: // jpeg
		img, err = jpeg.Decode(imgFile)
	}
	if err != nil {
		return fmt.Errorf("failed to decode image (physical_id=%d): %w", pf.ID, err)
	}

	// Resize
	resized := resizeImage(img, thumbnailSize)

	// Ensure thumbnail directory exists
	if err := s.storage.EnsureParentDir(thumbnailPath); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	// Save as JPEG
	outFile, err := os.Create(thumbnailPath)
	if err != nil {
		return fmt.Errorf("failed to create thumbnail: %w", err)
	}
	defer outFile.Close()

	if err := jpeg.Encode(outFile, resized, &jpeg.Options{Quality: 85}); err != nil {
		return fmt.Errorf("failed to encode thumbnail: %w", err)
	}

	// Update thumbnail path in database
	if err := s.physicalRepo.UpdateThumbnail(ctx, pf.ID, thumbnailPath); err != nil {
		log.Printf("warning: failed to update thumbnail path for physical file %d: %v", pf.ID, err)
	}

	return nil
}

func resizeImage(img image.Image, maxSize int) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width <= maxSize && height <= maxSize {
		return img
	}

	var newWidth, newHeight int
	if width > height {
		newWidth = maxSize
		newHeight = height * maxSize / width
	} else {
		newHeight = maxSize
		newWidth = width * maxSize / height
	}

	// Ensure minimum dimensions
	if newWidth < 1 {
		newWidth = 1
	}
	if newHeight < 1 {
		newHeight = 1
	}

	// Use simple nearest-neighbor scaling
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			srcX := x * width / newWidth
			srcY := y * height / newHeight
			dst.Set(x, y, img.At(srcX+bounds.Min.X, srcY+bounds.Min.Y))
		}
	}
	return dst
}

// StreamFolderZip creates a zip file and streams it to writer
func (s *FileService) StreamFolderZip(ctx context.Context, userID, folderID int64, writer io.Writer) error {
	folder, err := s.fileRepo.FindByIDAndOwner(ctx, folderID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFileNotFound
		}
		return fmt.Errorf("failed to find folder: %w", err)
	}

	if !folder.IsFolder {
		return errors.New("not a folder")
	}

	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()

	// Recursively add files
	return s.addFolderToZip(ctx, zipWriter, folder, folder.Name)
}

func (s *FileService) addFolderToZip(ctx context.Context, zipWriter *zip.Writer, folder *model.File, basePath string) error {
	files, err := s.fileRepo.FindByParentAndOwner(ctx, folder.ID, folder.OwnerID, false)
	if err != nil {
		return fmt.Errorf("failed to list folder contents: %w", err)
	}

	for _, file := range files {
		fullPath := filepath.Join(basePath, file.Name)

		if file.IsFolder {
			// Recursively add subfolder
			if err := s.addFolderToZip(ctx, zipWriter, &file, fullPath); err != nil {
				return err
			}
		} else if file.PhysicalID.Valid {
			// Add file
			pf, err := s.physicalRepo.FindByID(ctx, file.PhysicalID.Int64)
			if err != nil {
				log.Printf("warning: failed to find physical file %d: %v", file.PhysicalID.Int64, err)
				continue
			}

			absPath := s.storage.ToAbsPath(pf.StoragePath)
			if err := s.addFileToZip(zipWriter, absPath, fullPath); err != nil {
				log.Printf("warning: failed to add file %s to zip: %v", fullPath, err)
				continue
			}
		}
	}

	return nil
}

func (s *FileService) addFileToZip(zipWriter *zip.Writer, absPath, zipPath string) error {
	file, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = zipPath
	header.Method = zip.Deflate

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(writer, file)
	return err
}

// EmptyTrash permanently deletes all files in trash for a user
func (s *FileService) EmptyTrash(ctx context.Context, userID int64) (int, error) {
	// Get all trashed files
	trashFiles, err := s.fileRepo.FindTrash(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to list trash: %w", err)
	}

	if len(trashFiles) == 0 {
		return 0, nil
	}

	// Collect all files including descendants
	var allFiles []model.File
	physicalRefCount := make(map[int64]int)

	for _, file := range trashFiles {
		allFiles = append(allFiles, file)

		// Get descendants
		descendants, err := s.fileRepo.FindAllDescendants(ctx, file.ID)
		if err != nil {
			log.Printf("warning: failed to get descendants for %d: %v", file.ID, err)
			continue
		}
		allFiles = append(allFiles, descendants...)
	}

	// Count physical file references
	for _, f := range allFiles {
		if !f.IsFolder && f.PhysicalID.Valid {
			physicalRefCount[f.PhysicalID.Int64]++
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

	return len(trashFiles), nil
}

// SearchFiles searches files by keyword with optional folder scope and sorting
func (s *FileService) SearchFiles(ctx context.Context, userID int64, keyword string, folderID *int64, sort string) ([]model.File, error) {
	if keyword == "" {
		return nil, errors.New("keyword is required")
	}

	// Validate sort parameter
	validSorts := map[string]bool{"relevance": true, "time": true, "name": true}
	if sort == "" {
		sort = "relevance"
	}
	if !validSorts[sort] {
		sort = "relevance"
	}

	return s.fileRepo.Search(ctx, userID, keyword, folderID, sort)
}
