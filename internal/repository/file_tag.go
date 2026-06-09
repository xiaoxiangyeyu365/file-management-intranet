package repository

import (
	"cloudbox/internal/model"
	"context"

	"gorm.io/gorm"
)

type FileTagRepository struct {
	db *gorm.DB
}

func NewFileTagRepository(db *gorm.DB) *FileTagRepository {
	return &FileTagRepository{db: db}
}

func (r *FileTagRepository) Create(ctx context.Context, ft *model.FileTag) error {
	return r.db.WithContext(ctx).Create(ft).Error
}

func (r *FileTagRepository) CreateBatch(ctx context.Context, tags []model.FileTag) error {
	return r.db.WithContext(ctx).Create(&tags).Error
}

func (r *FileTagRepository) FindByFileID(ctx context.Context, fileID int64) ([]model.FileTag, error) {
	var tags []model.FileTag
	err := r.db.WithContext(ctx).Where("file_id = ?", fileID).Find(&tags).Error
	if err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *FileTagRepository) DeleteByFileID(ctx context.Context, fileID int64) error {
	return r.db.WithContext(ctx).Where("file_id = ?", fileID).Delete(&model.FileTag{}).Error
}

func (r *FileTagRepository) FindByTag(ctx context.Context, tag string) ([]model.FileTag, error) {
	var tags []model.FileTag
	err := r.db.WithContext(ctx).Where("tag = ?", tag).Find(&tags).Error
	if err != nil {
		return nil, err
	}
	return tags, nil
}
