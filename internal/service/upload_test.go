package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cloudbox/internal/model"
)

const testDefaultQuota = 10 * 1024 * 1024 * 1024 // 10GB

func TestUploadService_InitUpload(t *testing.T) {
	tests := []struct {
		name        string
		req         InitUploadRequest
		userID      int64
		setup       func(*mockFileRepo, *mockPhysicalFileRepo, *mockUserRepo, *mockStorage)
		wantErr     string
		wantInstant bool
	}{
		{
			name: "invalid_md5",
			req:  InitUploadRequest{MD5: "invalid", FileName: "test.txt", FileSize: 100, TargetFolderID: 0},
			setup: func(fr *mockFileRepo, pr *mockPhysicalFileRepo, ur *mockUserRepo, st *mockStorage) {},
			wantErr: "invalid MD5 format",
		},
		{
			name: "empty_filename",
			req:  InitUploadRequest{MD5: "d41d8cd98f00b204e9800998ecf8427e", FileName: "", FileSize: 0, TargetFolderID: 0},
			setup: func(fr *mockFileRepo, pr *mockPhysicalFileRepo, ur *mockUserRepo, st *mockStorage) {},
			wantErr: "file name is required",
		},
		{
			name: "quota_exceeded",
			req:  InitUploadRequest{MD5: "d41d8cd98f00b204e9800998ecf8427e", FileName: "big.txt", FileSize: 1024, TargetFolderID: 0},
			setup: func(fr *mockFileRepo, pr *mockPhysicalFileRepo, ur *mockUserRepo, st *mockStorage) {
				pr.calculateUserStorageUsageFn = func(ctx context.Context, userID int64) (int64, error) {
					return testDefaultQuota, nil // already at limit
				}
			},
			wantErr: "storage quota exceeded",
		},
		{
			name: "quota_user_override_exceeded",
			req:  InitUploadRequest{MD5: "d41d8cd98f00b204e9800998ecf8427e", FileName: "big.txt", FileSize: 1024, TargetFolderID: 0},
			setup: func(fr *mockFileRepo, pr *mockPhysicalFileRepo, ur *mockUserRepo, st *mockStorage) {
				smallQuota := int64(100)
				ur.getQuotaFn = func(ctx context.Context, userID int64) (*int64, error) {
					return &smallQuota, nil
				}
				pr.calculateUserStorageUsageFn = func(ctx context.Context, userID int64) (int64, error) {
					return 50, nil // 50/100 used, adding 1024 exceeds
				}
			},
			wantErr: "storage quota exceeded",
		},
		{
			name: "quota_unlimited",
			req:  InitUploadRequest{MD5: "d41d8cd98f00b204e9800998ecf8427e", FileName: "big.txt", FileSize: 1024, TargetFolderID: 0},
			setup: func(fr *mockFileRepo, pr *mockPhysicalFileRepo, ur *mockUserRepo, st *mockStorage) {
				unlimited := int64(0)
				ur.getQuotaFn = func(ctx context.Context, userID int64) (*int64, error) {
					return &unlimited, nil
				}
				pr.findByMD5Fn = func(ctx context.Context, md5 string) (*model.PhysicalFile, error) {
					return nil, errors.New("not found")
				}
				tmpDir := t.TempDir()
				st.tempChunkDirFn = func(uploadID string) string {
					return filepath.Join(tmpDir, "chunks", uploadID)
				}
			},
			wantInstant: false,
		},
		{
			name: "instant_upload_success",
			req:  InitUploadRequest{MD5: "d41d8cd98f00b204e9800998ecf8427e", FileName: "test.txt", FileSize: 0, TargetFolderID: 0},
			setup: func(fr *mockFileRepo, pr *mockPhysicalFileRepo, ur *mockUserRepo, st *mockStorage) {
				pr.findByMD5Fn = func(ctx context.Context, md5 string) (*model.PhysicalFile, error) {
					return &model.PhysicalFile{ID: 10, MD5: md5, Size: 0}, nil
				}
				fr.createFn = func(ctx context.Context, file *model.File) error {
					return nil
				}
				pr.incrementRefCountFn = func(ctx context.Context, id int64) error {
					return nil
				}
			},
			wantInstant: true,
		},
		{
			name: "instant_upload_create_error",
			req:  InitUploadRequest{MD5: "d41d8cd98f00b204e9800998ecf8427e", FileName: "test.txt", FileSize: 0, TargetFolderID: 0},
			setup: func(fr *mockFileRepo, pr *mockPhysicalFileRepo, ur *mockUserRepo, st *mockStorage) {
				pr.findByMD5Fn = func(ctx context.Context, md5 string) (*model.PhysicalFile, error) {
					return &model.PhysicalFile{ID: 10, MD5: md5}, nil
				}
				fr.createFn = func(ctx context.Context, file *model.File) error {
					return errors.New("db error")
				}
			},
			wantErr: "db error",
		},
		{
			name: "instant_upload_ref_increment_error",
			req:  InitUploadRequest{MD5: "d41d8cd98f00b204e9800998ecf8427e", FileName: "test.txt", FileSize: 0, TargetFolderID: 0},
			setup: func(fr *mockFileRepo, pr *mockPhysicalFileRepo, ur *mockUserRepo, st *mockStorage) {
				pr.findByMD5Fn = func(ctx context.Context, md5 string) (*model.PhysicalFile, error) {
					return &model.PhysicalFile{ID: 10, MD5: md5}, nil
				}
				fr.createFn = func(ctx context.Context, file *model.File) error {
					return nil
				}
				pr.incrementRefCountFn = func(ctx context.Context, id int64) error {
					return errors.New("ref error")
				}
			},
			wantErr: "ref error",
		},
		{
			name: "chunked_upload_no_existing_chunks",
			req:  InitUploadRequest{MD5: "d41d8cd98f00b204e9800998ecf8427e", FileName: "test.txt", FileSize: 1024, TargetFolderID: 0},
			setup: func(fr *mockFileRepo, pr *mockPhysicalFileRepo, ur *mockUserRepo, st *mockStorage) {
				pr.findByMD5Fn = func(ctx context.Context, md5 string) (*model.PhysicalFile, error) {
					return nil, errors.New("not found")
				}
				tmpDir := t.TempDir()
				st.tempChunkDirFn = func(uploadID string) string {
					return filepath.Join(tmpDir, "chunks", uploadID)
				}
			},
			wantInstant: false,
		},
		{
			name: "chunked_upload_existing_chunks",
			req:  InitUploadRequest{MD5: "d41d8cd98f00b204e9800998ecf8427e", FileName: "test.txt", FileSize: 1024, TargetFolderID: 0},
			setup: func(fr *mockFileRepo, pr *mockPhysicalFileRepo, ur *mockUserRepo, st *mockStorage) {
				pr.findByMD5Fn = func(ctx context.Context, md5 string) (*model.PhysicalFile, error) {
					return nil, errors.New("not found")
				}
				tmpDir := t.TempDir()
				uploadID := "d41d8cd98f00b204e9800998ecf8427e_1"
				chunkDir := filepath.Join(tmpDir, "chunks", uploadID)
				os.MkdirAll(chunkDir, 0755)
				os.WriteFile(filepath.Join(chunkDir, "0.chunk"), []byte("data"), 0644)
				os.WriteFile(filepath.Join(chunkDir, "1.chunk"), []byte("data"), 0644)
				st.tempChunkDirFn = func(uploadID string) string {
					return filepath.Join(tmpDir, "chunks", uploadID)
				}
			},
			wantInstant: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fr := &mockFileRepo{}
			pr := &mockPhysicalFileRepo{}
			ur := &mockUserRepo{}
			st := &mockStorage{}
			tt.setup(fr, pr, ur, st)

			svc := NewUploadService(fr, pr, ur, st, &mockImageProcessor{}, 5*1024*1024, testDefaultQuota)
			resp, err := svc.InitUpload(context.Background(), tt.userID, tt.req)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("got error %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Instant != tt.wantInstant {
				t.Errorf("Instant = %v, want %v", resp.Instant, tt.wantInstant)
			}
		})
	}
}

