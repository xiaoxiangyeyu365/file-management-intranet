// internal/service/file.go
package service

import (
	"archive/zip"
	"cloudbox/internal/model"
	"cloudbox/internal/repository"
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
	fileRepo     FileRepository
	physicalRepo PhysicalFileRepository
	storage      Storage
	audit        AuditRecorder
	fileTagRepo  *repository.FileTagRepository
}

func NewFileService(
	fileRepo FileRepository,
	physicalRepo PhysicalFileRepository,
	storage Storage,
	audit AuditRecorder,
	fileTagRepo *repository.FileTagRepository,
) *FileService {
	return &FileService{
		fileRepo:     fileRepo,
		physicalRepo: physicalRepo,
		storage:      storage,
		audit:        audit,
		fileTagRepo:  fileTagRepo,
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
		OwnerID:   userID,
		IsFolder:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if parentID > 0 {
		folder.ParentID = sql.NullInt64{Int64: parentID, Valid: true}
	}

	if err := s.fileRepo.Create(ctx, folder); err != nil {
		return nil, err
	}

	s.audit.Record(ctx, "folder.create", "folder", folder.ID, folder.Name, "")

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
	if file.Name == newName {
		return nil
	}

	parentID := int64(0)
	if file.ParentID.Valid {
		parentID = file.ParentID.Int64
	}
	if s.fileRepo.ExistsByName(ctx, userID, parentID, newName) {
		return ErrNameConflict
	}

	oldName := file.Name
	file.Name = newName
	file.UpdatedAt = time.Now()

	if err := s.fileRepo.Update(ctx, file); err != nil {
		return err
	}

	targetType := "file"
	if file.IsFolder {
		targetType = "folder"
	}
	s.audit.Record(ctx, "file.rename", targetType, file.ID, file.Name, fmt.Sprintf(`{"oldName":"%s"}`, oldName))

	return nil
}

func (s *FileService) MoveToTrash(ctx context.Context, userID, fileID int64) error {
	file, err := s.fileRepo.FindByIDAndOwner(ctx, fileID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFileNotFound
		}
		return fmt.Errorf("failed to find file for trash: %w", err)
	}

	if err := s.fileRepo.SoftDelete(ctx, file.ID); err != nil {
		return err
	}

	targetType := "file"
	if file.IsFolder {
		targetType = "folder"
	}
	s.audit.Record(ctx, "file.delete", targetType, file.ID, file.Name, "")

	return nil
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

	if err := s.fileRepo.BatchUpdateParent(ctx, fileIDs, targetFolderID); err != nil {
		return err
	}

	s.audit.Record(ctx, "file.move", "file", 0, "", fmt.Sprintf(`{"count":%d,"targetFolder":%d}`, len(fileIDs), targetFolderID))

	return nil
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
			newParentID = nil
		} else if parent.DeletedAt.Valid {
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
		ext := filepath.Ext(file.Name)
		base := file.Name[:len(file.Name)-len(ext)]
		newName = fmt.Sprintf("%s (恢复)_%s%s", base, time.Now().Format("20060102_150405"), ext)
	}

	if err := s.fileRepo.Restore(ctx, fileID, newParentID, newName); err != nil {
		return err
	}

	s.audit.Record(ctx, "file.restore", "file", fileID, file.Name, "")

	return nil
}

