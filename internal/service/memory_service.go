package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"fambow/internal/repository"
)

var ErrMemoryTextEmpty = errors.New("memory text cannot be empty")
var ErrMemoryContentEmpty = errors.New("memory content cannot be empty")
var ErrMemoryDateFormat = errors.New("invalid memory date format")
var ErrMemoryDateInFuture = errors.New("memory date cannot be in the future")
var ErrMemoryNotFound = errors.New("memory not found")

const memoryDateLayout = "2006-01-02"

type Memory struct {
	Text               string
	CreatedAt          time.Time
	TelegramFileID     string
	TelegramFileUnique string
}

type MemoryInput struct {
	Text               string
	TelegramFileID     string
	TelegramFileUnique string
	CreatedAt          *time.Time
}

type MemoryStore interface {
	SaveMemory(ctx context.Context, telegramUserID int64, firstName string, text string, telegramFileID string, telegramFileUnique string, createdAt *time.Time) (repository.Memory, error)
	ListRecentMemories(ctx context.Context, telegramUserID int64, limit int) ([]repository.Memory, error)
	RandomMemory(ctx context.Context) (repository.Memory, error)
}

type MemoryService struct {
	store MemoryStore
}

func NewMemoryService(store MemoryStore) *MemoryService {
	return &MemoryService{store: store}
}

func (s *MemoryService) AddMemory(ctx context.Context, telegramUserID int64, firstName string, input MemoryInput) (Memory, error) {
	input.Text = strings.TrimSpace(input.Text)
	input.TelegramFileID = strings.TrimSpace(input.TelegramFileID)
	input.TelegramFileUnique = strings.TrimSpace(input.TelegramFileUnique)

	parsedText, customDate, err := parseMemoryPayload(input.Text, time.Now())
	if err != nil {
		return Memory{}, err
	}
	input.Text = parsedText
	input.CreatedAt = customDate

	if input.Text == "" && input.TelegramFileID == "" {
		return Memory{}, ErrMemoryContentEmpty
	}

	record, err := s.store.SaveMemory(ctx, telegramUserID, strings.TrimSpace(firstName), input.Text, input.TelegramFileID, input.TelegramFileUnique, input.CreatedAt)
	if err != nil {
		return Memory{}, err
	}

	return Memory{
		Text:               record.Text,
		CreatedAt:          record.CreatedAt,
		TelegramFileID:     record.TelegramFileID,
		TelegramFileUnique: record.TelegramFileUnique,
	}, nil
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
		memories = append(memories, Memory{
			Text:               record.Text,
			CreatedAt:          record.CreatedAt,
			TelegramFileID:     record.TelegramFileID,
			TelegramFileUnique: record.TelegramFileUnique,
		})
	}

	return memories, nil
}

func (s *MemoryService) RandomMemory(ctx context.Context) (Memory, error) {
	record, err := s.store.RandomMemory(ctx)
	if err != nil {
		if errors.Is(err, repository.ErrMemoryNotFound) {
			return Memory{}, ErrMemoryNotFound
		}
		return Memory{}, err
	}

	return Memory{
		Text:               record.Text,
		CreatedAt:          record.CreatedAt,
		TelegramFileID:     record.TelegramFileID,
		TelegramFileUnique: record.TelegramFileUnique,
	}, nil
}

func parseMemoryPayload(payload string, now time.Time) (string, *time.Time, error) {
	trimmed := strings.TrimSpace(payload)
	if trimmed == "" {
		return "", nil, nil
	}

	datePart, textPart, hasSeparator := strings.Cut(trimmed, "|")
	if !hasSeparator {
		return trimmed, nil, nil
	}

	datePart = strings.TrimSpace(datePart)
	if !isMemoryDateCandidate(datePart) {
		return trimmed, nil, nil
	}

	parsedDate, err := time.ParseInLocation(memoryDateLayout, datePart, now.Location())
	if err != nil {
		return "", nil, ErrMemoryDateFormat
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if parsedDate.After(today) {
		return "", nil, ErrMemoryDateInFuture
	}

	customDate := time.Date(parsedDate.Year(), parsedDate.Month(), parsedDate.Day(), 0, 0, 0, 0, now.Location())
	return strings.TrimSpace(textPart), &customDate, nil
}

func isMemoryDateCandidate(value string) bool {
	if len(value) != len(memoryDateLayout) {
		return false
	}

	for i := 0; i < len(value); i++ {
		switch i {
		case 4, 7:
			if value[i] != '-' {
				return false
			}
		default:
			if value[i] < '0' || value[i] > '9' {
				return false
			}
		}
	}

	return true
}

func MemoryUsage() string {
	lines := []string{
		"Use one of these formats:",
		"/memory We watched the sunset together",
		"/memory 2020-12-24 | Cozy holiday walk",
		"Photo caption: /memory 2020-12-24 | Optional note",
		"Tip: custom date cannot be in the future.",
	}

	return strings.Join(lines, "\n")
}
