package model

import "time"

type ClipboardRecord struct {
    ID         int64     `gorm:"primaryKey" json:"id"`
    Content    string    `gorm:"type:text;not null" json:"content"`
    DeviceName string    `gorm:"size:100;default:'未命名设备'" json:"deviceName"`
    UserID     int64     `gorm:"not null;index" json:"userId"`
    Pinned     bool      `gorm:"default:false" json:"pinned"`
    CreatedAt  time.Time `json:"createdAt"`
}

func (ClipboardRecord) TableName() string {
    return "clipboard_records"
}

type ClipboardResponse struct {
    ID         int64     `json:"id"`
    Content    string    `json:"content"`
    DeviceName string    `json:"deviceName"`
    Pinned     bool      `json:"pinned"`
    CreatedAt  time.Time `json:"createdAt"`
}

func (c *ClipboardRecord) ToResponse() *ClipboardResponse {
    return &ClipboardResponse{
        ID:         c.ID,
        Content:    c.Content,
        DeviceName: c.DeviceName,
        Pinned:     c.Pinned,
        CreatedAt:  c.CreatedAt,
    }
}