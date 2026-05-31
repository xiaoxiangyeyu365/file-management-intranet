package model

import "time"

const (
	UserStatusPending  = "pending"
	UserStatusApproved = "approved"
	UserStatusDisabled = "disabled"
)

type User struct {
	ID              int64     `gorm:"primaryKey" json:"id"`
	Username        string    `gorm:"uniqueIndex;size:50;not null" json:"username"`
	PasswordHash    string    `gorm:"size:255;not null" json:"-"`
	Role            string    `gorm:"size:20;default:user" json:"role"`
	Status          string    `gorm:"size:20;default:approved" json:"status"`
	PasswordChanged bool      `gorm:"default:true" json:"passwordChanged"`
	CreatedAt       time.Time `json:"createdAt"`
	DiskQuota       *int64    `gorm:"column:disk_quota" json:"diskQuota"`
}

func (User) TableName() string {
	return "users"
}
