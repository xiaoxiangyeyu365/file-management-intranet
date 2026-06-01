package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"cloudbox/internal/model"

	"gorm.io/gorm"
)

func TestFileService_CreateFolder(t *testing.T) {
	tests := []struct {
		name     string
		userID   int64
		parentID int64
		folderName string
		setup    func(*mockFileRepo)
		wantErr  error
	}{
		{
			name:       "name_conflict",
			userID:     1,
			parentID:   0,
			folderName: "existing",
			setup: func(fr *mockFileRepo) {
				fr.existsByNameFn = func(ctx context.Context, ownerID, parentID int64, name string) bool {
					return true
				}
			},
			wantErr: ErrNameConflict,
		},
		{
			name:       "create_error",
			userID:     1,
			parentID:   0,
			folderName: "newfolder",
			setup: func(fr *mockFileRepo) {
				fr.existsByNameFn = func(ctx context.Context, ownerID, parentID int64, name string) bool {
					return false
				}
				fr.createFn = func(ctx context.Context, file *model.File) error {
					return errors.New("db error")
				}
			},
			wantErr: errors.New("db error"),
		},
		{
			name:       "success_root_folder",
			userID:     1,
			parentID:   0,
			folderName: "myfolder",
			setup: func(fr *mockFileRepo) {
				fr.existsByNameFn = func(ctx context.Context, ownerID, parentID int64, name string) bool {
					return false
				}
				fr.createFn = func(ctx context.Context, file *model.File) error {
					if file.ParentID.Valid {
						t.Error("expected ParentID to be invalid for root folder")
					}
					if !file.IsFolder {
						t.Error("expected IsFolder to be true")
					}
					return nil
				}
			},
		},
		{
			name:       "success_subfolder",
			userID:     1,
			parentID:   5,
			folderName: "subfolder",
			setup: func(fr *mockFileRepo) {
				fr.existsByNameFn = func(ctx context.Context, ownerID, parentID int64, name string) bool {
					return false
				}
				fr.createFn = func(ctx context.Context, file *model.File) error {
					if !file.ParentID.Valid || file.ParentID.Int64 != 5 {
						t.Errorf("expected ParentID=5, got %v", file.ParentID)
					}
					return nil
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fr := &mockFileRepo{}
			tt.setup(fr)

			svc := NewFileService(fr, &mockPhysicalFileRepo{}, &mockStorage{})
			_, err := svc.CreateFolder(context.Background(), tt.userID, tt.parentID, tt.folderName)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if tt.wantErr.Error() != err.Error() {
					t.Errorf("got error %q, want %q", err.Error(), tt.wantErr.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestFileService_Rename(t *testing.T) {
	tests := []struct {
		name     string
		newName  string
		setup    func(*mockFileRepo)
		wantErr  error
	}{
		{
			name:    "file_not_found",
			newName: "newname",
			setup: func(fr *mockFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return nil, gorm.ErrRecordNotFound
				}
			},
			wantErr: ErrFileNotFound,
		},
		{
			name:    "same_name_noop",
			newName: "same",
			setup: func(fr *mockFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: 1, Name: "same", OwnerID: 1}, nil
				}
			},
		},
		{
			name:    "name_conflict",
			newName: "conflict",
			setup: func(fr *mockFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: 1, Name: "old", OwnerID: 1}, nil
				}
				fr.existsByNameFn = func(ctx context.Context, ownerID, parentID int64, name string) bool {
					return true
				}
			},
			wantErr: ErrNameConflict,
		},
		{
			name:    "success",
			newName: "newname",
			setup: func(fr *mockFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: 1, Name: "old", OwnerID: 1}, nil
				}
				fr.existsByNameFn = func(ctx context.Context, ownerID, parentID int64, name string) bool {
					return false
				}
				fr.updateFn = func(ctx context.Context, file *model.File) error {
					if file.Name != "newname" {
						t.Errorf("expected name newname, got %s", file.Name)
					}
					return nil
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fr := &mockFileRepo{}
			tt.setup(fr)

			svc := NewFileService(fr, &mockPhysicalFileRepo{}, &mockStorage{})
			err := svc.Rename(context.Background(), 1, 1, tt.newName)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if tt.wantErr.Error() != err.Error() {
					t.Errorf("got error %q, want %q", err.Error(), tt.wantErr.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestFileService_MoveToTrash(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*mockFileRepo)
		wantErr error
	}{
		{
			name: "file_not_found",
			setup: func(fr *mockFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return nil, gorm.ErrRecordNotFound
				}
			},
			wantErr: ErrFileNotFound,
		},
		{
			name: "success",
			setup: func(fr *mockFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: 1, OwnerID: 1}, nil
				}
				fr.softDeleteFn = func(ctx context.Context, id int64) error {
					if id != 1 {
						t.Errorf("expected soft delete id=1, got %d", id)
					}
					return nil
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fr := &mockFileRepo{}
			tt.setup(fr)

			svc := NewFileService(fr, &mockPhysicalFileRepo{}, &mockStorage{})
			err := svc.MoveToTrash(context.Background(), 1, 1)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if tt.wantErr.Error() != err.Error() {
					t.Errorf("got error %q, want %q", err.Error(), tt.wantErr.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestFileService_MoveFiles(t *testing.T) {
	tests := []struct {
		name         string
		fileIDs      []int64
		targetID     int64
		setup        func(*mockFileRepo)
		wantErr      error
	}{
		{
			name:     "target_not_found",
			fileIDs:  []int64{1},
			targetID: 99,
			setup: func(fr *mockFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return nil, gorm.ErrRecordNotFound
				}
			},
			wantErr: ErrInvalidTarget,
		},
		{
			name:     "target_not_folder",
			fileIDs:  []int64{1},
			targetID: 10,
			setup: func(fr *mockFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: 10, IsFolder: false, DeletedAt: sql.NullTime{}}, nil
				}
			},
			wantErr: ErrInvalidTarget,
		},
		{
			name:     "target_trashed",
			fileIDs:  []int64{1},
			targetID: 10,
			setup: func(fr *mockFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: 10, IsFolder: true, DeletedAt: sql.NullTime{Valid: true, Time: time.Now()}}, nil
				}
			},
			wantErr: ErrInvalidTarget,
		},
		{
			name:     "file_not_found",
			fileIDs:  []int64{999},
			targetID: 10,
			setup: func(fr *mockFileRepo) {
				callCount := 0
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					callCount++
					if callCount == 1 {
						return &model.File{ID: 10, IsFolder: true, DeletedAt: sql.NullTime{}}, nil
					}
					return nil, gorm.ErrRecordNotFound
				}
			},
			wantErr: ErrFileNotFound,
		},
		{
			name:     "circular_reference",
			fileIDs:  []int64{5},
			targetID: 10,
			setup: func(fr *mockFileRepo) {
				callCount := 0
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					callCount++
					if callCount == 1 {
						return &model.File{ID: 10, IsFolder: true, DeletedAt: sql.NullTime{}}, nil
					}
					return &model.File{ID: 5, Name: "docs", OwnerID: 1}, nil
				}
				fr.isAncestorFn = func(ctx context.Context, fileID, targetID int64) (bool, error) {
					return true, nil
				}
			},
			wantErr: ErrCircularReference,
		},
		{
			name:     "name_conflict",
			fileIDs:  []int64{5},
			targetID: 10,
			setup: func(fr *mockFileRepo) {
				callCount := 0
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					callCount++
					if callCount == 1 {
						return &model.File{ID: 10, IsFolder: true, DeletedAt: sql.NullTime{}}, nil
					}
					return &model.File{ID: 5, Name: "docs", OwnerID: 1}, nil
				}
				fr.isAncestorFn = func(ctx context.Context, fileID, targetID int64) (bool, error) {
					return false, nil
				}
				fr.existsByNameFn = func(ctx context.Context, ownerID, parentID int64, name string) bool {
					return true
				}
			},
			wantErr: ErrNameConflict,
		},
		{
			name:     "success",
			fileIDs:  []int64{5, 6},
			targetID: 10,
			setup: func(fr *mockFileRepo) {
				callCount := 0
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					callCount++
					if callCount == 1 {
						return &model.File{ID: 10, IsFolder: true, DeletedAt: sql.NullTime{}}, nil
					}
					return &model.File{ID: id, Name: "file" + string(rune('0'+id)), OwnerID: 1}, nil
				}
				fr.isAncestorFn = func(ctx context.Context, fileID, targetID int64) (bool, error) {
					return false, nil
				}
				fr.existsByNameFn = func(ctx context.Context, ownerID, parentID int64, name string) bool {
					return false
				}
				fr.batchUpdateParentFn = func(ctx context.Context, fileIDs []int64, newParentID int64) error {
					if len(fileIDs) != 2 {
						t.Errorf("expected 2 file IDs, got %d", len(fileIDs))
					}
					if newParentID != 10 {
						t.Errorf("expected target=10, got %d", newParentID)
					}
					return nil
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fr := &mockFileRepo{}
			tt.setup(fr)

			svc := NewFileService(fr, &mockPhysicalFileRepo{}, &mockStorage{})
			err := svc.MoveFiles(context.Background(), 1, tt.fileIDs, tt.targetID)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if tt.wantErr.Error() != err.Error() {
					t.Errorf("got error %q, want %q", err.Error(), tt.wantErr.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestFileService_RestoreFile(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*mockFileRepo)
		wantErr error
	}{
		{
			name: "file_not_found",
			setup: func(fr *mockFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return nil, gorm.ErrRecordNotFound
				}
			},
			wantErr: ErrFileNotFound,
		},
		{
			name: "parent_exists_and_active",
			setup: func(fr *mockFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: 1, Name: "file.txt", OwnerID: 1, ParentID: sql.NullInt64{Int64: 10, Valid: true}}, nil
				}
				fr.findByIDFn = func(ctx context.Context, id int64) (*model.File, error) {
					return &model.File{ID: 10, DeletedAt: sql.NullTime{}}, nil
				}
				fr.existsByNameFn = func(ctx context.Context, ownerID, parentID int64, name string) bool {
					return false
				}
				fr.restoreFn = func(ctx context.Context, id int64, newParentID *int64, newName string) error {
					if newParentID == nil || *newParentID != 10 {
						t.Errorf("expected parentID=10, got %v", newParentID)
					}
					return nil
				}
			},
		},
		{
			name: "parent_trashed",
			setup: func(fr *mockFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: 1, Name: "file.txt", OwnerID: 1, ParentID: sql.NullInt64{Int64: 10, Valid: true}}, nil
				}
				fr.findByIDFn = func(ctx context.Context, id int64) (*model.File, error) {
					return &model.File{ID: 10, DeletedAt: sql.NullTime{Valid: true, Time: time.Now()}}, nil
				}
				fr.existsByNameFn = func(ctx context.Context, ownerID, parentID int64, name string) bool {
					return false
				}
				fr.restoreFn = func(ctx context.Context, id int64, newParentID *int64, newName string) error {
					if newParentID != nil {
						t.Errorf("expected nil parentID for trashed parent, got %v", newParentID)
					}
					return nil
				}
			},
		},
		{
			name: "parent_not_found",
			setup: func(fr *mockFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: 1, Name: "file.txt", OwnerID: 1, ParentID: sql.NullInt64{Int64: 99, Valid: true}}, nil
				}
				fr.findByIDFn = func(ctx context.Context, id int64) (*model.File, error) {
					return nil, gorm.ErrRecordNotFound
				}
				fr.existsByNameFn = func(ctx context.Context, ownerID, parentID int64, name string) bool {
					return false
				}
				fr.restoreFn = func(ctx context.Context, id int64, newParentID *int64, newName string) error {
					if newParentID != nil {
						t.Errorf("expected nil parentID, got %v", newParentID)
					}
					return nil
				}
			},
		},
		{
			name: "name_conflict_suffix_added",
			setup: func(fr *mockFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: 1, Name: "file.txt", OwnerID: 1}, nil
				}
				fr.existsByNameFn = func(ctx context.Context, ownerID, parentID int64, name string) bool {
					return true // name conflict
				}
				fr.restoreFn = func(ctx context.Context, id int64, newParentID *int64, newName string) error {
					if newName == "file.txt" {
						t.Error("expected suffixed name, got original")
					}
					return nil
				}
			},
		},
		{
			name: "success_no_conflict",
			setup: func(fr *mockFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: 1, Name: "file.txt", OwnerID: 1}, nil
				}
				fr.existsByNameFn = func(ctx context.Context, ownerID, parentID int64, name string) bool {
					return false
				}
				fr.restoreFn = func(ctx context.Context, id int64, newParentID *int64, newName string) error {
					if newName != "file.txt" {
						t.Errorf("expected original name, got %s", newName)
					}
					return nil
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fr := &mockFileRepo{}
			tt.setup(fr)

			svc := NewFileService(fr, &mockPhysicalFileRepo{}, &mockStorage{})
			err := svc.RestoreFile(context.Background(), 1, 1)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if tt.wantErr.Error() != err.Error() {
					t.Errorf("got error %q, want %q", err.Error(), tt.wantErr.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestFileService_PermanentDelete(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*mockFileRepo, *mockPhysicalFileRepo)
		wantErr error
	}{
		{
			name: "file_not_found",
			setup: func(fr *mockFileRepo, pr *mockPhysicalFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return nil, gorm.ErrRecordNotFound
				}
			},
			wantErr: ErrFileNotFound,
		},
		{
			name: "single_file_ref_count_not_zero",
			setup: func(fr *mockFileRepo, pr *mockPhysicalFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: 1, OwnerID: 1, ContentRef: 10}, nil
				}
				fr.findAllDescendantsFn = func(ctx context.Context, parentID int64) ([]model.File, error) {
					return nil, nil
				}
				fr.deleteFn = func(ctx context.Context, id int64) error {
					return nil
				}
				pr.decrementRefCountFn = func(ctx context.Context, id int64, count int) (int, error) {
					return 2, nil // still referenced
				}
			},
		},
		{
			name: "single_file_ref_count_zero",
			setup: func(fr *mockFileRepo, pr *mockPhysicalFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: 1, OwnerID: 1, ContentRef: 10}, nil
				}
				fr.findAllDescendantsFn = func(ctx context.Context, parentID int64) ([]model.File, error) {
					return nil, nil
				}
				fr.deleteFn = func(ctx context.Context, id int64) error {
					return nil
				}
				pr.decrementRefCountFn = func(ctx context.Context, id int64, count int) (int, error) {
					return 0, nil
				}
				pr.findByIDFn = func(ctx context.Context, id int64) (*model.PhysicalFile, error) {
					return &model.PhysicalFile{ID: 10, StoragePath: "2024/01/01/10.txt"}, nil
				}
				pr.deleteFn = func(ctx context.Context, id int64) error {
					return nil
				}
			},
		},
		{
			name: "folder_with_descendants",
			setup: func(fr *mockFileRepo, pr *mockPhysicalFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: 1, OwnerID: 1, IsFolder: true}, nil
				}
				fr.findAllDescendantsFn = func(ctx context.Context, parentID int64) ([]model.File, error) {
					return []model.File{
						{ID: 2, OwnerID: 1, ContentRef: 20},
						{ID: 3, OwnerID: 1, IsFolder: true},
					}, nil
				}
				var deletedIDs []int64
				fr.deleteFn = func(ctx context.Context, id int64) error {
					deletedIDs = append(deletedIDs, id)
					return nil
				}
				pr.decrementRefCountFn = func(ctx context.Context, id int64, count int) (int, error) {
					return 0, nil
				}
				pr.findByIDFn = func(ctx context.Context, id int64) (*model.PhysicalFile, error) {
					return &model.PhysicalFile{ID: 20, StoragePath: "2024/01/01/20.txt"}, nil
				}
				pr.deleteFn = func(ctx context.Context, id int64) error {
					return nil
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fr := &mockFileRepo{}
			pr := &mockPhysicalFileRepo{}
			tt.setup(fr, pr)

			st := &mockStorage{
				toAbsPathFn: func(relative string) string {
					return t.TempDir() + "/" + relative
				},
			}

			svc := NewFileService(fr, pr, st)
			err := svc.PermanentDelete(context.Background(), 1, 1)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if tt.wantErr.Error() != err.Error() {
					t.Errorf("got error %q, want %q", err.Error(), tt.wantErr.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestFileService_ListFiles(t *testing.T) {
	fr := &mockFileRepo{
		findByParentAndOwnerFn: func(ctx context.Context, parentID, ownerID int64, includeDeleted bool) ([]model.File, error) {
			return []model.File{{ID: 1, Name: "test.txt"}}, nil
		},
	}
	svc := NewFileService(fr, &mockPhysicalFileRepo{}, &mockStorage{})
	files, err := svc.ListFiles(context.Background(), 1, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
}

func TestFileService_GetFile(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*mockFileRepo)
		wantErr error
	}{
		{
			name: "not_found",
			setup: func(fr *mockFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return nil, gorm.ErrRecordNotFound
				}
			},
			wantErr: ErrFileNotFound,
		},
		{
			name: "success",
			setup: func(fr *mockFileRepo) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: 1, Name: "test.txt"}, nil
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fr := &mockFileRepo{}
			tt.setup(fr)
			svc := NewFileService(fr, &mockPhysicalFileRepo{}, &mockStorage{})
			_, err := svc.GetFile(context.Background(), 1, 1)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if tt.wantErr.Error() != err.Error() {
					t.Errorf("got error %q, want %q", err.Error(), tt.wantErr.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestFileService_SearchFiles(t *testing.T) {
	tests := []struct {
		name    string
		keyword string
		sort    string
		wantErr bool
	}{
		{name: "empty_keyword", keyword: "", sort: "relevance", wantErr: true},
		{name: "invalid_sort_defaults_to_relevance", keyword: "test", sort: "invalid", wantErr: false},
		{name: "success", keyword: "test", sort: "name", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fr := &mockFileRepo{
				searchFn: func(ctx context.Context, userID int64, keyword string, folderID *int64, sort string) ([]model.File, error) {
					return []model.File{}, nil
				},
			}
			svc := NewFileService(fr, &mockPhysicalFileRepo{}, &mockStorage{})
			_, err := svc.SearchFiles(context.Background(), 1, tt.keyword, nil, tt.sort)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
