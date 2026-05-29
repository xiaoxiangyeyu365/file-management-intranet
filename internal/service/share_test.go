package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"cloudbox/internal/config"
	"cloudbox/internal/model"

	"gorm.io/gorm"
)

// newTestShareService builds a ShareService with the given mocks.
func newTestShareService(sr *mockShareRepo, fr *mockFileRepo, pr *mockPhysicalFileRepo) *ShareService {
	return NewShareService(sr, fr, pr, &mockStorage{}, &FileService{fileRepo: fr, physicalRepo: pr, storage: &mockStorage{}}, &mockPasswordHasher{})
}

// ensureConfig loads the config singleton once so that generateToken/generateCredential
// do not fail in tests.
func init() {
	config.Load()
}

// --------------- CreateShare ---------------

func TestShareService_CreateShare(t *testing.T) {
	tests := []struct {
		name        string
		userID      int64
		fileID      int64
		password    string
		expiresAt   *time.Time
		maxDownloads int
		setup       func(*mockShareRepo, *mockFileRepo, *mockPasswordHasher)
		wantErr     error
		check       func(*model.FileShare) error
	}{
		{
			name:   "file_not_found",
			userID: 1,
			fileID: 999,
			setup: func(sr *mockShareRepo, fr *mockFileRepo, ph *mockPasswordHasher) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return nil, gorm.ErrRecordNotFound
				}
			},
			wantErr: ErrFileNotFound,
		},
		{
			name:   "repo_error_on_ownership_check",
			userID: 1,
			fileID: 1,
			setup: func(sr *mockShareRepo, fr *mockFileRepo, ph *mockPasswordHasher) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return nil, errors.New("db error")
				}
			},
			wantErr: errors.New("failed to verify file ownership: db error"),
		},
		{
			name:        "success_without_password",
			userID:      1,
			fileID:      10,
			password:    "",
			maxDownloads: 5,
			setup: func(sr *mockShareRepo, fr *mockFileRepo, ph *mockPasswordHasher) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: id, OwnerID: ownerID}, nil
				}
				sr.createFn = func(ctx context.Context, share *model.FileShare) error {
					return nil
				}
			},
			check: func(share *model.FileShare) error {
				if share.Token == "" {
					return errors.New("expected token to be generated")
				}
				if share.PasswordHash.Valid {
					return errors.New("expected no password hash for share without password")
				}
				if share.MaxDownloads != 5 {
					return errors.New("expected MaxDownloads=5")
				}
				return nil
			},
		},
		{
			name:     "success_with_password",
			userID:   1,
			fileID:   10,
			password: "secret123",
			setup: func(sr *mockShareRepo, fr *mockFileRepo, ph *mockPasswordHasher) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: id, OwnerID: ownerID}, nil
				}
				ph.hashPasswordFn = func(password string) (string, error) {
					return "hashed_" + password, nil
				}
				sr.createFn = func(ctx context.Context, share *model.FileShare) error {
					if !share.PasswordHash.Valid {
						t.Error("expected PasswordHash to be valid")
					}
					if share.PasswordHash.String != "hashed_secret123" {
						t.Errorf("expected hashed password, got %s", share.PasswordHash.String)
					}
					return nil
				}
			},
			check: func(share *model.FileShare) error {
				if !share.PasswordHash.Valid {
					return errors.New("expected password hash to be set")
				}
				return nil
			},
		},
		{
			name:     "hash_password_error",
			userID:   1,
			fileID:   10,
			password: "secret123",
			setup: func(sr *mockShareRepo, fr *mockFileRepo, ph *mockPasswordHasher) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: id, OwnerID: ownerID}, nil
				}
				ph.hashPasswordFn = func(password string) (string, error) {
					return "", errors.New("hash failed")
				}
			},
			wantErr: errors.New("failed to hash password: hash failed"),
		},
		{
			name:   "success_with_expiry",
			userID: 1,
			fileID: 10,
			expiresAt: func() *time.Time {
				t := time.Now().Add(24 * time.Hour)
				return &t
			}(),
			setup: func(sr *mockShareRepo, fr *mockFileRepo, ph *mockPasswordHasher) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: id, OwnerID: ownerID}, nil
				}
				sr.createFn = func(ctx context.Context, share *model.FileShare) error {
					return nil
				}
			},
			check: func(share *model.FileShare) error {
				if !share.ExpiresAt.Valid {
					return errors.New("expected ExpiresAt to be set")
				}
				return nil
			},
		},
		{
			name:   "create_repo_error",
			userID: 1,
			fileID: 10,
			setup: func(sr *mockShareRepo, fr *mockFileRepo, ph *mockPasswordHasher) {
				fr.findByIDAndOwnerFn = func(ctx context.Context, id, ownerID int64) (*model.File, error) {
					return &model.File{ID: id, OwnerID: ownerID}, nil
				}
				sr.createFn = func(ctx context.Context, share *model.FileShare) error {
					return errors.New("db insert error")
				}
			},
			wantErr: errors.New("failed to create share: db insert error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := &mockShareRepo{}
			fr := &mockFileRepo{}
			pr := &mockPhysicalFileRepo{}
			ph := &mockPasswordHasher{}

			if tt.setup != nil {
				tt.setup(sr, fr, ph)
			}

			svc := NewShareService(sr, fr, pr, &mockStorage{}, &FileService{fileRepo: fr, physicalRepo: pr, storage: &mockStorage{}}, ph)
			share, err := svc.CreateShare(context.Background(), tt.userID, tt.fileID, tt.password, tt.expiresAt, tt.maxDownloads)

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
			if tt.check != nil {
				if checkErr := tt.check(share); checkErr != nil {
					t.Error(checkErr)
				}
			}
		})
	}
}

