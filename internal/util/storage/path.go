package storage

import (
	"cloudbox/internal/config"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type StorageManager struct {
	rootDir      string
	tempDir      string
	thumbnailDir string
}

var (
	storageManager *StorageManager
	storageOnce    sync.Once
)

func InitStorage(cfg *config.Config) *StorageManager {
	storageOnce.Do(func() {
		storageManager = &StorageManager{
			rootDir:      cfg.Storage.Root,
			tempDir:      cfg.Storage.Temp,
			thumbnailDir: cfg.Storage.Thumbnails,
		}
	})
	return storageManager
}

func GetStorage() *StorageManager {
	if storageManager == nil {
		panic("storage manager not initialized - call InitStorage first")
	}
	return storageManager
}

func (s *StorageManager) GenerateFilePath(physicalID int64, ext string) (relative, absolute string) {
	dateDir := time.Now().Format("2006/01/02")
	filename := fmt.Sprintf("%d%s", physicalID, ext)
	relative = filepath.Join(dateDir, filename)
	absolute = filepath.Join(s.rootDir, relative)
	return
}

func (s *StorageManager) ToAbsPath(relative string) string {
	return filepath.Join(s.rootDir, relative)
}

func (s *StorageManager) ThumbnailPath(physicalID int64) string {
	return filepath.Join(s.thumbnailDir, fmt.Sprintf("%d.jpg", physicalID))
}

func (s *StorageManager) TempChunkDir(uploadID string) string {
	return filepath.Join(s.tempDir, "chunks", uploadID)
}

func (s *StorageManager) EnsureParentDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}
