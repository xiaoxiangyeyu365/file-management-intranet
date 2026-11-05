package service

import (
	"context"
	"strings"
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

func TestChunkByParagraphs_ChineseRunes(t *testing.T) {
	paragraphs := make([]string, 10)
	for i := range paragraphs {
		paragraphs[i] = strings.Repeat("中", 90)
	}

	chunks := chunkByParagraphs(paragraphs, 100, 500, 0)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks for 900 runes, got %d", len(chunks))
	}
	for i, c := range chunks {
		if n := len([]rune(c)); n > 500 {
			t.Fatalf("chunk %d has %d runes, exceeds max 500", i, n)
		}
	}
}

func TestChunkByParagraphs_SplitsOversizedParagraph(t *testing.T) {
	chunks := chunkByParagraphs([]string{strings.Repeat("文", 1200)}, 100, 500, 0)
	if len(chunks) < 3 {
		t.Fatalf("expected oversized paragraph to be split, got %d chunks", len(chunks))
	}
	for i, c := range chunks {
		if n := len([]rune(c)); n > 500 {
			t.Fatalf("chunk %d has %d runes, exceeds max 500", i, n)
		}
	}
}

func TestBuildSearchQuery(t *testing.T) {
	recent := []model.Message{
		{Role: "user", Content: "第一份文档讲了什么"},
		{Role: "assistant", Content: "讲了 A 和 B。"},
	}

	followUp := buildSearchQuery("它的结论呢", recent)
	if !strings.Contains(followUp, "第一份文档讲了什么") || !strings.Contains(followUp, "它的结论呢") {
		t.Fatalf("follow-up query should include previous question, got %q", followUp)
	}

	longQuestion := strings.Repeat("这是一个全新主题的长问题", 10)
	if got := buildSearchQuery(longQuestion, recent); got != longQuestion {
		t.Fatalf("unrelated long question should not include previous context, got %q", got)
	}
}

func TestTrimCurrentQuestion(t *testing.T) {
	msgs := []model.Message{
		{Role: "user", Content: "previous"},
		{Role: "assistant", Content: "answer"},
		{Role: "user", Content: "current"},
	}

	trimmed := trimCurrentQuestion(msgs, "current")
	if len(trimmed) != 2 {
		t.Fatalf("expected current question trimmed, got %d messages", len(trimmed))
	}
	if got := trimCurrentQuestion(msgs, "different"); len(got) != len(msgs) {
		t.Fatalf("unrelated question should not trim, got %d messages", len(got))
	}
}

func TestResolvePhysicalIDs_AllScope(t *testing.T) {
	fr := &mockFileRepo{
		findAllByOwnerFn: func(ctx context.Context, ownerID int64) ([]model.File, error) {
			return []model.File{
				{ID: 1, Name: "a.txt", ContentRef: 10, Physical: &model.PhysicalFile{ChunkCount: 2}},
				{ID: 2, Name: "b.txt", ContentRef: 11, Physical: &model.PhysicalFile{ChunkCount: 0}},
				{ID: 3, Name: "dir", IsFolder: true},
			}, nil
		},
	}

	svc := &RAGService{fileRepo: fr}
	ids, refs, err := svc.resolvePhysicalIDs(context.Background(), &model.Conversation{Scope: "all"}, 1)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(ids) != 1 || ids[0] != 10 {
		t.Fatalf("expected only indexed physical 10, got %v", ids)
	}
	if ref := refs[10]; ref.ID != 1 || ref.Name != "a.txt" {
		t.Fatalf("unexpected file ref: %+v", ref)
	}
}

func TestResolvePhysicalIDs_SelectedFiles(t *testing.T) {
	fr := &mockFileRepo{
		findByIDAndOwnerFn: func(ctx context.Context, id, ownerID int64) (*model.File, error) {
			if id == 7 {
				return &model.File{ID: 7, Name: "selected.txt", ContentRef: 20, Physical: &model.PhysicalFile{ChunkCount: 1}}, nil
			}
			return nil, gorm.ErrRecordNotFound
		},
	}

	svc := &RAGService{fileRepo: fr}
	ids, refs, err := svc.resolvePhysicalIDs(context.Background(), &model.Conversation{Scope: "files", FileIDs: "[7,8]"}, 1)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(ids) != 1 || ids[0] != 20 {
		t.Fatalf("expected physical 20, got %v", ids)
	}
	if ref := refs[20]; ref.ID != 7 {
		t.Fatalf("unexpected file ref: %+v", ref)
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
