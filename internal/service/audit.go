package service

import (
	"cloudbox/internal/model"
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const (
	auditChannelSize   = 256
	auditBatchSize     = 50
	auditFlushInterval = 500 * time.Millisecond
	maxDetailLen       = 4096
)

type auditEntry struct {
	UserID     int64
	Username   string
	Action     string
	TargetType string
	TargetID   int64
	TargetName string
	Detail     string
	IPAddress  string
	CreatedAt  time.Time
}

type AuditService struct {
	repo    AuditRepository
	ch      chan *auditEntry
	wg      sync.WaitGroup
	dropped atomic.Int64
	closed  atomic.Bool
}

func NewAuditService(repo AuditRepository) *AuditService {
	s := &AuditService{
		repo: repo,
		ch:   make(chan *auditEntry, auditChannelSize),
	}
	s.wg.Add(1)
	go s.consume()
	return s
}

func (s *AuditService) Record(ctx context.Context, action, targetType string, targetID int64, targetName, detail string) {
	if s.closed.Load() {
		s.dropped.Add(1)
		return
	}

	entry := &auditEntry{
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		TargetName: targetName,
		Detail:     detail,
		CreatedAt:  time.Now(),
	}

	if len(entry.Detail) > maxDetailLen {
		entry.Detail = entry.Detail[:maxDetailLen]
	}

	if v, ok := ctx.Value("userID").(int64); ok {
		entry.UserID = v
	}
	if v, ok := ctx.Value("username").(string); ok {
		entry.Username = v
	}
	if v, ok := ctx.Value("clientIP").(string); ok {
		entry.IPAddress = v
	}

	select {
	case s.ch <- entry:
	default:
		s.dropped.Add(1)
		log.Printf("[audit] channel full, dropped entry: action=%s target=%s", action, targetName)
	}
}

func (s *AuditService) Shutdown() {
	s.closed.Store(true)
	close(s.ch)
	s.wg.Wait()
}

func (s *AuditService) DroppedCount() int64 {
	return s.dropped.Load()
}

func (s *AuditService) consume() {
	defer s.wg.Done()

	batch := make([]model.AuditLog, 0, auditBatchSize)
	ticker := time.NewTicker(auditFlushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.repo.BatchCreate(context.Background(), batch); err != nil {
			log.Printf("[audit] batch write failed (%d entries): %v", len(batch), err)
			s.dropped.Add(int64(len(batch)))
		}
		batch = batch[:0]
	}

	for {
		select {
		case entry, ok := <-s.ch:
			if !ok {
				flush()
				return
			}
			var tid *int64
			if entry.TargetID != 0 {
				tid = &entry.TargetID
			}
			batch = append(batch, model.AuditLog{
				UserID:     entry.UserID,
				Username:   entry.Username,
				Action:     entry.Action,
				TargetType: entry.TargetType,
				TargetID:   tid,
				TargetName: entry.TargetName,
				Detail:     entry.Detail,
				IPAddress:  entry.IPAddress,
				CreatedAt:  entry.CreatedAt,
			})
			if len(batch) >= auditBatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}