func (s *FileService) PermanentDelete(ctx context.Context, userID, fileID int64) error {
	file, err := s.fileRepo.FindByIDAndOwner(ctx, fileID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrFileNotFound
		}
		return fmt.Errorf("failed to find file for permanent delete: %w", err)
	}

	descendants, err := s.fileRepo.FindAllDescendants(ctx, fileID)
	if err != nil {
		return err
	}

	allFiles := append(descendants, *file)
	physicalRefCount := make(map[int64]int)

	for _, f := range allFiles {
		if !f.IsFolder && f.ContentRef != 0 {
			physicalRefCount[f.ContentRef]++
		}
	}

	// Delete tags for all files
	if s.fileTagRepo != nil {
		for _, f := range allFiles {
			if err := s.fileTagRepo.DeleteByFileID(ctx, f.ID); err != nil {
				log.Printf("warning: failed to delete tags for file %d: %v", f.ID, err)
			}
		}
	}

	// Delete file records first
	for _, f := range allFiles {
		if err := s.fileRepo.Delete(ctx, f.ID); err != nil {
			return err
		}
		targetType := "file"
		if f.IsFolder {
			targetType = "folder"
		}
		s.audit.Record(ctx, "file.permanent_delete", targetType, f.ID, f.Name, "")
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

			absPath := s.storage.ToAbsPath(pf.StoragePath)
			if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
				log.Printf("warning: failed to delete physical file %s: %v", absPath, err)
			}

			if pf.ThumbnailPath != "" {
				if err := os.Remove(pf.ThumbnailPath); err != nil && !os.IsNotExist(err) {
					log.Printf("warning: failed to delete thumbnail %s: %v", pf.ThumbnailPath, err)
				}
			}

			if err := s.physicalRepo.Delete(ctx, pid); err != nil {
				log.Printf("warning: failed to delete physical file record %d: %v", pid, err)
			}
		}
	}

	return nil
}

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

	if file.ContentRef == 0 {
		return nil, nil, ErrNoPhysicalContent
	}

	pf, err := s.physicalRepo.FindByID(ctx, file.ContentRef)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find physical file: %w", err)
	}

	s.audit.Record(ctx, "file.download", "file", file.ID, file.Name, "")

	return file, pf, nil
}

func (s *FileService) GetStorage() Storage {
	return s.storage
}

func (s *FileService) GetThumbnail(ctx context.Context, userID, fileID int64) (string, error) {
	file, err := s.fileRepo.FindByIDAndOwner(ctx, fileID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrFileNotFound
		}
		return "", fmt.Errorf("failed to find file: %w", err)
	}

	if file.IsFolder || file.ContentRef == 0 {
		return "", ErrNotImage
	}

	pf, err := s.physicalRepo.FindByID(ctx, file.ContentRef)
	if err != nil {
		return "", fmt.Errorf("failed to find physical file: %w", err)
	}

	if !isImageType(pf.MimeType) {
		return "", ErrNotImage
	}

	thumbnailPath := s.storage.ThumbnailPath(pf.ID)

	if _, err := os.Stat(thumbnailPath); err == nil {
		return thumbnailPath, nil
	}

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
	lockPath := thumbnailPath + ".lock"

	for i := 0; i < 10; i++ {
		f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err == nil {
			defer os.Remove(lockPath)
			defer f.Close()
			break
		}

		if info, statErr := os.Stat(lockPath); statErr == nil {
			if time.Since(info.ModTime()) > maxLockAge {
				os.Remove(lockPath)
				continue
			}
		}

		if _, err := os.Stat(thumbnailPath); err == nil {
			return nil
		}

		if i < 9 {
			time.Sleep(100 * time.Millisecond)
		} else {
			return fmt.Errorf("failed to acquire lock after retries: %w", err)
		}
	}

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
	default:
		img, err = jpeg.Decode(imgFile)
	}
	if err != nil {
		return fmt.Errorf("failed to decode image (physical_id=%d): %w", pf.ID, err)
	}

	resized := resizeImage(img, thumbnailSize)

	if err := s.storage.EnsureParentDir(thumbnailPath); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	outFile, err := os.Create(thumbnailPath)
	if err != nil {
		return fmt.Errorf("failed to create thumbnail: %w", err)
	}
	defer outFile.Close()

	if err := jpeg.Encode(outFile, resized, &jpeg.Options{Quality: 85}); err != nil {
		return fmt.Errorf("failed to encode thumbnail: %w", err)
	}

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

	if newWidth < 1 {
		newWidth = 1
	}
	if newHeight < 1 {
		newHeight = 1
	}

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

	return s.addFolderToZip(ctx, zipWriter, folder, folder.Name)
}

func (s *FileService) StreamFolderZipByID(ctx context.Context, folderID int64, writer io.Writer) error {
	folder, err := s.fileRepo.FindByID(ctx, folderID)
	if err != nil {
		return fmt.Errorf("failed to find folder: %w", err)
	}
	if !folder.IsFolder {
		return errors.New("not a folder")
	}

	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()

	return s.addFolderToZip(ctx, zipWriter, folder, folder.Name)
}

