package repository

import (
	"cloudbox/internal/model"
	"context"

	"gorm.io/gorm"
)

type ShareRepository struct {
	db *gorm.DB
}

func NewShareRepository(db *gorm.DB) *ShareRepository {
	return &ShareRepository{db: db}
}

func (r *ShareRepository) Create(ctx context.Context, share *model.FileShare) error {
	return r.db.WithContext(ctx).Create(share).Error
}

func (r *ShareRepository) FindByToken(ctx context.Context, token string) (*model.FileShare, error) {
	var share model.FileShare
	if err := r.db.WithContext(ctx).Where("token = ?", token).First(&share).Error; err != nil {
		return nil, err
	}
	return &share, nil
}

func (r *ShareRepository) FindByOwner(ctx context.Context, ownerID int64) ([]model.FileShare, error) {
	var shares []model.FileShare
	if err := r.db.WithContext(ctx).Where("owner_id = ?", ownerID).Order("created_at DESC").Find(&shares).Error; err != nil {
		return nil, err
	}
	return shares, nil
}

func (r *ShareRepository) FindByFile(ctx context.Context, fileID int64) ([]model.FileShare, error) {
	var shares []model.FileShare
	if err := r.db.WithContext(ctx).Where("file_id = ?", fileID).Order("created_at DESC").Find(&shares).Error; err != nil {
		return nil, err
	}
	return shares, nil
}

func (r *ShareRepository) Revoke(ctx context.Context, id, ownerID int64) error {
	return r.db.WithContext(ctx).Model(&model.FileShare{}).
		Where("id = ? AND owner_id = ?", id, ownerID).
		Update("revoked", true).Error
}

func (r *ShareRepository) IncrementDownloadCount(ctx context.Context, token string) (bool, error) {
	result := r.db.WithContext(ctx).Exec(`
		UPDATE file_shares
		SET download_count = download_count + 1
		WHERE token = ? AND (max_downloads = 0 OR download_count < max_downloads) AND revoked = false
	`, token)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}
