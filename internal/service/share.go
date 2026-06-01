// internal/service/share.go
package service

import (
	"cloudbox/internal/config"
	"cloudbox/internal/model"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	ErrShareNotFound      = errors.New("share not found")
	ErrShareExpired       = errors.New("share has expired")
	ErrShareRevoked       = errors.New("share has been revoked")
	ErrShareLimitReached  = errors.New("download limit reached")
	ErrInvalidCredential  = errors.New("invalid or expired download credential")
	ErrWrongSharePassword = errors.New("wrong password")
)

type ShareService struct {
	shareRepo    ShareRepository
	fileRepo     FileRepository
	physicalRepo PhysicalFileRepository
	storage      Storage
	fileService  *FileService
	hasher       PasswordHasher
	audit        AuditRecorder
}

func NewShareService(
	shareRepo ShareRepository,
	fileRepo FileRepository,
	physicalRepo PhysicalFileRepository,
	storage Storage,
	fileService *FileService,
	hasher PasswordHasher,
	audit AuditRecorder,
) *ShareService {
	return &ShareService{
		shareRepo:    shareRepo,
		fileRepo:     fileRepo,
		physicalRepo: physicalRepo,
		storage:      storage,
		fileService:  fileService,
		hasher:       hasher,
		audit:        audit,
	}
}

// CreateShare creates a new share link for a file.
func (s *ShareService) CreateShare(ctx context.Context, userID, fileID int64, password string, expiresAt *time.Time, maxDownloads int) (*model.FileShare, error) {
	// Verify file ownership
	_, err := s.fileRepo.FindByIDAndOwner(ctx, fileID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("failed to verify file ownership: %w", err)
	}

	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	share := &model.FileShare{
		Token:        token,
		FileID:       fileID,
		OwnerID:      userID,
		MaxDownloads: maxDownloads,
		CreatedAt:    time.Now(),
	}

	if password != "" {
		hash, err := s.hasher.HashPassword(password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		share.PasswordHash = sql.NullString{String: hash, Valid: true}
	}

	if expiresAt != nil {
		share.ExpiresAt = sql.NullTime{Time: *expiresAt, Valid: true}
	}

	if err := s.shareRepo.Create(ctx, share); err != nil {
		return nil, fmt.Errorf("failed to create share: %w", err)
	}

	s.audit.Record(ctx, "share.create", "share", share.ID, fmt.Sprintf("token=%s", share.Token), fmt.Sprintf(`{"fileId":%d,"hasPassword":%v}`, fileID, password != ""))

	return share, nil
}

// GetShareInfo returns share details and the associated file info.
// The password hash is never returned to the caller.
func (s *ShareService) GetShareInfo(ctx context.Context, token string) (*model.FileShare, *model.File, error) {
	share, err := s.shareRepo.FindByToken(ctx, token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrShareNotFound
		}
		return nil, nil, fmt.Errorf("failed to find share: %w", err)
	}

	if err := checkShareValidity(share); err != nil {
		return nil, nil, err
	}

	file, err := s.fileRepo.FindByID(ctx, share.FileID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrFileNotFound
		}
		return nil, nil, fmt.Errorf("failed to find file: %w", err)
	}

	// Never expose password hash
	share.PasswordHash = sql.NullString{}

	return share, file, nil
}

// ShareCredential wraps the HMAC-based download credential.
type ShareCredential struct {
	Credential string `json:"credential"`
}

// VerifyOrGetCredential validates the share password (if any) and returns
// a time-limited HMAC credential for subsequent downloads.
func (s *ShareService) VerifyOrGetCredential(ctx context.Context, token, password string) (*ShareCredential, error) {
	share, err := s.shareRepo.FindByToken(ctx, token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrShareNotFound
		}
		return nil, fmt.Errorf("failed to find share: %w", err)
	}

	if err := checkShareValidity(share); err != nil {
		return nil, err
	}

	if share.PasswordHash.Valid {
		if !s.hasher.CheckPassword(password, share.PasswordHash.String) {
			return nil, ErrWrongSharePassword
		}
	}

	credential := generateCredential(share.Token)
	return &ShareCredential{Credential: credential}, nil
}

