package model

import (
	"database/sql"
	"encoding/json"
	"time"
)

type File struct {
	ID         int64         `gorm:"primaryKey" json:"id"`
	Name       string        `gorm:"size:255;not null" json:"name"`
	ParentID   sql.NullInt64 `gorm:"column:parent_id;type:integer;index" json:"parentId"`
	OwnerID    int64         `gorm:"not null;index" json:"ownerId"`
	IsFolder   bool          `gorm:"default:false" json:"isFolder"`
	DeletedAt  sql.NullTime  `gorm:"column:deleted_at;type:datetime;index" json:"deletedAt"`
	CreatedAt time.Time     `json:"createdAt"`
	UpdatedAt time.Time     `json:"updatedAt"`

	// ContentRef - reference to physical file content, manually managed
	ContentRef int64          `gorm:"column:physical_ref" json:"physicalId"`
	// Physical - populated manually via repository, NOT via GORM auto-relation
	Physical       *PhysicalFile `gorm:"-" json:"physical,omitempty"`
}

func (File) TableName() string {
	return "files"
}

type FileResponse struct {
	ID         int64         `json:"id"`
	Name       string        `json:"name"`
	PhysicalID *int64        `json:"physicalId"`
	ParentID   *int64        `json:"parentId"`
	OwnerID    int64         `json:"ownerId"`
	IsFolder   bool          `json:"isFolder"`
	DeletedAt  *time.Time   `json:"deletedAt"`
	CreatedAt  time.Time     `json:"createdAt"`
	UpdatedAt  time.Time     `json:"updatedAt"`
	Physical   *PhysicalFile `json:"physical,omitempty"`
}

func (f *File) ToResponse() *FileResponse {
	resp := &FileResponse{
		ID:         f.ID,
		Name:       f.Name,
		OwnerID:    f.OwnerID,
		IsFolder:   f.IsFolder,
		CreatedAt:  f.CreatedAt,
		UpdatedAt:  f.UpdatedAt,
		Physical:   f.Physical,
	}
	if f.ContentRef != 0 {
		resp.PhysicalID = &f.ContentRef
	}
	if f.ParentID.Valid {
		resp.ParentID = &f.ParentID.Int64
	}
	if f.DeletedAt.Valid {
		resp.DeletedAt = &f.DeletedAt.Time
	}
	return resp
}

func (f File) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.ToResponse())
}