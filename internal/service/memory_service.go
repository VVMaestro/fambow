package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"fambow/internal/repository"
)

var ErrMemoryTextEmpty = errors.New("memory text cannot be empty")

type Memory struct {
	Text      string
	CreatedAt time.Time
}

type MemoryStore interface {
	SaveMemory(ctx context.Context, telegramUserID int64, firstName string, text string) (repository.Memory, error)
	ListRecentMemories(ctx context.Context, telegramUserID int64, limit int) ([]repository.Memory, error)
}

type MemoryService struct {
	store MemoryStore
}

func NewMemoryService(store MemoryStore) *MemoryService {
	return &MemoryService{store: store}
}

func (s *MemoryService) AddMemory(ctx context.Context, telegramUserID int64, firstName string, text string) (Memory, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return Memory{}, ErrMemoryTextEmpty
	}

	record, err := s.store.SaveMemory(ctx, telegramUserID, strings.TrimSpace(firstName), trimmed)
	if err != nil {
		return Memory{}, err
	}

	return Memory{Text: record.Text, CreatedAt: record.CreatedAt}, nil
}

func (s *MemoryService) RecentMemories(ctx context.Context, telegramUserID int64, limit int) ([]Memory, error) {
	if limit <= 0 {
		limit = 5
	}

	records, err := s.store.ListRecentMemories(ctx, telegramUserID, limit)
	if err != nil {
		return nil, err
	}

	memories := make([]Memory, 0, len(records))
	for _, record := range records {
		memories = append(memories, Memory{Text: record.Text, CreatedAt: record.CreatedAt})
	}

	return memories, nil
}
