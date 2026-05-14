package model

import "time"

type PhysicalFile struct {
	ID            int64     `gorm:"primaryKey" json:"id"`
	StoragePath   string    `gorm:"uniqueIndex;size:500;not null" json:"storagePath"`
	MD5           string    `gorm:"uniqueIndex;size:32;not null" json:"md5"`
	Size          int64     `gorm:"not null" json:"size"`
	MimeType      string    `gorm:"size:100" json:"mimeType"`
	RefCount      int       `gorm:"default:1" json:"refCount"`
	ThumbnailPath string    `gorm:"size:500" json:"thumbnailPath"`
	Width         int       `gorm:"default:0" json:"width"`
	Height        int       `gorm:"default:0" json:"height"`
	MetadataJSON  string    `gorm:"type:text" json:"metadataJson"`
	CreatedAt     time.Time `json:"createdAt"`
}

func (PhysicalFile) TableName() string {
	return "physical_files"
}
