// internal/model/document_chunk.go
package model

import "time"

type DocumentChunk struct {
	ID             int64     `gorm:"primaryKey" json:"id"`
	PhysicalFileID int64     `gorm:"not null;index:idx_chunks_physical" json:"physicalFileId"`
	ChunkIndex     int       `gorm:"not null;index:idx_chunks_physical" json:"chunkIndex"`
	Content        string    `gorm:"type:text;not null" json:"content"`
	Embedding      []byte    `gorm:"type:blob" json:"-"`
	TokenCount     int       `json:"tokenCount"`
	CreatedAt      time.Time `gorm:"not null" json:"createdAt"`
}

func (DocumentChunk) TableName() string { return "document_chunks" }