// --------------- GetShareInfo ---------------

func TestShareService_GetShareInfo(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		setup   func(*mockShareRepo, *mockFileRepo)
		wantErr error
	}{
		{
			name:  "not_found",
			token: "missing",
			setup: func(sr *mockShareRepo, fr *mockFileRepo) {
				sr.findByTokenFn = func(ctx context.Context, token string) (*model.FileShare, error) {
					return nil, gorm.ErrRecordNotFound
				}
			},
			wantErr: ErrShareNotFound,
		},
		{
			name:  "repo_error",
			token: "abc",
			setup: func(sr *mockShareRepo, fr *mockFileRepo) {
				sr.findByTokenFn = func(ctx context.Context, token string) (*model.FileShare, error) {
					return nil, errors.New("db error")
				}
			},
			wantErr: errors.New("failed to find share: db error"),
		},
		{
			name:  "expired",
			token: "expired",
			setup: func(sr *mockShareRepo, fr *mockFileRepo) {
				sr.findByTokenFn = func(ctx context.Context, token string) (*model.FileShare, error) {
					past := time.Now().Add(-1 * time.Hour)
					return &model.FileShare{
						Token:    token,
						ExpiresAt: sql.NullTime{Time: past, Valid: true},
					}, nil
				}
			},
			wantErr: ErrShareExpired,
		},
		{
			name:  "revoked",
			token: "revoked",
			setup: func(sr *mockShareRepo, fr *mockFileRepo) {
				sr.findByTokenFn = func(ctx context.Context, token string) (*model.FileShare, error) {
					return &model.FileShare{
						Token:   token,
						Revoked: true,
					}, nil
				}
			},
			wantErr: ErrShareRevoked,
		},
		{
			name:  "download_limit_reached",
			token: "limited",
			setup: func(sr *mockShareRepo, fr *mockFileRepo) {
				sr.findByTokenFn = func(ctx context.Context, token string) (*model.FileShare, error) {
					return &model.FileShare{
						Token:         token,
						MaxDownloads:  3,
						DownloadCount: 3,
					}, nil
				}
			},
			wantErr: ErrShareLimitReached,
		},
		{
			name:  "success",
			token: "valid",
			setup: func(sr *mockShareRepo, fr *mockFileRepo) {
				sr.findByTokenFn = func(ctx context.Context, token string) (*model.FileShare, error) {
					return &model.FileShare{
						Token:  token,
						FileID: 42,
						OwnerID: 1,
					}, nil
				}
				fr.findByIDFn = func(ctx context.Context, id int64) (*model.File, error) {
					return &model.File{ID: id, Name: "test.txt", OwnerID: 1}, nil
				}
			},
		},
		{
			name:  "file_not_found",
			token: "valid_token",
			setup: func(sr *mockShareRepo, fr *mockFileRepo) {
				sr.findByTokenFn = func(ctx context.Context, token string) (*model.FileShare, error) {
					return &model.FileShare{
						Token:  token,
						FileID: 99,
					}, nil
				}
				fr.findByIDFn = func(ctx context.Context, id int64) (*model.File, error) {
					return nil, gorm.ErrRecordNotFound
				}
			},
			wantErr: ErrFileNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := &mockShareRepo{}
			fr := &mockFileRepo{}
			pr := &mockPhysicalFileRepo{}
			tt.setup(sr, fr)

			svc := newTestShareService(sr, fr, pr)
			share, file, err := svc.GetShareInfo(context.Background(), tt.token)

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
			if share == nil {
				t.Fatal("expected non-nil share")
			}
			if file == nil {
				t.Fatal("expected non-nil file")
			}
			// Password hash must never be exposed
			if share.PasswordHash.Valid {
				t.Error("expected PasswordHash to be cleared in response")
			}
		})
	}
}

