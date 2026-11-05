package repository

import (
	"context"
	"testing"
	"time"

	"cloudbox/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupFileTestDB(t *testing.T) (*gorm.DB, *FileRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.File{}, &model.PhysicalFile{}); err != nil {
		t.Fatalf("failed to auto-migrate: %v", err)
	}
	return db, NewFileRepository(db)
}

// seedSortableFiles creates one folder plus three files with distinct
// sizes and modification times inside the user's root folder.
func seedSortableFiles(t *testing.T, db *gorm.DB, ownerID int64) []model.File {
	t.Helper()

	folder := model.File{Name: "zzz-folder", OwnerID: ownerID, IsFolder: true}
	if err := db.Create(&folder).Error; err != nil {
		t.Fatalf("create folder: %v", err)
	}

	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	specs := []struct {
		name    string
		size    int64
		updated time.Time
	}{
		{"a.txt", 300, now.Add(48 * time.Hour)},
		{"b.txt", 100, now},
		{"c.txt", 50, now.Add(24 * time.Hour)},
	}

	files := make([]model.File, 0, len(specs))
	for _, spec := range specs {
		pf := model.PhysicalFile{
			StoragePath: "files/" + spec.name,
			MD5:         "md5-" + spec.name,
			Size:        spec.size,
			MimeType:    "text/plain",
		}
		if err := db.Create(&pf).Error; err != nil {
			t.Fatalf("create physical file: %v", err)
		}
		f := model.File{
			Name:       spec.name,
			OwnerID:    ownerID,
			ContentRef: pf.ID,
			UpdatedAt:  spec.updated,
		}
		if err := db.Create(&f).Error; err != nil {
			t.Fatalf("create file: %v", err)
		}
		files = append(files, f)
	}

	return files
}

func namesOf(files []model.File) []string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Name
	}
	return names
}

func assertNames(t *testing.T, got []model.File, want []string) {
	t.Helper()
	names := namesOf(got)
	if len(names) != len(want) {
		t.Fatalf("expected %d entries %v, got %d: %v", len(want), want, len(names), names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("expected order %v, got %v", want, names)
		}
	}
}

func TestFindByParentAndOwner_NameSort(t *testing.T) {
	db, repo := setupFileTestDB(t)
	ctx := context.Background()
	seedSortableFiles(t, db, 1)

	asc, err := repo.FindByParentAndOwner(ctx, 0, 1, false, "name", "asc")
	if err != nil {
		t.Fatalf("name asc: %v", err)
	}
	assertNames(t, asc, []string{"zzz-folder", "a.txt", "b.txt", "c.txt"})

	desc, err := repo.FindByParentAndOwner(ctx, 0, 1, false, "name", "desc")
	if err != nil {
		t.Fatalf("name desc: %v", err)
	}
	// Folders stay first; only files change direction.
	assertNames(t, desc, []string{"zzz-folder", "c.txt", "b.txt", "a.txt"})
}

func TestFindByParentAndOwner_SizeSort(t *testing.T) {
	db, repo := setupFileTestDB(t)
	ctx := context.Background()
	seedSortableFiles(t, db, 1)

	desc, err := repo.FindByParentAndOwner(ctx, 0, 1, false, "size", "desc")
	if err != nil {
		t.Fatalf("size desc: %v", err)
	}
	assertNames(t, desc, []string{"zzz-folder", "a.txt", "b.txt", "c.txt"})

	asc, err := repo.FindByParentAndOwner(ctx, 0, 1, false, "size", "asc")
	if err != nil {
		t.Fatalf("size asc: %v", err)
	}
	assertNames(t, asc, []string{"zzz-folder", "c.txt", "b.txt", "a.txt"})
}

func TestFindByParentAndOwner_UpdatedAtSort(t *testing.T) {
	db, repo := setupFileTestDB(t)
	ctx := context.Background()
	seedSortableFiles(t, db, 1)

	desc, err := repo.FindByParentAndOwner(ctx, 0, 1, false, "updatedAt", "desc")
	if err != nil {
		t.Fatalf("updatedAt desc: %v", err)
	}
	assertNames(t, desc, []string{"zzz-folder", "a.txt", "c.txt", "b.txt"})

	asc, err := repo.FindByParentAndOwner(ctx, 0, 1, false, "updatedAt", "asc")
	if err != nil {
		t.Fatalf("updatedAt asc: %v", err)
	}
	assertNames(t, asc, []string{"zzz-folder", "b.txt", "c.txt", "a.txt"})
}

func TestFindByParentAndOwnerPaged_SortsAcrossPages(t *testing.T) {
	db, repo := setupFileTestDB(t)
	ctx := context.Background()
	seedSortableFiles(t, db, 1)

	page1, total, err := repo.FindByParentAndOwnerPaged(ctx, 0, 1, 1, 2, "size", "desc")
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if total != 4 {
		t.Fatalf("expected total 4, got %d", total)
	}
	assertNames(t, page1, []string{"zzz-folder", "a.txt"})

	page2, _, err := repo.FindByParentAndOwnerPaged(ctx, 0, 1, 2, 2, "size", "desc")
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	assertNames(t, page2, []string{"b.txt", "c.txt"})
}

func TestFindByParentAndOwner_InvalidSortFallsBackToName(t *testing.T) {
	db, repo := setupFileTestDB(t)
	ctx := context.Background()
	seedSortableFiles(t, db, 1)

	files, err := repo.FindByParentAndOwner(ctx, 0, 1, false, "DROP TABLE files", "sideways")
	if err != nil {
		t.Fatalf("invalid sort: %v", err)
	}
	assertNames(t, files, []string{"zzz-folder", "a.txt", "b.txt", "c.txt"})
}
