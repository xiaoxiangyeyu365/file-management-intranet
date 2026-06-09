// internal/repository/conversation.go
package repository

import (
	"cloudbox/internal/model"
	"context"

	"gorm.io/gorm"
)

type ConversationRepository struct {
	db *gorm.DB
}

func NewConversationRepository(db *gorm.DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

func (r *ConversationRepository) Create(ctx context.Context, conv *model.Conversation) error {
	return r.db.WithContext(ctx).Create(conv).Error
}

func (r *ConversationRepository) FindByIDAndUser(ctx context.Context, id, userID int64) (*model.Conversation, error) {
	var conv model.Conversation
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&conv).Error
	if err != nil {
		return nil, err
	}
	return &conv, nil
}

func (r *ConversationRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]model.Conversation, int64, error) {
	var total int64
	r.db.WithContext(ctx).Model(&model.Conversation{}).
		Where("user_id = ?", userID).Count(&total)

	var convs []model.Conversation
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("updated_at DESC").
		Limit(limit).Offset(offset).
		Find(&convs).Error
	return convs, total, err
}

func (r *ConversationRepository) Update(ctx context.Context, conv *model.Conversation) error {
	return r.db.WithContext(ctx).Save(conv).Error
}

func (r *ConversationRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Conversation{}, id).Error
}