// --------------- VerifyOrGetCredential ---------------

func TestShareService_VerifyOrGetCredential(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		password string
		setup    func(*mockShareRepo, *mockPasswordHasher)
		wantErr  error
	}{
		{
			name:  "not_found",
			token: "missing",
			setup: func(sr *mockShareRepo, ph *mockPasswordHasher) {
				sr.findByTokenFn = func(ctx context.Context, token string) (*model.FileShare, error) {
					return nil, gorm.ErrRecordNotFound
				}
			},
			wantErr: ErrShareNotFound,
		},
		{
			name:  "expired",
			token: "expired",
			setup: func(sr *mockShareRepo, ph *mockPasswordHasher) {
				sr.findByTokenFn = func(ctx context.Context, token string) (*model.FileShare, error) {
					past := time.Now().Add(-1 * time.Hour)
					return &model.FileShare{
						Token:     token,
						ExpiresAt: sql.NullTime{Time: past, Valid: true},
					}, nil
				}
			},
			wantErr: ErrShareExpired,
		},
		{
			name:     "wrong_password",
			token:    "protected",
			password: "wrong",
			setup: func(sr *mockShareRepo, ph *mockPasswordHasher) {
				sr.findByTokenFn = func(ctx context.Context, token string) (*model.FileShare, error) {
					return &model.FileShare{
						Token:        token,
						PasswordHash: sql.NullString{String: "hashed_pw", Valid: true},
					}, nil
				}
				ph.checkPasswordFn = func(password, hash string) bool {
					return false
				}
			},
			wantErr: ErrWrongSharePassword,
		},
		{
			name:  "success_without_password",
			token: "open",
			setup: func(sr *mockShareRepo, ph *mockPasswordHasher) {
				sr.findByTokenFn = func(ctx context.Context, token string) (*model.FileShare, error) {
					return &model.FileShare{
						Token:        token,
						PasswordHash: sql.NullString{Valid: false},
					}, nil
				}
			},
		},
		{
			name:     "success_with_correct_password",
			token:    "protected",
			password: "correct",
			setup: func(sr *mockShareRepo, ph *mockPasswordHasher) {
				sr.findByTokenFn = func(ctx context.Context, token string) (*model.FileShare, error) {
					return &model.FileShare{
						Token:        token,
						PasswordHash: sql.NullString{String: "hashed_correct", Valid: true},
					}, nil
				}
				ph.checkPasswordFn = func(password, hash string) bool {
					return password == "correct" && hash == "hashed_correct"
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := &mockShareRepo{}
			fr := &mockFileRepo{}
			pr := &mockPhysicalFileRepo{}
			ph := &mockPasswordHasher{}
			tt.setup(sr, ph)

			svc := NewShareService(sr, fr, pr, &mockStorage{}, &FileService{fileRepo: fr, physicalRepo: pr, storage: &mockStorage{}}, ph)
			cred, err := svc.VerifyOrGetCredential(context.Background(), tt.token, tt.password)

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
			if cred == nil {
				t.Fatal("expected non-nil credential")
			}
			if cred.Credential == "" {
				t.Error("expected non-empty credential string")
			}
		})
	}
}

// --------------- DownloadByShare ---------------

