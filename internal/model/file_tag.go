package model

import "time"

type FileTag struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	FileID    int64     `gorm:"not null;index" json:"fileId"`
	Tag       string    `gorm:"size:50;not null;uniqueIndex:idx_file_tag" json:"tag"`
	CreatedAt time.Time `gorm:"not null" json:"createdAt"`
}

func (FileTag) TableName() string {
	return "file_tags"
}
