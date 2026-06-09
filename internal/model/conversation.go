// internal/model/conversation.go
package model

import "time"

type Conversation struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	UserID    int64     `gorm:"not null;index:idx_conversations_user" json:"userId"`
	Title     string    `gorm:"size:200" json:"title"`
	FileIDs   string    `gorm:"type:text" json:"fileIds"`
	CreatedAt time.Time `gorm:"not null" json:"createdAt"`
	UpdatedAt time.Time `gorm:"not null;index:idx_conversations_user" json:"updatedAt"`
}

func (Conversation) TableName() string { return "conversations" }