// getValidCredential is a test helper that obtains a valid HMAC credential
// for the given token by temporarily adjusting the shareRepo mock to return
// a valid (non-expired, non-revoked) share with no password.
func getValidCredential(t *testing.T, svc *ShareService, sr *mockShareRepo, token string) string {
	t.Helper()
	origFn := sr.findByTokenFn
	sr.findByTokenFn = func(ctx context.Context, tok string) (*model.FileShare, error) {
		return &model.FileShare{
			Token:        tok,
			FileID:       10,
			OwnerID:      1,
			PasswordHash: sql.NullString{Valid: false},
		}, nil
	}
	cred, err := svc.VerifyOrGetCredential(context.Background(), token, "")
	if err != nil {
		t.Fatalf("failed to obtain valid credential for test setup: %v", err)
	}
	sr.findByTokenFn = origFn
	return cred.Credential
}

func TestShareService_DownloadByShare(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		setup    func(*mockShareRepo, *mockFileRepo, *mockPhysicalFileRepo)
		wantErr  error
		wantFile bool
		wantPhys bool
		// needCredential indicates whether a valid HMAC credential should
		// be obtained before calling DownloadByShare. false only for the
		// "invalid_credential" case which tests garbage input.
		needCredential bool
	}{
		{
			name:           "invalid_credential",
			token:          "abc",
			needCredential: false,
			setup: func(sr *mockShareRepo, fr *mockFileRepo, pr *mockPhysicalFileRepo) {
				// No mocks needed — credential check fails first
			},
			wantErr: ErrInvalidCredential,
		},
		{
			name:           "share_expired",
			token:          "expired",
			needCredential: true,
			setup: func(sr *mockShareRepo, fr *mockFileRepo, pr *mockPhysicalFileRepo) {
				sr.findByTokenFn = func(ctx context.Context, token string) (*model.FileShare, error) {
					past := time.Now().Add(-1 * time.Hour)
					return &model.FileShare{
						Token:     token,
						FileID:    10,
						ExpiresAt: sql.NullTime{Time: past, Valid: true},
					}, nil
				}
			},
			wantErr: ErrShareExpired,
		},
		{
			name:           "share_revoked",
			token:          "revoked",
			needCredential: true,
			setup: func(sr *mockShareRepo, fr *mockFileRepo, pr *mockPhysicalFileRepo) {
				sr.findByTokenFn = func(ctx context.Context, token string) (*model.FileShare, error) {
					return &model.FileShare{
						Token:   token,
						FileID:  10,
						Revoked: true,
					}, nil
				}
			},
			wantErr: ErrShareRevoked,
		},
		{
			name:           "download_limit_reached",
			token:          "limited",
			needCredential: true,
			setup: func(sr *mockShareRepo, fr *mockFileRepo, pr *mockPhysicalFileRepo) {
				sr.findByTokenFn = func(ctx context.Context, token string) (*model.FileShare, error) {
					return &model.FileShare{
						Token:    token,
						FileID:   10,
						OwnerID:  1,
					}, nil
				}
				sr.incrementDownloadCountFn = func(ctx context.Context, token string) (bool, error) {
					return false, nil
				}
			},
			wantErr: ErrShareLimitReached,
		},
		{
			name:           "success_file",
			token:          "fileshare",
			needCredential: true,
			setup: func(sr *mockShareRepo, fr *mockFileRepo, pr *mockPhysicalFileRepo) {
				sr.findByTokenFn = func(ctx context.Context, token string) (*model.FileShare, error) {
					return &model.FileShare{
						Token:   token,
						FileID:  10,
						OwnerID: 1,
					}, nil
				}
				sr.incrementDownloadCountFn = func(ctx context.Context, token string) (bool, error) {
					return true, nil
				}
				fr.findByIDFn = func(ctx context.Context, id int64) (*model.File, error) {
					return &model.File{ID: id, Name: "doc.pdf", ContentRef: 100, OwnerID: 1}, nil
				}
				pr.findByIDFn = func(ctx context.Context, id int64) (*model.PhysicalFile, error) {
					return &model.PhysicalFile{ID: id, StoragePath: "/files/doc.pdf", MD5: "abc123", Size: 1024}, nil
				}
			},
			wantFile: true,
			wantPhys: true,
		},
		{
			name:           "success_folder",
			token:          "foldershare",
			needCredential: true,
			setup: func(sr *mockShareRepo, fr *mockFileRepo, pr *mockPhysicalFileRepo) {
				sr.findByTokenFn = func(ctx context.Context, token string) (*model.FileShare, error) {
					return &model.FileShare{
						Token:   token,
						FileID:  20,
						OwnerID: 1,
					}, nil
				}
				sr.incrementDownloadCountFn = func(ctx context.Context, token string) (bool, error) {
					return true, nil
				}
				fr.findByIDFn = func(ctx context.Context, id int64) (*model.File, error) {
					return &model.File{ID: id, Name: "myfolder", IsFolder: true, OwnerID: 1}, nil
				}
			},
			wantFile: true,
			wantPhys: false,
		},
		{
			name:           "file_not_found_after_increment",
			token:          "ghostshare",
			needCredential: true,
			setup: func(sr *mockShareRepo, fr *mockFileRepo, pr *mockPhysicalFileRepo) {
				sr.findByTokenFn = func(ctx context.Context, token string) (*model.FileShare, error) {
					return &model.FileShare{
						Token:   token,
						FileID:  99,
						OwnerID: 1,
					}, nil
				}
				sr.incrementDownloadCountFn = func(ctx context.Context, token string) (bool, error) {
					return true, nil
				}
				fr.findByIDFn = func(ctx context.Context, id int64) (*model.File, error) {
					return nil, gorm.ErrRecordNotFound
				}
			},
			wantErr: ErrFileNotFound,
		},
		{
			name:           "no_physical_content",
			token:          "nophys",
			needCredential: true,
			setup: func(sr *mockShareRepo, fr *mockFileRepo, pr *mockPhysicalFileRepo) {
				sr.findByTokenFn = func(ctx context.Context, token string) (*model.FileShare, error) {
					return &model.FileShare{
						Token:   token,
						FileID:  30,
						OwnerID: 1,
					}, nil
				}
				sr.incrementDownloadCountFn = func(ctx context.Context, token string) (bool, error) {
					return true, nil
				}
				fr.findByIDFn = func(ctx context.Context, id int64) (*model.File, error) {
					return &model.File{ID: id, Name: "orphan.txt", ContentRef: 0, OwnerID: 1}, nil
				}
			},
			wantErr: ErrNoPhysicalContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := &mockShareRepo{}
			fr := &mockFileRepo{}
			pr := &mockPhysicalFileRepo{}
			ph := &mockPasswordHasher{}

			tt.setup(sr, fr, pr)

			svc := NewShareService(sr, fr, pr, &mockStorage{}, &FileService{fileRepo: fr, physicalRepo: pr, storage: &mockStorage{}}, ph)

			var credential string
			if tt.needCredential {
				credential = getValidCredential(t, svc, sr, tt.token)
			} else {
				credential = "garbage"
			}

			file, pf, err := svc.DownloadByShare(context.Background(), tt.token, credential)

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
			if tt.wantFile && file == nil {
				t.Error("expected non-nil file")
			}
			if tt.wantPhys && pf == nil {
				t.Error("expected non-nil physical file")
			}
			if !tt.wantPhys && pf != nil {
				t.Error("expected nil physical file")
			}
		})
	}
}

