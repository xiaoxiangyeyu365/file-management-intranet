package repository

import (
	"cloudbox/internal/model"
	"context"
	"time"

	"gorm.io/gorm"
)

type AuditRepository struct {
	db *gorm.DB
}

func NewAuditRepository(db *gorm.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) BatchCreate(ctx context.Context, logs []model.AuditLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&logs).Error
}

func (r *AuditRepository) FindWithFilter(ctx context.Context, action string, userID int64, targetType string, keyword string, startDate, endDate *time.Time, page, pageSize int) ([]model.AuditLog, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	query := r.db.WithContext(ctx).Model(&model.AuditLog{})
	if action != "" {
		query = query.Where("action = ?", action)
	}
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	}
	if targetType != "" {
		query = query.Where("target_type = ?", targetType)
	}
	if keyword != "" {
		query = query.Where("target_name LIKE ?", "%"+keyword+"%")
	}
	if startDate != nil {
		query = query.Where("created_at >= ?", startDate)
	}
	if endDate != nil {
		query = query.Where("created_at <= ?", endDate)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []model.AuditLog
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&logs).Error; err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

func (r *AuditRepository) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Where("created_at < ?", before).Delete(&model.AuditLog{})
	return result.RowsAffected, result.Error
}
