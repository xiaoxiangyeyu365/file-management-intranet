package model

import (
	"database/sql"
	"time"
)

type File struct {
	ID         int64          `gorm:"primaryKey" json:"id"`
	Name       string         `gorm:"size:255;not null" json:"name"`
	PhysicalID sql.NullInt64  `gorm:"index" json:"physicalId"`
	ParentID   sql.NullInt64  `gorm:"index" json:"parentId"`
	OwnerID    int64          `gorm:"not null;index" json:"ownerId"`
	IsFolder   bool           `gorm:"default:false" json:"isFolder"`
	DeletedAt  sql.NullTime   `json:"deletedAt"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`

	// Relations
	Physical   *PhysicalFile `gorm:"foreignKey:PhysicalID" json:"physical,omitempty"`
}

func (File) TableName() string {
	return "files"
}
