package model

import "time"

type AuditLog struct {
	ID         int64     `gorm:"primaryKey" json:"id"`
	UserID     int64     `gorm:"not null;index:idx_audit_user" json:"userId"`
	Username   string    `gorm:"size:50;not null" json:"username"`
	Action     string    `gorm:"size:50;not null;index:idx_audit_action" json:"action"`
	TargetType string    `gorm:"size:30;not null" json:"targetType"`
	TargetID   *int64    `json:"targetId"`
	TargetName string    `gorm:"size:255" json:"targetName"`
	Detail     string    `json:"detail"`
	IPAddress  string    `gorm:"size:45" json:"ipAddress"`
	CreatedAt  time.Time `gorm:"not null;index:idx_audit_time" json:"createdAt"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}
