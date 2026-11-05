package service

import (
	"context"
	"testing"

	"cloudbox/internal/model"
	"cloudbox/internal/repository"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestFilterChunksByScore(t *testing.T) {
	chunks := []scoredChunk{
		{score: 0.10},
		{score: 0.80},
		{score: 0.25},
		{score: 0.24},
	}

	filtered := filterChunksByScore(chunks, 0.25)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 chunks above threshold, got %d", len(filtered))
	}
	if filtered[0].score != 0.80 || filtered[1].score != 0.25 {
		t.Fatalf("unexpected scores: %+v", filtered)
	}

	if got := filterChunksByScore(chunks, 0); len(got) != len(chunks) {
		t.Fatalf("minScore 0 should not filter, got %d", len(got))
	}

	if got := filterChunksByScore(chunks, 0.9); len(got) != 0 {
		t.Fatalf("expected no chunks, got %d", len(got))
	}
}

func TestRAGService_Stats(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.PhysicalFile{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	oldDB := repository.DB
	repository.DB = db
	defer func() { repository.DB = oldDB }()

	files := []model.PhysicalFile{
		{StoragePath: "a.txt", MD5: "md5-a", Size: 10, MimeType: "text/plain", ChunkCount: 3, Summary: "summary"},
		{StoragePath: "b.txt", MD5: "md5-b", Size: 10, MimeType: "text/plain", ChunkCount: 0},
		{StoragePath: "c.jpg", MD5: "md5-c", Size: 10, MimeType: "image/jpeg", ChunkCount: 0},
		{StoragePath: "d.pdf", MD5: "md5-d", Size: 10, MimeType: "application/pdf", ChunkCount: 2},
	}
	if err := db.Create(&files).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	stats, err := (&RAGService{}).Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.TotalPhysical != 4 {
		t.Errorf("TotalPhysical = %d, want 4", stats.TotalPhysical)
	}
	if stats.IndexedFiles != 2 {
		t.Errorf("IndexedFiles = %d, want 2", stats.IndexedFiles)
	}
	if stats.UnindexedSupported != 1 {
		t.Errorf("UnindexedSupported = %d, want 1", stats.UnindexedSupported)
	}
	if stats.TotalChunks != 5 {
		t.Errorf("TotalChunks = %d, want 5", stats.TotalChunks)
	}
	if stats.SummarizedFiles != 1 {
		t.Errorf("SummarizedFiles = %d, want 1", stats.SummarizedFiles)
	}
}

func TestIsIndexable(t *testing.T) {
	tests := []struct {
		name        string
		mime        string
		storagePath string
		want        bool
	}{
		{"plain text", "text/plain", "a.txt", true},
		{"markdown", "text/markdown", "a.md", true},
		{"pdf", "application/pdf", "a.pdf", true},
		{"docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "a.docx", true},
		{"html", "text/html", "a.html", true},
		{"image", "image/jpeg", "a.jpg", false},
		{"zip", "application/zip", "a.zip", false},
		{"legacy doc is not supported", "application/msword", "a.doc", false},
		{"octet-stream text extension", "application/octet-stream", "a.txt", true},
		{"octet-stream unknown extension", "application/octet-stream", "a.bin", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsIndexable(tt.mime, tt.storagePath); got != tt.want {
				t.Fatalf("IsIndexable(%q, %q) = %v, want %v", tt.mime, tt.storagePath, got, tt.want)
			}
		})
	}
}