func (s *FileService) StreamBatchZip(ctx context.Context, userID int64, fileIDs []int64, writer io.Writer) error {
	zipWriter := zip.NewWriter(writer)
	defer zipWriter.Close()

	for _, id := range fileIDs {
		file, err := s.fileRepo.FindByIDAndOwner(ctx, id, userID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return fmt.Errorf("failed to find file %d: %w", id, err)
		}

		if file.DeletedAt.Valid {
			continue
		}

		if file.IsFolder {
			if err := s.addFolderToZip(ctx, zipWriter, file, file.Name); err != nil {
				log.Printf("warning: failed to add folder %s to batch zip: %v", file.Name, err)
				continue
			}
		} else if file.ContentRef != 0 {
			pf, err := s.physicalRepo.FindByID(ctx, file.ContentRef)
			if err != nil {
				log.Printf("warning: failed to find physical file %d: %v", file.ContentRef, err)
				continue
			}
			absPath := s.storage.ToAbsPath(pf.StoragePath)
			if err := s.addFileToZip(zipWriter, absPath, file.Name); err != nil {
				log.Printf("warning: failed to add file %s to batch zip: %v", file.Name, err)
				continue
			}
		}
	}

	return nil
}

func (s *FileService) addFolderToZip(ctx context.Context, zipWriter *zip.Writer, folder *model.File, basePath string) error {
	files, err := s.fileRepo.FindByParentAndOwner(ctx, folder.ID, folder.OwnerID, false)
	if err != nil {
		return fmt.Errorf("failed to list folder contents: %w", err)
	}

	for _, file := range files {
		fullPath := filepath.Join(basePath, file.Name)

		if file.IsFolder {
			if err := s.addFolderToZip(ctx, zipWriter, &file, fullPath); err != nil {
				return err
			}
		} else if file.ContentRef != 0 {
			pf, err := s.physicalRepo.FindByID(ctx, file.ContentRef)
			if err != nil {
				log.Printf("warning: failed to find physical file %d: %v", file.ContentRef, err)
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

func (s *FileService) EmptyTrash(ctx context.Context, userID int64) (int, error) {
	trashFiles, err := s.fileRepo.FindTrash(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to list trash: %w", err)
	}

	if len(trashFiles) == 0 {
		return 0, nil
	}

	var allFiles []model.File
	physicalRefCount := make(map[int64]int)

	for _, file := range trashFiles {
		allFiles = append(allFiles, file)

		descendants, err := s.fileRepo.FindAllDescendants(ctx, file.ID)
		if err != nil {
			log.Printf("warning: failed to get descendants for %d: %v", file.ID, err)
			continue
		}
		allFiles = append(allFiles, descendants...)
	}

	for _, f := range allFiles {
		if !f.IsFolder && f.ContentRef != 0 {
			physicalRefCount[f.ContentRef]++
		}
	}

	for _, f := range allFiles {
		if err := s.fileRepo.Delete(ctx, f.ID); err != nil {
			log.Printf("warning: failed to delete file record %d: %v", f.ID, err)
		}
	}

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

			absPath := s.storage.ToAbsPath(pf.StoragePath)
			if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
				log.Printf("warning: failed to delete physical file %s: %v", absPath, err)
			}

			if pf.ThumbnailPath != "" {
				if err := os.Remove(pf.ThumbnailPath); err != nil && !os.IsNotExist(err) {
					log.Printf("warning: failed to delete thumbnail %s: %v", pf.ThumbnailPath, err)
				}
			}

			if err := s.physicalRepo.Delete(ctx, pid); err != nil {
				log.Printf("warning: failed to delete physical file record %d: %v", pid, err)
			}
		}
	}

	s.audit.Record(ctx, "trash.empty", "trash", 0, "", fmt.Sprintf(`{"count":%d}`, len(trashFiles)))

	return len(trashFiles), nil
}

func (s *FileService) SearchFiles(ctx context.Context, userID int64, keyword string, folderID *int64, sort string) ([]model.File, error) {
	if keyword == "" {
		return nil, errors.New("keyword is required")
	}

	validSorts := map[string]bool{"relevance": true, "time": true, "name": true}
	if sort == "" {
		sort = "relevance"
	}
	if !validSorts[sort] {
		sort = "relevance"
	}

	return s.fileRepo.Search(ctx, userID, keyword, folderID, sort)
}