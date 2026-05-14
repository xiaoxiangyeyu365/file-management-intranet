// internal/service/preview.go
package service

import (
	"cloudbox/internal/model"
	"cloudbox/internal/repository"
	"cloudbox/internal/util/storage"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"time"

	"github.com/dsoprea/go-exif/v3"
	exifcommon "github.com/dsoprea/go-exif/v3/common"
)

var (
	ErrMetadataFailed = errors.New("failed to extract metadata")
)

type ImageMetadata struct {
	Width  int            `json:"width"`
	Height int            `json:"height"`
	EXIF   map[string]any `json:"exif,omitempty"`
}

type PreviewService struct {
	physicalRepo *repository.PhysicalFileRepository
	fileRepo     *repository.FileRepository
	storage      *storage.StorageManager
}

func NewPreviewService(
	physicalRepo *repository.PhysicalFileRepository,
	fileRepo *repository.FileRepository,
	storage *storage.StorageManager,
) *PreviewService {
	return &PreviewService{
		physicalRepo: physicalRepo,
		fileRepo:     fileRepo,
		storage:      storage,
	}
}

// ProcessImage processes an image after upload: extract dimensions, generate thumbnail, extract EXIF
func (s *PreviewService) ProcessImage(ctx context.Context, physicalID int64) error {
	pf, err := s.physicalRepo.FindByID(ctx, physicalID)
	if err != nil {
		return fmt.Errorf("failed to find physical file: %w", err)
	}

	// Check if image type
	if !isImageType(pf.MimeType) {
		return nil // Not an image, skip processing
	}

	absPath := s.storage.ToAbsPath(pf.StoragePath)

	// 1. Get image dimensions
	width, height, err := s.getImageDimensions(absPath)
	if err != nil {
		log.Printf("warning: failed to get image dimensions for %d: %v", physicalID, err)
	}

	// 2. Extract EXIF metadata
	metadataJSON := ""
	if exifData, err := s.extractEXIF(absPath); err == nil {
		if jsonData, err := json.Marshal(exifData); err == nil {
			metadataJSON = string(jsonData)
		}
	}

	// 3. Generate thumbnail if needed
	thumbnailPath := s.storage.ThumbnailPath(pf.ID)
	if _, err := os.Stat(thumbnailPath); os.IsNotExist(err) {
		if err := s.generateThumbnail(pf, absPath, thumbnailPath); err != nil {
			log.Printf("warning: failed to generate thumbnail for %d: %v", physicalID, err)
		}
	}

	// 4. Update database with all info
	if width > 0 && height > 0 {
		if err := s.physicalRepo.UpdateImageInfo(ctx, physicalID, width, height, metadataJSON); err != nil {
			log.Printf("warning: failed to update image info for %d: %v", physicalID, err)
		}
	}

	return nil
}

// GetMetadata returns image metadata, extracting on-demand if needed
func (s *PreviewService) GetMetadata(ctx context.Context, userID, fileID int64) (*ImageMetadata, error) {
	// Get file and verify ownership
	file, err := s.fileRepo.FindByIDAndOwner(ctx, fileID, userID)
	if err != nil {
		return nil, ErrFileNotFound
	}

	if file.IsFolder || !file.PhysicalID.Valid {
		return nil, ErrNotImage
	}

	pf, err := s.physicalRepo.FindByID(ctx, file.PhysicalID.Int64)
	if err != nil {
		return nil, fmt.Errorf("failed to find physical file: %w", err)
	}

	if !isImageType(pf.MimeType) {
		return nil, ErrNotImage
	}

	metadata := &ImageMetadata{
		Width:  pf.Width,
		Height: pf.Height,
	}

	// If metadata exists, parse it
	if pf.MetadataJSON != "" {
		if err := json.Unmarshal([]byte(pf.MetadataJSON), &metadata.EXIF); err != nil {
			log.Printf("warning: failed to parse metadata JSON: %v", err)
		}
		return metadata, nil
	}

	// Extract on-demand
	absPath := s.storage.ToAbsPath(pf.StoragePath)

	// Get dimensions if not set
	if pf.Width == 0 || pf.Height == 0 {
		width, height, err := s.getImageDimensions(absPath)
		if err != nil {
			return nil, err
		}
		metadata.Width = width
		metadata.Height = height
	}

	// Extract EXIF
	exifData, err := s.extractEXIF(absPath)
	if err == nil {
		metadata.EXIF = exifData
	}

	// Try to save with optimistic lock
	if jsonData, err := json.Marshal(metadata.EXIF); err == nil {
		updated, err := s.physicalRepo.UpdateMetadataWithOptimisticLock(ctx, pf.ID, string(jsonData))
		if err != nil {
			log.Printf("warning: failed to update metadata: %v", err)
		}
		if !updated {
			// Another process updated it, fetch the latest
			pf, _ = s.physicalRepo.FindByID(ctx, pf.ID)
			if pf.MetadataJSON != "" {
				json.Unmarshal([]byte(pf.MetadataJSON), &metadata.EXIF)
			}
		}
	}

	return metadata, nil
}

func (s *PreviewService) getImageDimensions(path string) (int, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}
	return config.Width, config.Height, nil
}

func (s *PreviewService) extractEXIF(path string) (map[string]any, error) {
	data, err := exif.SearchFileAndExtractExif(path)
	if err != nil {
		return nil, err
	}

	im, err := exifcommon.NewIfdMappingWithStandard()
	if err != nil {
		return nil, err
	}

	ti := exif.NewTagIndex()
	_, index, err := exif.Collect(im, ti, data)
	if err != nil {
		return nil, err
	}

	result := make(map[string]any)

	// Walk through all IFDs and their tags
	for _, ifd := range index.Ifds {
		for _, entry := range ifd.Entries() {
			tagName := entry.TagName()
			if tagName == "" {
				continue
			}

			value, err := entry.Value()
			if err != nil {
				continue
			}

			// Format specific tags
			switch v := value.(type) {
			case []byte:
				if len(v) < 100 {
					result[tagName] = fmt.Sprintf("%x", v)
				}
			case []interface{}:
				if len(v) <= 4 {
					result[tagName] = v
				}
			default:
				result[tagName] = v
			}
		}
	}

	return result, nil
}

func (s *PreviewService) generateThumbnail(pf *model.PhysicalFile, srcPath, thumbPath string) error {
	const thumbnailSize = 200
	const maxLockAge = 30 * time.Second

	// Ensure directory exists
	if err := s.storage.EnsureParentDir(thumbPath); err != nil {
		return err
	}

	// Lock file for concurrency safety
	lockPath := thumbPath + ".lock"
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

		if _, err := os.Stat(thumbPath); err == nil {
			return nil // Already generated
		}

		if i < 9 {
			time.Sleep(100 * time.Millisecond)
		} else {
			return fmt.Errorf("failed to acquire lock")
		}
	}

	// Open source image
	file, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer file.Close()

	var img image.Image
	switch pf.MimeType {
	case "image/png":
		img, err = png.Decode(file)
	case "image/gif":
		img, err = gif.Decode(file)
	default:
		img, err = jpeg.Decode(file)
	}
	if err != nil {
		return err
	}

	// Resize
	resized := resizeImage(img, thumbnailSize)

	// Save thumbnail
	outFile, err := os.Create(thumbPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	if err := jpeg.Encode(outFile, resized, &jpeg.Options{Quality: 85}); err != nil {
		return err
	}

	// Update database
	return s.physicalRepo.UpdateThumbnail(context.Background(), pf.ID, thumbPath)
}
