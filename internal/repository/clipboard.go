package repository

import (
    "cloudbox/internal/model"
    "context"

    "gorm.io/gorm"
)

type ClipboardRepository struct {
    db *gorm.DB
}

func NewClipboardRepository(db *gorm.DB) *ClipboardRepository {
    return &ClipboardRepository{db: db}
}

func (r *ClipboardRepository) Create(ctx context.Context, record *model.ClipboardRecord) error {
    return r.db.WithContext(ctx).Create(record).Error
}

func (r *ClipboardRepository) FindByUser(ctx context.Context, userID int64, limit int) ([]model.ClipboardRecord, error) {
    var records []model.ClipboardRecord
    err := r.db.WithContext(ctx).
        Where("user_id = ?", userID).
        Order("pinned DESC, created_at DESC").
        Limit(limit).
        Find(&records).Error
    return records, err
}

func (r *ClipboardRepository) FindByIDAndUser(ctx context.Context, id, userID int64) (*model.ClipboardRecord, error) {
    var record model.ClipboardRecord
    err := r.db.WithContext(ctx).
        Where("id = ? AND user_id = ?", id, userID).
        First(&record).Error
    if err != nil {
        return nil, err
    }
    return &record, nil
}

func (r *ClipboardRepository) DeleteByID(ctx context.Context, id int64) error {
    return r.db.WithContext(ctx).Delete(&model.ClipboardRecord{}, id).Error
}

func (r *ClipboardRepository) UpdatePinned(ctx context.Context, id int64, pinned bool) error {
    return r.db.WithContext(ctx).
        Model(&model.ClipboardRecord{}).
        Where("id = ?", id).
        Update("pinned", pinned).Error
}

func (r *ClipboardRepository) DeleteByUserUnpinned(ctx context.Context, userID int64) error {
    return r.db.WithContext(ctx).
        Where("user_id = ? AND pinned = ?", userID, false).
        Delete(&model.ClipboardRecord{}).Error
}

func (r *ClipboardRepository) DeleteByUser(ctx context.Context, userID int64) error {
    return r.db.WithContext(ctx).
        Where("user_id = ?", userID).
        Delete(&model.ClipboardRecord{}).Error
}

func (r *ClipboardRepository) CountUnpinned(ctx context.Context, userID int64) (int64, error) {
    var count int64
    err := r.db.WithContext(ctx).
        Model(&model.ClipboardRecord{}).
        Where("user_id = ? AND pinned = ?", userID, false).
        Count(&count).Error
    return count, err
}

func (r *ClipboardRepository) DeleteOldestUnpinned(ctx context.Context, userID int64, keepCount int) error {
    sql := `DELETE FROM clipboard_records
            WHERE id IN (
                SELECT id FROM clipboard_records
                WHERE user_id = ? AND pinned = 0
                ORDER BY created_at ASC
                LIMIT ?
            )`
    return r.db.WithContext(ctx).Exec(sql, userID, keepCount).Error
}