// DownloadByShare validates the HMAC credential and returns the file
// (and physical file) for download. For folders, only the file record
// is returned — the handler should use StreamFolderZipByID.
func (s *ShareService) DownloadByShare(ctx context.Context, token, credential string) (*model.File, *model.PhysicalFile, error) {
	if !verifyCredential(token, credential) {
		return nil, nil, ErrInvalidCredential
	}

	share, err := s.shareRepo.FindByToken(ctx, token)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrShareNotFound
		}
		return nil, nil, fmt.Errorf("failed to find share: %w", err)
	}

	if err := checkShareValidity(share); err != nil {
		return nil, nil, err
	}

	// Atomic increment — fails if limit would be exceeded
	ok, err := s.shareRepo.IncrementDownloadCount(ctx, token)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to increment download count: %w", err)
	}
	if !ok {
		return nil, nil, ErrShareLimitReached
	}

	file, err := s.fileRepo.FindByID(ctx, share.FileID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrFileNotFound
		}
		return nil, nil, fmt.Errorf("failed to find file: %w", err)
	}

	if file.IsFolder {
		return file, nil, nil
	}

	if file.ContentRef == 0 {
		return nil, nil, ErrNoPhysicalContent
	}

	pf, err := s.physicalRepo.FindByID(ctx, file.ContentRef)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find physical file: %w", err)
	}

	s.audit.Record(ctx, "share.download", "share", share.ID, file.Name, fmt.Sprintf(`{"token":"%s"}`, token))

	return file, pf, nil
}

// ListMyShares returns all shares owned by the given user.
func (s *ShareService) ListMyShares(ctx context.Context, userID int64) ([]model.FileShare, error) {
	return s.shareRepo.FindByOwner(ctx, userID)
}

// ListFileShares returns all shares for a given file, filtered to those
// owned by the requesting user.
func (s *ShareService) ListFileShares(ctx context.Context, userID, fileID int64) ([]model.FileShare, error) {
	shares, err := s.shareRepo.FindByFile(ctx, fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to find shares for file: %w", err)
	}

	// Filter to only the user's shares
	var result []model.FileShare
	for _, share := range shares {
		if share.OwnerID == userID {
			result = append(result, share)
		}
	}
	return result, nil
}

// RevokeShare revokes a share link.
func (s *ShareService) RevokeShare(ctx context.Context, userID, shareID int64) error {
	if err := s.shareRepo.Revoke(ctx, shareID, userID); err != nil {
		return err
	}

	s.audit.Record(ctx, "share.revoke", "share", shareID, "", "")

	return nil
}

// ---------- internal helpers ----------

// checkShareValidity checks whether a share is still usable.
func checkShareValidity(share *model.FileShare) error {
	if share.Revoked {
		return ErrShareRevoked
	}
	if share.ExpiresAt.Valid && !share.ExpiresAt.Time.After(time.Now()) {
		return ErrShareExpired
	}
	if share.MaxDownloads > 0 && share.DownloadCount >= share.MaxDownloads {
		return ErrShareLimitReached
	}
	return nil
}

const tokenCharset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// generateToken produces a cryptographically random token string.
func generateToken() (string, error) {
	cfg := config.Get()
	length := cfg.Share.TokenLength
	if length <= 0 {
		length = 8
	}

	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(tokenCharset))))
		if err != nil {
			return "", err
		}
		b[i] = tokenCharset[n.Int64()]
	}
	return string(b), nil
}

// generateCredential creates an HMAC-SHA256 credential in the format:
//
//	token.timestamp.hex_signature
func generateCredential(token string) string {
	cfg := config.Get()
	ts := time.Now().Unix()
	message := fmt.Sprintf("%s.%d", token, ts)

	mac := hmac.New(sha256.New, []byte(cfg.Share.Secret))
	mac.Write([]byte(message))
	sig := hex.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("%s.%d.%s", token, ts, sig)
}

// verifyCredential parses the credential and checks:
//  1. The token prefix matches
//  2. The timestamp is within ±CredentialTTL seconds of now
//  3. The HMAC signature is valid
func verifyCredential(token, credential string) bool {
	cfg := config.Get()
	parts := strings.SplitN(credential, ".", 3)
	if len(parts) != 3 {
		return false
	}

	credToken := parts[0]
	if credToken != token {
		return false
	}

	ts, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return false
	}

	ttl := cfg.Share.CredentialTTL
	if ttl <= 0 {
		ttl = 300
	}
	now := time.Now().Unix()
	if now-ts > int64(ttl) || ts-now > int64(ttl) {
		return false
	}

	message := fmt.Sprintf("%s.%d", credToken, ts)
	mac := hmac.New(sha256.New, []byte(cfg.Share.Secret))
	mac.Write([]byte(message))
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(parts[2]), []byte(expectedSig))
}
