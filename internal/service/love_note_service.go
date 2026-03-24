package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"fambow/internal/repository"
)

var ErrLoveNoteContentEmpty = errors.New("love note content cannot be empty")
var ErrLoveNotesEmpty = errors.New("no love notes available")

type LoveNote struct {
	Text               string
	TelegramFileID     string
	TelegramFileUnique string
}

type LoveNoteInput struct {
	Text               string
	TelegramFileID     string
	TelegramFileUnique string
}

type LoveNoteStore interface {
	AddLoveNote(ctx context.Context, note repository.LoveNote) error
	RandomNote(ctx context.Context) (repository.LoveNote, error)
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

func formatLoveNoteText(template string, name string) string {
	if template == "" {
		return ""
	}

	if strings.Contains(template, "%s") {
		return fmt.Sprintf(template, name)
	}

	return template
}
