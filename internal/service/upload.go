// internal/service/upload.go
package service

import (
	"cloudbox/internal/config"
	"cloudbox/internal/model"
	"cloudbox/internal/repository"
	"cloudbox/internal/util/storage"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

var (
	ErrUploadNotFound = errors.New("upload not found")
	ErrChunkNotFound  = errors.New("chunk not found")
	ErrInvalidChunk   = errors.New("invalid chunk")
)

type UploadService struct {
	fileRepo     *repository.FileRepository
	physicalRepo *repository.PhysicalFileRepository
	storage      *storage.StorageManager
	chunkSize    int64
}

func NewUploadService(
	fileRepo *repository.FileRepository,
	physicalRepo *repository.PhysicalFileRepository,
	storage *storage.StorageManager,
) *UploadService {
	cfg := config.Get()
	return &UploadService{
		fileRepo:     fileRepo,
		physicalRepo: physicalRepo,
		storage:      storage,
		chunkSize:    cfg.Upload.ChunkSize,
	}
}

type InitUploadRequest struct {
	MD5            string `json:"md5"`
	FileName       string `json:"fileName"`
	FileSize       int64  `json:"fileSize"`
	TargetFolderID int64  `json:"targetFolderId"`
}

type InitUploadResponse struct {
	Instant           bool        `json:"instant"`
	File              *model.File `json:"file,omitempty"`
	UploadID          string      `json:"uploadID,omitempty"`
	ChunkSize         int64       `json:"chunkSize,omitempty"`
	ChunksAlreadyDone []int       `json:"chunksAlreadyDone,omitempty"`
}

func (s *UploadService) InitUpload(ctx context.Context, userID int64, req InitUploadRequest) (*InitUploadResponse, error) {
	// Check for instant upload
	pf, err := s.physicalRepo.FindByMD5(ctx, req.MD5)
	if err == nil {
		// Instant upload: physical file exists
		file, err := s.createFileRecord(ctx, userID, req.FileName, req.TargetFolderID, pf)
		if err != nil {
			return nil, err
		}

		// Increment ref count
		s.physicalRepo.IncrementRefCount(ctx, pf.ID)

		return &InitUploadResponse{
			Instant: true,
			File:    file,
		}, nil
	}

	// Generate uploadID
	uploadID := fmt.Sprintf("%s_%d", req.MD5, userID)
	tempDir := s.storage.TempChunkDir(uploadID)

	// Check existing chunks
	chunksAlreadyDone := []int{}
	if info, err := os.Stat(tempDir); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(tempDir)
		for _, entry := range entries {
			var idx int
			if _, err := fmt.Sscanf(entry.Name(), "%d.chunk", &idx); err == nil {
				chunksAlreadyDone = append(chunksAlreadyDone, idx)
			}
		}
	}

	return &InitUploadResponse{
		Instant:           false,
		UploadID:          uploadID,
		ChunkSize:         s.chunkSize,
		ChunksAlreadyDone: chunksAlreadyDone,
	}, nil
}

func (s *UploadService) SaveChunk(ctx context.Context, uploadID string, chunkIndex int, reader io.Reader) error {
	tempDir := s.storage.TempChunkDir(uploadID)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return err
	}

	chunkPath := filepath.Join(tempDir, fmt.Sprintf("%d.chunk", chunkIndex))
	file, err := os.Create(chunkPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, reader)
	return err
}

func (s *UploadService) GetProgress(ctx context.Context, uploadID string) ([]int, error) {
	tempDir := s.storage.TempChunkDir(uploadID)

	chunks := []int{}
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return chunks, nil
	}

	for _, entry := range entries {
		var idx int
		if _, err := fmt.Sscanf(entry.Name(), "%d.chunk", &idx); err == nil {
			chunks = append(chunks, idx)
		}
	}

	return chunks, nil
}

type CompleteUploadRequest struct {
	FileName       string `json:"fileName"`
	FileSize       int64  `json:"fileSize"`
	MD5            string `json:"md5"`
	TargetFolderID int64  `json:"targetFolderId"`
}

func (s *UploadService) CompleteUpload(ctx context.Context, userID int64, uploadID string, req CompleteUploadRequest) (*model.File, error) {
	tempDir := s.storage.TempChunkDir(uploadID)
	defer os.RemoveAll(tempDir)

	// Calculate total chunks
	totalChunks := int((req.FileSize + s.chunkSize - 1) / s.chunkSize)

	// Merge chunks and calculate MD5
	hash := md5.New()
	var mergedSize int64

	// Create temp file for merged content
	mergedPath := filepath.Join(tempDir, "merged.tmp")
	mergedFile, err := os.Create(mergedPath)
	if err != nil {
		return nil, err
	}

	for i := 0; i < totalChunks; i++ {
		chunkPath := filepath.Join(tempDir, fmt.Sprintf("%d.chunk", i))
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			mergedFile.Close()
			return nil, ErrChunkNotFound
		}

		n, err := io.Copy(io.MultiWriter(mergedFile, hash), chunkFile)
		chunkFile.Close()
		if err != nil {
			mergedFile.Close()
			return nil, err
		}
		mergedSize += n
	}
	mergedFile.Close()

	// Verify MD5
	calculatedMD5 := hex.EncodeToString(hash.Sum(nil))
	if calculatedMD5 != req.MD5 {
		return nil, errors.New("MD5 mismatch")
	}

	// Verify size
	if mergedSize != req.FileSize {
		return nil, errors.New("file size mismatch")
	}

	// Create physical file record
	pf := &model.PhysicalFile{
		MD5:      req.MD5,
		Size:     req.FileSize,
		MimeType: detectMimeType(req.FileName),
	}

	if err := s.physicalRepo.Create(ctx, pf); err != nil {
		return nil, err
	}

	// Generate storage path
	ext := filepath.Ext(req.FileName)
	relative, absolute := s.storage.GenerateFilePath(pf.ID, ext)
	pf.StoragePath = relative

	// Ensure parent directory
	s.storage.EnsureParentDir(absolute)

	// Move merged file to final location
	if err := os.Rename(mergedPath, absolute); err != nil {
		// Fallback: copy
		copyFile(mergedPath, absolute)
	}

	// Update physical file with storage path
	s.physicalRepo.Update(ctx, pf)

	// Create file record
	file, err := s.createFileRecord(ctx, userID, req.FileName, req.TargetFolderID, pf)
	if err != nil {
		return nil, err
	}

	// Generate thumbnail asynchronously
	go s.generateThumbnail(pf.ID, absolute)

	return file, nil
}

func (s *UploadService) createFileRecord(ctx context.Context, userID int64, fileName string, folderID int64, pf *model.PhysicalFile) (*model.File, error) {
	file := &model.File{
		Name:       fileName,
		PhysicalID: repository.NullInt64(pf.ID),
		ParentID:   repository.NullInt64(folderID),
		OwnerID:    userID,
		IsFolder:   false,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.fileRepo.Create(ctx, file); err != nil {
		return nil, err
	}

	return file, nil
}

func (s *UploadService) generateThumbnail(physicalID int64, filePath string) {
	// TODO: Implement thumbnail generation
	// This will be implemented in a separate task
}

func detectMimeType(filename string) string {
	ext := filepath.Ext(filename)
	mimeTypes := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".pdf":  "application/pdf",
		".doc":  "application/msword",
		".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".xls":  "application/vnd.ms-excel",
		".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".mp4":  "video/mp4",
		".mp3":  "audio/mpeg",
		".zip":  "application/zip",
	}
	if mt, ok := mimeTypes[ext]; ok {
		return mt
	}
	return "application/octet-stream"
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
