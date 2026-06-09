package repository

import (
	"context"
	"testing"

	"cloudbox/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupFileTagTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.FileTag{}); err != nil {
		t.Fatalf("failed to auto-migrate: %v", err)
	}
	return db
}

func TestFileTagRepository_CreateAndFindByFileID(t *testing.T) {
	db := setupFileTagTestDB(t)
	repo := NewFileTagRepository(db)
	ctx := context.Background()

	ft := &model.FileTag{
		FileID:    1,
		Tag:       "important",
		CreatedAt: model.FileTag{}.CreatedAt,
	}
	if err := repo.Create(ctx, ft); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if ft.ID == 0 {
		t.Fatal("expected ID to be set after Create")
	}

	tags, err := repo.FindByFileID(ctx, 1)
	if err != nil {
		t.Fatalf("FindByFileID failed: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(tags))
	}
	if tags[0].Tag != "important" {
		t.Fatalf("expected tag 'important', got '%s'", tags[0].Tag)
	}
}

func TestFileTagRepository_DeleteByFileID(t *testing.T) {
	db := setupFileTagTestDB(t)
	repo := NewFileTagRepository(db)
	ctx := context.Background()

	ft1 := &model.FileTag{FileID: 10, Tag: "alpha"}
	ft2 := &model.FileTag{FileID: 10, Tag: "beta"}
	if err := repo.Create(ctx, ft1); err != nil {
		t.Fatalf("Create ft1 failed: %v", err)
	}
	if err := repo.Create(ctx, ft2); err != nil {
		t.Fatalf("Create ft2 failed: %v", err)
	}

	tags, err := repo.FindByFileID(ctx, 10)
	if err != nil {
		t.Fatalf("FindByFileID before delete failed: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags before delete, got %d", len(tags))
	}

	if err := repo.DeleteByFileID(ctx, 10); err != nil {
		t.Fatalf("DeleteByFileID failed: %v", err)
	}

	tags, err = repo.FindByFileID(ctx, 10)
	if err != nil {
		t.Fatalf("FindByFileID after delete failed: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("expected 0 tags after delete, got %d", len(tags))
	}
}

func TestFileTagRepository_CreateBatch(t *testing.T) {
	db := setupFileTagTestDB(t)
	repo := NewFileTagRepository(db)
	ctx := context.Background()

	batch := []model.FileTag{
		{FileID: 20, Tag: "red"},
		{FileID: 20, Tag: "green"},
		{FileID: 20, Tag: "blue"},
	}
	if err := repo.CreateBatch(ctx, batch); err != nil {
		t.Fatalf("CreateBatch failed: %v", err)
	}

	tags, err := repo.FindByFileID(ctx, 20)
	if err != nil {
		t.Fatalf("FindByFileID failed: %v", err)
	}
	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(tags))
	}

	tagNames := make(map[string]bool)
	for _, ft := range tags {
		tagNames[ft.Tag] = true
	}
	for _, expected := range []string{"red", "green", "blue"} {
		if !tagNames[expected] {
			t.Fatalf("expected tag '%s' not found", expected)
		}
	}
}