// --------------- RevokeShare ---------------

func TestShareService_RevokeShare(t *testing.T) {
	tests := []struct {
		name    string
		userID  int64
		shareID int64
		setup   func(*mockShareRepo)
		wantErr error
	}{
		{
			name:    "success",
			userID:  1,
			shareID: 10,
			setup: func(sr *mockShareRepo) {
				sr.revokeFn = func(ctx context.Context, id, ownerID int64) error {
					if id != 10 {
						t.Errorf("expected share id 10, got %d", id)
					}
					if ownerID != 1 {
						t.Errorf("expected owner id 1, got %d", ownerID)
					}
					return nil
				}
			},
		},
		{
			name:    "not_found_or_unauthorized",
			userID:  2,
			shareID: 99,
			setup: func(sr *mockShareRepo) {
				sr.revokeFn = func(ctx context.Context, id, ownerID int64) error {
					return gorm.ErrRecordNotFound
				}
			},
			wantErr: gorm.ErrRecordNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := &mockShareRepo{}
			tt.setup(sr)

			svc := newTestShareService(sr, &mockFileRepo{}, &mockPhysicalFileRepo{})
			err := svc.RevokeShare(context.Background(), tt.userID, tt.shareID)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) && tt.wantErr.Error() != err.Error() {
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