func TestUploadService_SaveChunk(t *testing.T) {
	tests := []struct {
		name       string
		uploadID   string
		chunkIndex int
		wantErr    string
	}{
		{
			name:     "invalid_upload_id",
			uploadID: "bad",
			wantErr:  "invalid uploadID format",
		},
		{
			name:       "success",
			uploadID:   "d41d8cd98f00b204e9800998ecf8427e_1",
			chunkIndex: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			st := &mockStorage{
				tempChunkDirFn: func(uploadID string) string {
					return filepath.Join(tmpDir, "chunks", uploadID)
				},
			}

			svc := NewUploadService(&mockFileRepo{}, &mockPhysicalFileRepo{}, &mockUserRepo{}, st, &mockImageProcessor{}, 5*1024*1024, testDefaultQuota)
			err := svc.SaveChunk(context.Background(), tt.uploadID, tt.chunkIndex, strings.NewReader("test data"))

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("got error %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestUploadService_CancelUpload(t *testing.T) {
	tests := []struct {
		name     string
		uploadID string
		wantErr  string
	}{
		{
			name:     "invalid_upload_id",
			uploadID: "bad",
			wantErr:  "invalid uploadID format",
		},
		{
			name:     "success",
			uploadID: "d41d8cd98f00b204e9800998ecf8427e_1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			st := &mockStorage{
				tempChunkDirFn: func(uploadID string) string {
					return filepath.Join(tmpDir, "chunks", uploadID)
				},
			}

			svc := NewUploadService(&mockFileRepo{}, &mockPhysicalFileRepo{}, &mockUserRepo{}, st, &mockImageProcessor{}, 5*1024*1024, testDefaultQuota)
			err := svc.CancelUpload(context.Background(), tt.uploadID)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("got error %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestUploadService_CompleteUpload(t *testing.T) {
	content := "hello world"
	hash := md5.Sum([]byte(content))
	md5Str := hex.EncodeToString(hash[:])
	uploadID := fmt.Sprintf("%s_1", md5Str)
	chunkSize := int64(5 * 1024 * 1024)

	tests := []struct {
		name             string
		req              CompleteUploadRequest
		setup            func(tmpDir string, fr *mockFileRepo, pr *mockPhysicalFileRepo, st *mockStorage)
		wantErr          string
		needProcessImage bool
	}{
		{
			name: "invalid_upload_id",
			req:  CompleteUploadRequest{FileName: "test.txt", FileSize: 11, MD5: md5Str, TargetFolderID: 0},
			setup: func(tmpDir string, fr *mockFileRepo, pr *mockPhysicalFileRepo, st *mockStorage) {
				st.tempChunkDirFn = func(uploadID string) string {
					return filepath.Join(tmpDir, "chunks", "bad")
				}
			},
			wantErr: "invalid uploadID format",
		},
		{
			name: "missing_chunk",
			req:  CompleteUploadRequest{FileName: "test.txt", FileSize: 11, MD5: md5Str, TargetFolderID: 0},
			setup: func(tmpDir string, fr *mockFileRepo, pr *mockPhysicalFileRepo, st *mockStorage) {
				chunkDir := filepath.Join(tmpDir, "chunks", uploadID)
				os.MkdirAll(chunkDir, 0755)
				// No chunk files created
				st.tempChunkDirFn = func(uid string) string {
					return filepath.Join(tmpDir, "chunks", uid)
				}
				st.generateFilePathFn = func(physicalID int64, ext string) (string, string) {
					return "2024/01/01/1.txt", filepath.Join(tmpDir, "files", "2024/01/01/1.txt")
				}
				st.ensureParentDirFn = func(path string) error {
					return os.MkdirAll(filepath.Dir(path), 0755)
				}
			},
			wantErr: "chunk not found",
		},
		{
			name: "md5_mismatch",
			req:  CompleteUploadRequest{FileName: "test.txt", FileSize: 11, MD5: "00000000000000000000000000000000", TargetFolderID: 0},
			setup: func(tmpDir string, fr *mockFileRepo, pr *mockPhysicalFileRepo, st *mockStorage) {
				chunkDir := filepath.Join(tmpDir, "chunks", uploadID)
				os.MkdirAll(chunkDir, 0755)
				os.WriteFile(filepath.Join(chunkDir, "0.chunk"), []byte(content), 0644)
				st.tempChunkDirFn = func(uid string) string {
					return filepath.Join(tmpDir, "chunks", uid)
				}
				st.generateFilePathFn = func(physicalID int64, ext string) (string, string) {
					return "2024/01/01/1.txt", filepath.Join(tmpDir, "files", "2024/01/01/1.txt")
				}
				st.ensureParentDirFn = func(path string) error {
					return os.MkdirAll(filepath.Dir(path), 0755)
				}
			},
			wantErr: "MD5 mismatch",
		},
		{
			name: "size_mismatch",
			req:  CompleteUploadRequest{FileName: "test.txt", FileSize: 999, MD5: md5Str, TargetFolderID: 0},
			setup: func(tmpDir string, fr *mockFileRepo, pr *mockPhysicalFileRepo, st *mockStorage) {
				chunkDir := filepath.Join(tmpDir, "chunks", uploadID)
				os.MkdirAll(chunkDir, 0755)
				os.WriteFile(filepath.Join(chunkDir, "0.chunk"), []byte(content), 0644)
				st.tempChunkDirFn = func(uid string) string {
					return filepath.Join(tmpDir, "chunks", uid)
				}
				st.generateFilePathFn = func(physicalID int64, ext string) (string, string) {
					return "2024/01/01/1.txt", filepath.Join(tmpDir, "files", "2024/01/01/1.txt")
				}
				st.ensureParentDirFn = func(path string) error {
					return os.MkdirAll(filepath.Dir(path), 0755)
				}
			},
			wantErr: "file size mismatch",
		},
		{
			name: "success",
			req:  CompleteUploadRequest{FileName: "test.txt", FileSize: int64(len(content)), MD5: md5Str, TargetFolderID: 0},
			setup: func(tmpDir string, fr *mockFileRepo, pr *mockPhysicalFileRepo, st *mockStorage) {
				chunkDir := filepath.Join(tmpDir, "chunks", uploadID)
				os.MkdirAll(chunkDir, 0755)
				os.WriteFile(filepath.Join(chunkDir, "0.chunk"), []byte(content), 0644)
				st.tempChunkDirFn = func(uid string) string {
					return filepath.Join(tmpDir, "chunks", uid)
				}
				st.generateFilePathFn = func(physicalID int64, ext string) (string, string) {
					return "2024/01/01/1.txt", filepath.Join(tmpDir, "files", "2024/01/01/1.txt")
				}
				st.ensureParentDirFn = func(path string) error {
					return os.MkdirAll(filepath.Dir(path), 0755)
				}
				pr.createFn = func(ctx context.Context, pf *model.PhysicalFile) error {
					pf.ID = 1
					return nil
				}
				pr.updateFn = func(ctx context.Context, pf *model.PhysicalFile) error {
					return nil
				}
				fr.createFn = func(ctx context.Context, file *model.File) error {
					return nil
				}
			},
			needProcessImage: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			fr := &mockFileRepo{}
			pr := &mockPhysicalFileRepo{}
			st := &mockStorage{}
			tt.setup(tmpDir, fr, pr, st)

			img := &mockImageProcessor{}
			done := make(chan struct{}, 1)
			if tt.needProcessImage {
				img.processImageFn = func(ctx context.Context, physicalID int64) error {
					done <- struct{}{}
					return nil
				}
			}

			svc := NewUploadService(fr, pr, &mockUserRepo{}, st, img, chunkSize, testDefaultQuota)

			// Use the correct uploadID for all tests except the invalid one
			uid := uploadID
			if tt.name == "invalid_upload_id" {
				uid = "bad"
			}

			_, err := svc.CompleteUpload(context.Background(), 1, uid, tt.req)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("got error %q, want containing %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.needProcessImage {
				select {
				case <-done:
				case <-time.After(2 * time.Second):
					t.Error("ProcessImage was not called")
				}
			}
		})
	}
}
