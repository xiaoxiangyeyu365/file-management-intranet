package service

import (
	"cloudbox/internal/model"
	"cloudbox/internal/repository"
	"context"
	"errors"
)

var (
	ErrClipboardEmpty    = errors.New("content is empty")
	ErrClipboardTooLong  = errors.New("content exceeds 10KB limit")
	ErrClipboardNotFound = errors.New("clipboard record not found")
)

const (
	MaxClipboardContentSize = 10240 // 10KB
	MaxClipboardRecords     = 50
)

type ClipboardService struct {
	repo  *repository.ClipboardRepository
	audit AuditRecorder
}

func NewClipboardService(repo *repository.ClipboardRepository, audit AuditRecorder) *ClipboardService {
	return &ClipboardService{repo: repo, audit: audit}
}

type CreateClipboardRequest struct {
	Content    string
	DeviceName string
	UserID     int64
}

func (s *ClipboardService) Create(ctx context.Context, req CreateClipboardRequest) (*model.ClipboardRecord, error) {
	if req.Content == "" {
		return nil, ErrClipboardEmpty
	}
	if len(req.Content) > MaxClipboardContentSize {
		return nil, ErrClipboardTooLong
	}

	deviceName := req.DeviceName
	if deviceName == "" {
		deviceName = "未命名设备"
	}

	record := &model.ClipboardRecord{
		Content:    req.Content,
		DeviceName: deviceName,
		UserID:     req.UserID,
		Pinned:     false,
	}

	if err := s.repo.Create(ctx, record); err != nil {
		return nil, err
	}

	// Auto cleanup: keep max 50 unpinned records
	count, err := s.repo.CountUnpinned(ctx, req.UserID)
	if err == nil && count > MaxClipboardRecords {
		deleteCount := int(count - MaxClipboardRecords + 1)
		s.repo.DeleteOldestUnpinned(ctx, req.UserID, deleteCount)
	}

	return record, nil
}

func (s *ClipboardService) List(ctx context.Context, userID int64) ([]model.ClipboardRecord, error) {
	return s.repo.FindByUser(ctx, userID, MaxClipboardRecords)
}

func (s *ClipboardService) TogglePin(ctx context.Context, userID, recordID int64, pinned bool) error {
	_, err := s.repo.FindByIDAndUser(ctx, recordID, userID)
	if err != nil {
		return ErrClipboardNotFound
	}
	return s.repo.UpdatePinned(ctx, recordID, pinned)
}

func (s *ClipboardService) Delete(ctx context.Context, userID, recordID int64) error {
	_, err := s.repo.FindByIDAndUser(ctx, recordID, userID)
	if err != nil {
		return ErrClipboardNotFound
	}
	return s.repo.DeleteByID(ctx, recordID)
}

func (s *ClipboardService) ClearUnpinned(ctx context.Context, userID int64) error {
	return s.repo.DeleteByUserUnpinned(ctx, userID)
}

func (s *ClipboardService) ClearAll(ctx context.Context, userID int64) error {
	return s.repo.DeleteByUser(ctx, userID)
}