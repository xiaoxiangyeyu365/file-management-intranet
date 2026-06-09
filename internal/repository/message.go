// internal/repository/message.go
package repository

import (
	"cloudbox/internal/model"
	"context"

	"gorm.io/gorm"
)

type MessageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) Create(ctx context.Context, msg *model.Message) error {
	return r.db.WithContext(ctx).Create(msg).Error
}

func (r *MessageRepository) FindByConversation(ctx context.Context, conversationID int64) ([]model.Message, error) {
	var msgs []model.Message
	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("created_at ASC").
		Find(&msgs).Error
	return msgs, err
}

func (r *MessageRepository) FindRecentByConversation(ctx context.Context, conversationID int64, limit int) ([]model.Message, error) {
	var msgs []model.Message
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND role IN ('user', 'assistant')", conversationID).
		Order("created_at DESC").
		Limit(limit).
		Find(&msgs).Error
	// Reverse to chronological order
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, err
}

func (r *MessageRepository) DeleteByConversation(ctx context.Context, conversationID int64) error {
	return r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Delete(&model.Message{}).Error
}

func (r *MessageRepository) CountByConversation(ctx context.Context, conversationID int64) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Message{}).
		Where("conversation_id = ? AND role = 'user'", conversationID).
		Count(&count).Error
	return count, err
}
