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
