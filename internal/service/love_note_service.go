package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fambow/internal/repository"
)

var ErrLoveNoteContentEmpty = errors.New("love note content cannot be empty")
var ErrLoveNotesEmpty = errors.New("no love notes available")
var ErrLoveNoteIDsInvalid = errors.New("love note ids are invalid")

type LoveNote struct {
	Text               string
	TelegramFileID     string
	TelegramFileUnique string
}

type AdminLoveNote struct {
	ID        int64
	Text      string
	HasPhoto  bool
	CreatedAt time.Time
}

type LoveNoteInput struct {
	Text               string
	TelegramFileID     string
	TelegramFileUnique string
}

type DeleteLoveNotesResult struct {
	DeletedIDs []int64
	MissingIDs []int64
}

type LoveNoteStore interface {
	AddLoveNote(ctx context.Context, note repository.LoveNote) error
	RandomNote(ctx context.Context) (repository.LoveNote, error)
	NextRandomNoteForUser(ctx context.Context, telegramUserID int64) (repository.LoveNote, error)
	ListLoveNotes(ctx context.Context) ([]repository.LoveNote, error)
	DeleteLoveNotes(ctx context.Context, noteIDs []int64) ([]int64, error)
}

type LoveNoteService struct {
	store LoveNoteStore
}

func NewLoveNoteService(store LoveNoteStore) *LoveNoteService {
	return &LoveNoteService{
		store: store,
	}
}

func (s *LoveNoteService) AddLoveNote(ctx context.Context, input LoveNoteInput) error {
	if s.store == nil {
		return nil
	}

	input.Text = strings.TrimSpace(input.Text)
	input.TelegramFileID = strings.TrimSpace(input.TelegramFileID)
	input.TelegramFileUnique = strings.TrimSpace(input.TelegramFileUnique)
	if input.Text == "" && input.TelegramFileID == "" {
		return ErrLoveNoteContentEmpty
	}

	return s.store.AddLoveNote(ctx, repository.LoveNote{
		Text:               input.Text,
		TelegramFileID:     input.TelegramFileID,
		TelegramFileUnique: input.TelegramFileUnique,
	})
}

func (s *LoveNoteService) RandomNote(ctx context.Context, firstName string) (LoveNote, error) {
	if s.store == nil {
		return LoveNote{}, ErrLoveNotesEmpty
	}

	name := strings.TrimSpace(firstName)
	if name == "" {
		name = "my love"
	}

	storedNote, err := s.store.RandomNote(ctx)
	if err != nil {
		return LoveNote{}, err
	}

	note := LoveNote{
		Text:               strings.TrimSpace(storedNote.Text),
		TelegramFileID:     strings.TrimSpace(storedNote.TelegramFileID),
		TelegramFileUnique: strings.TrimSpace(storedNote.TelegramFileUnique),
	}

	if note.Text == "" && note.TelegramFileID == "" {
		return LoveNote{}, ErrLoveNotesEmpty
	}

	note.Text = formatLoveNoteText(note.Text, name)
	return note, nil
}

func (s *LoveNoteService) NextNoteForUser(ctx context.Context, telegramUserID int64, firstName string) (LoveNote, error) {
	if s.store == nil {
		return LoveNote{}, ErrLoveNotesEmpty
	}

	name := strings.TrimSpace(firstName)
	if name == "" {
		name = "my love"
	}

	storedNote, err := s.store.NextRandomNoteForUser(ctx, telegramUserID)
	if err != nil {
		return LoveNote{}, err
	}

	note := LoveNote{
		Text:               strings.TrimSpace(storedNote.Text),
		TelegramFileID:     strings.TrimSpace(storedNote.TelegramFileID),
		TelegramFileUnique: strings.TrimSpace(storedNote.TelegramFileUnique),
	}

	if note.Text == "" && note.TelegramFileID == "" {
		return LoveNote{}, ErrLoveNotesEmpty
	}

	note.Text = formatLoveNoteText(note.Text, name)
	return note, nil
}

func (s *LoveNoteService) ListLoveNotes(ctx context.Context) ([]AdminLoveNote, error) {
	if s.store == nil {
		return nil, nil
	}

	records, err := s.store.ListLoveNotes(ctx)
	if err != nil {
		return nil, err
	}

	notes := make([]AdminLoveNote, 0, len(records))
	for _, record := range records {
		notes = append(notes, AdminLoveNote{
			ID:        record.ID,
			Text:      strings.TrimSpace(record.Text),
			HasPhoto:  strings.TrimSpace(record.TelegramFileID) != "",
			CreatedAt: record.CreatedAt,
		})
	}

	return notes, nil
}

func (s *LoveNoteService) DeleteLoveNotes(ctx context.Context, noteIDs []int64) (DeleteLoveNotesResult, error) {
	normalizedIDs, err := normalizeLoveNoteIDs(noteIDs)
	if err != nil {
		return DeleteLoveNotesResult{}, err
	}

	if s.store == nil {
		return DeleteLoveNotesResult{MissingIDs: normalizedIDs}, nil
	}

	deletedIDs, err := s.store.DeleteLoveNotes(ctx, normalizedIDs)
	if err != nil {
		return DeleteLoveNotesResult{}, err
	}

	deletedSet := make(map[int64]struct{}, len(deletedIDs))
	for _, id := range deletedIDs {
		deletedSet[id] = struct{}{}
	}

	result := DeleteLoveNotesResult{
		DeletedIDs: make([]int64, 0, len(deletedIDs)),
		MissingIDs: make([]int64, 0),
	}
	for _, id := range normalizedIDs {
		if _, ok := deletedSet[id]; ok {
			result.DeletedIDs = append(result.DeletedIDs, id)
			continue
		}
		result.MissingIDs = append(result.MissingIDs, id)
	}

	return result, nil
}

func formatLoveNoteText(template string, name string) string {
	if template == "" {
		return ""
	}

	if strings.Contains(template, "%s") {
		return fmt.Sprintf(template, name)
	}

	return template
}

func normalizeLoveNoteIDs(noteIDs []int64) ([]int64, error) {
	if len(noteIDs) == 0 {
		return nil, ErrLoveNoteIDsInvalid
	}

	seen := make(map[int64]struct{}, len(noteIDs))
	normalized := make([]int64, 0, len(noteIDs))
	for _, id := range noteIDs {
		if id <= 0 {
			return nil, ErrLoveNoteIDsInvalid
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}

	return normalized, nil
}
