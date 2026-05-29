package model

import (
	"database/sql"
	"time"
)

type FileShare struct {
	ID            int64          `gorm:"primaryKey" json:"id"`
	Token         string         `gorm:"uniqueIndex;size:8;not null" json:"token"`
	FileID        int64          `gorm:"not null;index" json:"fileId"`
	OwnerID       int64          `gorm:"not null;index" json:"ownerId"`
	PasswordHash  sql.NullString `gorm:"size:255" json:"-"`
	ExpiresAt     sql.NullTime   `gorm:"column:expires_at;type:datetime" json:"expiresAt"`
	MaxDownloads  int            `gorm:"default:0" json:"maxDownloads"`
	DownloadCount int            `gorm:"default:0" json:"downloadCount"`
	Revoked       bool           `gorm:"default:false" json:"revoked"`
	CreatedAt     time.Time      `json:"createdAt"`
}

func (FileShare) TableName() string {
	return "file_shares"
}
