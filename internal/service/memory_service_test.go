package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"fambow/internal/repository"
)

type memoryStoreSpy struct {
	saveCalled bool
	saveInput  memorySaveInput

	result repository.Memory
	err    error
}

type memorySaveInput struct {
	telegramUserID int64
	firstName      string
	text           string
	fileID         string
	fileUnique     string
	createdAt      *time.Time
}

func (s *memoryStoreSpy) SaveMemory(ctx context.Context, telegramUserID int64, firstName string, text string, telegramFileID string, telegramFileUnique string, createdAt *time.Time) (repository.Memory, error) {
	_ = ctx
	s.saveCalled = true
	s.saveInput.telegramUserID = telegramUserID
	s.saveInput.firstName = firstName
	s.saveInput.text = text
	s.saveInput.fileID = telegramFileID
	s.saveInput.fileUnique = telegramFileUnique
	if createdAt != nil {
		copied := *createdAt
		s.saveInput.createdAt = &copied
	}

	if s.err != nil {
		return repository.Memory{}, s.err
	}

	record := s.result
	if record.Text == "" {
		record.Text = text
	}
	if record.TelegramFileID == "" {
		record.TelegramFileID = telegramFileID
	}
	if record.TelegramFileUnique == "" {
		record.TelegramFileUnique = telegramFileUnique
	}
	if record.CreatedAt.IsZero() {
		if createdAt != nil {
			record.CreatedAt = *createdAt
		} else {
			record.CreatedAt = time.Now()
		}
	}

	return record, nil
}

func (s *memoryStoreSpy) ListRecentMemories(ctx context.Context, telegramUserID int64, limit int) ([]repository.Memory, error) {
	_ = ctx
	_ = telegramUserID
	_ = limit
	return nil, nil
}

func TestAddMemoryWithoutCustomDate(t *testing.T) {
	store := &memoryStoreSpy{}
	svc := NewMemoryService(store)

	_, err := svc.AddMemory(context.Background(), 101, "Anna", MemoryInput{Text: "Our first hike"})
	if err != nil {
		t.Fatalf("AddMemory() unexpected error: %v", err)
	}

	if !store.saveCalled {
		t.Fatal("SaveMemory was not called")
	}
	if store.saveInput.text != "Our first hike" {
		t.Fatalf("SaveMemory text mismatch: got %q", store.saveInput.text)
	}
	if store.saveInput.createdAt != nil {
		t.Fatal("expected nil custom date for plain text memory")
	}
}

func TestAddMemoryWithCustomDate(t *testing.T) {
	store := &memoryStoreSpy{}
	svc := NewMemoryService(store)

	_, err := svc.AddMemory(context.Background(), 101, "Anna", MemoryInput{Text: "2020-06-12 | Our first trip"})
	if err != nil {
		t.Fatalf("AddMemory() unexpected error: %v", err)
	}

	if !store.saveCalled {
		t.Fatal("SaveMemory was not called")
	}
	if store.saveInput.text != "Our first trip" {
		t.Fatalf("SaveMemory text mismatch: got %q", store.saveInput.text)
	}
	if store.saveInput.createdAt == nil {
		t.Fatal("expected custom date to be passed to SaveMemory")
	}
	if got := store.saveInput.createdAt.Format(memoryDateLayout); got != "2020-06-12" {
		t.Fatalf("custom date mismatch: got %q", got)
	}
	if store.saveInput.createdAt.Hour() != 0 || store.saveInput.createdAt.Minute() != 0 || store.saveInput.createdAt.Second() != 0 {
		t.Fatalf("expected normalized midnight datetime, got %s", store.saveInput.createdAt.Format(time.RFC3339))
	}
}

func TestAddMemoryRejectsInvalidCustomDate(t *testing.T) {
	store := &memoryStoreSpy{}
	svc := NewMemoryService(store)

	_, err := svc.AddMemory(context.Background(), 101, "Anna", MemoryInput{Text: "2020-99-12 | Broken date"})
	if !errors.Is(err, ErrMemoryDateFormat) {
		t.Fatalf("expected ErrMemoryDateFormat, got %v", err)
	}
	if store.saveCalled {
		t.Fatal("SaveMemory should not be called on invalid date")
	}
}

func TestAddMemoryRejectsFutureCustomDate(t *testing.T) {
	store := &memoryStoreSpy{}
	svc := NewMemoryService(store)

	futureDate := time.Now().AddDate(0, 0, 1).Format(memoryDateLayout)
	_, err := svc.AddMemory(context.Background(), 101, "Anna", MemoryInput{Text: futureDate + " | Not yet happened"})
	if !errors.Is(err, ErrMemoryDateInFuture) {
		t.Fatalf("expected ErrMemoryDateInFuture, got %v", err)
	}
	if store.saveCalled {
		t.Fatal("SaveMemory should not be called on future date")
	}
}

func TestAddMemoryPhotoOnlyWithCustomDate(t *testing.T) {
	store := &memoryStoreSpy{}
	svc := NewMemoryService(store)

	_, err := svc.AddMemory(context.Background(), 101, "Anna", MemoryInput{
		Text:           "2020-06-12 |   ",
		TelegramFileID: "photo-file-id",
	})
	if err != nil {
		t.Fatalf("AddMemory() unexpected error: %v", err)
	}

	if !store.saveCalled {
		t.Fatal("SaveMemory was not called")
	}
	if store.saveInput.text != "" {
		t.Fatalf("expected empty text for photo-only memory, got %q", store.saveInput.text)
	}
	if store.saveInput.createdAt == nil {
		t.Fatal("expected custom date to be passed to SaveMemory")
	}
}

func TestAddMemoryKeepsPipeWhenNotDatePrefix(t *testing.T) {
	store := &memoryStoreSpy{}
	svc := NewMemoryService(store)

	original := "Cafe visit | best cappuccino"
	_, err := svc.AddMemory(context.Background(), 101, "Anna", MemoryInput{Text: original})
	if err != nil {
		t.Fatalf("AddMemory() unexpected error: %v", err)
	}

	if store.saveInput.text != original {
		t.Fatalf("expected original text preserved, got %q", store.saveInput.text)
	}
	if store.saveInput.createdAt != nil {
		t.Fatal("expected nil custom date when prefix is not date")
	}
}
