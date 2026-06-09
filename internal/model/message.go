// internal/model/message.go
package model

import "time"

type Message struct {
	ID             int64     `gorm:"primaryKey" json:"id"`
	ConversationID int64     `gorm:"not null;index:idx_messages_conversation" json:"conversationId"`
	Role           string    `gorm:"size:20;not null" json:"role"`
	Content        string    `gorm:"type:text;not null" json:"content"`
	Sources        string    `gorm:"type:text" json:"sources"`
	CreatedAt      time.Time `gorm:"not null;index:idx_messages_conversation" json:"createdAt"`
}

func (Message) TableName() string { return "messages" }
