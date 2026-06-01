package storage

import (
	"cloudbox/internal/config"
	"fmt"
	"log"
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
	// Ensure forward slashes for database storage
	relative = filepath.ToSlash(relative)
	return
}

func (s *StorageManager) ToAbsPath(relative string) string {
	// Convert any backslashes to forward slashes first, then join
	// This handles cases where the DB stores Windows-style paths
	relative = filepath.ToSlash(relative)
	path := filepath.Join(s.rootDir, relative)
	// Convert back to OS-native separators for the final path
	return filepath.FromSlash(path)
}

func (s *StorageManager) ThumbnailPath(physicalID int64) string {
	return filepath.Join(s.thumbnailDir, fmt.Sprintf("%d.jpg", physicalID))
}

func (s *StorageManager) TempChunkDir(uploadID string) string {
	return filepath.Join(s.tempDir, "chunks", uploadID)
}

// cleanupInterval controls how often the background goroutine scans for stale temp directories.
const cleanupInterval = 1 * time.Hour

// StartTempCleanup launches a background goroutine that periodically removes
// stale upload chunk directories under {tempDir}/chunks/. A directory is
// considered stale if its mtime is older than tempExpire.
func StartTempCleanup(tempExpire time.Duration) {
	if storageManager == nil {
		panic("storage manager not initialized - call InitStorage first")
	}

	chunksDir := filepath.Join(storageManager.tempDir, "chunks")

	go func() {
		log.Printf("[temp-cleanup] started: interval=%s, expire=%s, dir=%s", cleanupInterval, tempExpire, chunksDir)
		ticker := time.NewTicker(cleanupInterval)
		for range ticker.C {
			cleanStaleChunks(chunksDir, tempExpire)
		}
	}()
}

func cleanStaleChunks(chunksDir string, tempExpire time.Duration) {
	entries, err := os.ReadDir(chunksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Printf("[temp-cleanup] error reading chunks dir: %v", err)
		return
	}

	now := time.Now()
	var removed int

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			log.Printf("[temp-cleanup] error statting %s: %v", entry.Name(), err)
			continue
		}

		if now.Sub(info.ModTime()) <= tempExpire {
			continue
		}

		dirPath := filepath.Join(chunksDir, entry.Name())
		if err := os.RemoveAll(dirPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			log.Printf("[temp-cleanup] error removing %s: %v", entry.Name(), err)
			continue
		}
		removed++
	}

	if removed > 0 {
		log.Printf("[temp-cleanup] removed %d stale upload directory(s)", removed)
	}
}

func (s *StorageManager) EnsureParentDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}
