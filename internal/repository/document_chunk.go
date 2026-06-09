// internal/repository/document_chunk.go
package repository

import (
	"cloudbox/internal/model"
	"context"

	"gorm.io/gorm"
)

type ChunkRepository struct {
	db *gorm.DB
}

func NewChunkRepository(db *gorm.DB) *ChunkRepository {
	return &ChunkRepository{db: db}
}

func (r *ChunkRepository) CountByPhysicalFileID(ctx context.Context, physicalFileID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.DocumentChunk{}).
		Where("physical_file_id = ?", physicalFileID).
		Count(&count).Error
	return count, err
}

func (r *ChunkRepository) CreateBatch(ctx context.Context, chunks []model.DocumentChunk) error {
	return r.db.WithContext(ctx).Create(&chunks).Error
}

func (r *ChunkRepository) FindByPhysicalFileIDs(ctx context.Context, physicalFileIDs []int64) ([]model.DocumentChunk, error) {
	var chunks []model.DocumentChunk
	err := r.db.WithContext(ctx).
		Where("physical_file_id IN ?", physicalFileIDs).
		Order("physical_file_id, chunk_index").
		Find(&chunks).Error
	return chunks, err
}

func (r *ChunkRepository) DeleteByPhysicalFileID(ctx context.Context, physicalFileID int64) error {
	return r.db.WithContext(ctx).
		Where("physical_file_id = ?", physicalFileID).
		Delete(&model.DocumentChunk{}).Error
}

func (r *ChunkRepository) UpdateChunkCount(ctx context.Context, physicalFileID int64, count int) error {
	return r.db.WithContext(ctx).
		Model(&model.PhysicalFile{}).
		Where("id = ?", physicalFileID).
		Update("chunk_count", count).Error
}
