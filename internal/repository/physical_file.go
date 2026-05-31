// internal/repository/physical_file.go
package repository

import (
	"cloudbox/internal/model"
	"context"

	"gorm.io/gorm"
)

type PhysicalFileRepository struct {
	db *gorm.DB
}

func NewPhysicalFileRepository(db *gorm.DB) *PhysicalFileRepository {
	return &PhysicalFileRepository{db: db}
}

func (r *PhysicalFileRepository) Create(ctx context.Context, pf *model.PhysicalFile) error {
	return r.db.WithContext(ctx).Create(pf).Error
}

func (r *PhysicalFileRepository) FindByID(ctx context.Context, id int64) (*model.PhysicalFile, error) {
	var pf model.PhysicalFile
	err := r.db.WithContext(ctx).First(&pf, id).Error
	if err != nil {
		return nil, err
	}
	return &pf, nil
}

func (r *PhysicalFileRepository) FindByMD5(ctx context.Context, md5 string) (*model.PhysicalFile, error) {
	var pf model.PhysicalFile
	err := r.db.WithContext(ctx).Where("md5 = ?", md5).First(&pf).Error
	if err != nil {
		return nil, err
	}
	return &pf, nil
}

func (r *PhysicalFileRepository) IncrementRefCount(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).
		Model(&model.PhysicalFile{}).
		Where("id = ?", id).
		Update("ref_count", gorm.Expr("ref_count + 1")).Error
}

func (r *PhysicalFileRepository) DecrementRefCount(ctx context.Context, id int64, count int) (int, error) {
	var newRefCount int

	// SQLite 3.35+ supports RETURNING
	err := r.db.WithContext(ctx).
		Model(&model.PhysicalFile{}).
		Where("id = ?", id).
		Update("ref_count", gorm.Expr("ref_count - ?", count)).
		Scan(&newRefCount).Error

	if err != nil {
		// Fallback: query after update
		var pf model.PhysicalFile
		if err := r.db.WithContext(ctx).First(&pf, id).Error; err != nil {
			return 0, err
		}
		newRefCount = pf.RefCount
	}

	return newRefCount, nil
}

func (r *PhysicalFileRepository) UpdateThumbnail(ctx context.Context, id int64, thumbnailPath string) error {
	return r.db.WithContext(ctx).
		Model(&model.PhysicalFile{}).
		Where("id = ?", id).
		Update("thumbnail_path", thumbnailPath).Error
}

func (r *PhysicalFileRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.PhysicalFile{}, id).Error
}

func (r *PhysicalFileRepository) Update(ctx context.Context, pf *model.PhysicalFile) error {
	return r.db.WithContext(ctx).Save(pf).Error
}

// UpdateImageInfo updates width, height, and optionally metadata in one call
func (r *PhysicalFileRepository) UpdateImageInfo(ctx context.Context, id int64, width, height int, metadataJSON string) error {
	updates := map[string]interface{}{
		"width":  width,
		"height": height,
	}
	if metadataJSON != "" {
		updates["metadata_json"] = metadataJSON
	}
	return r.db.WithContext(ctx).
		Model(&model.PhysicalFile{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// UpdateMetadataWithOptimisticLock updates metadata only if it's currently NULL
// Returns true if updated, false if already set by another process
func (r *PhysicalFileRepository) UpdateMetadataWithOptimisticLock(ctx context.Context, id int64, metadataJSON string) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&model.PhysicalFile{}).
		Where("id = ? AND (metadata_json IS NULL OR metadata_json = '')", id).
		Update("metadata_json", metadataJSON)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *PhysicalFileRepository) CalculateUserStorageUsage(ctx context.Context, userID int64) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Table("files").
		Select("COALESCE(SUM(physical_files.size), 0)").
		Joins("JOIN physical_files ON files.physical_id = physical_files.id").
		Where("files.owner_id = ? AND files.is_folder = 0", userID).
		Scan(&total).Error
	return total, err
}

func (r *PhysicalFileRepository) CalculateAllUserStorageUsage(ctx context.Context) (map[int64]int64, error) {
	type usageRow struct {
		OwnerID   int64
		UsedBytes int64
	}
	var rows []usageRow
	err := r.db.WithContext(ctx).Table("files").
		Select("files.owner_id, COALESCE(SUM(physical_files.size), 0) AS used_bytes").
		Joins("JOIN physical_files ON files.physical_id = physical_files.id").
		Where("files.is_folder = 0").
		Group("files.owner_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[int64]int64, len(rows))
	for _, row := range rows {
		result[row.OwnerID] = row.UsedBytes
	}
	return result, nil
}
