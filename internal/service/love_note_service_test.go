package service

import (
	"context"
	"testing"

	"fambow/internal/repository"
)

type loveNoteStoreSpy struct {
	defaultNotes []string
	addedNote    repository.LoveNote
	addErr       error
	randomResult repository.LoveNote
	randomErr    error
}

func (s *loveNoteStoreSpy) AddDefaultNotes(_ context.Context, notes []string) error {
	s.defaultNotes = append([]string(nil), notes...)
	return nil
}

func (s *loveNoteStoreSpy) AddLoveNote(_ context.Context, note repository.LoveNote) error {
	s.addedNote = note
	return s.addErr
}

func (s *loveNoteStoreSpy) RandomNote(_ context.Context) (repository.LoveNote, error) {
	return s.randomResult, s.randomErr
}

func TestAddLoveNoteTextOnly(t *testing.T) {
	store := &loveNoteStoreSpy{}
	svc := NewLoveNoteService(store)

	if err := svc.AddLoveNote(context.Background(), LoveNoteInput{Text: "  You are sunshine  "}); err != nil {
		t.Fatalf("AddLoveNote() unexpected error: %v", err)
	}

	if store.addedNote.Text != "You are sunshine" {
		t.Fatalf("expected trimmed text, got %#v", store.addedNote)
	}
	if store.addedNote.TelegramFileID != "" {
		t.Fatalf("expected no photo fields, got %#v", store.addedNote)
	}
}

func TestAddLoveNotePhotoOnly(t *testing.T) {
	store := &loveNoteStoreSpy{}
	svc := NewLoveNoteService(store)

	if err := svc.AddLoveNote(context.Background(), LoveNoteInput{
		TelegramFileID:     "photo-id",
		TelegramFileUnique: "photo-uniq",
	}); err != nil {
		t.Fatalf("AddLoveNote() unexpected error: %v", err)
	}

	if store.addedNote.Text != "" {
		t.Fatalf("expected empty text for photo-only note, got %#v", store.addedNote)
	}
	if store.addedNote.TelegramFileID != "photo-id" || store.addedNote.TelegramFileUnique != "photo-uniq" {
		t.Fatalf("unexpected photo fields: %#v", store.addedNote)
	}
}

func TestAddLoveNotePhotoAndText(t *testing.T) {
	store := &loveNoteStoreSpy{}
	svc := NewLoveNoteService(store)

	if err := svc.AddLoveNote(context.Background(), LoveNoteInput{
		Text:               "  Sunset kiss  ",
		TelegramFileID:     "photo-id",
		TelegramFileUnique: "photo-uniq",
	}); err != nil {
		t.Fatalf("AddLoveNote() unexpected error: %v", err)
	}

	if store.addedNote.Text != "Sunset kiss" {
		t.Fatalf("expected trimmed text, got %#v", store.addedNote)
	}
}

func TestAddLoveNoteRejectsEmptyContent(t *testing.T) {
	store := &loveNoteStoreSpy{}
	svc := NewLoveNoteService(store)

	if err := svc.AddLoveNote(context.Background(), LoveNoteInput{}); err != ErrLoveNoteContentEmpty {
		t.Fatalf("expected ErrLoveNoteContentEmpty, got %v", err)
	}
}

func TestRandomLoveNoteReturnsPhotoPayload(t *testing.T) {
	store := &loveNoteStoreSpy{randomResult: repository.LoveNote{
		Text:               "You look beautiful",
		TelegramFileID:     "photo-1",
		TelegramFileUnique: "photo-1-uniq",
	}}
	svc := NewLoveNoteService(store)

	note, err := svc.RandomNote(context.Background(), "Anna")
	if err != nil {
		t.Fatalf("RandomNote() unexpected error: %v", err)
	}

	if note.Text != "You look beautiful" {
		t.Fatalf("unexpected text: %#v", note)
	}
	if note.TelegramFileID != "photo-1" || note.TelegramFileUnique != "photo-1-uniq" {
		t.Fatalf("unexpected photo payload: %#v", note)
	}
}

func TestRandomLoveNoteFormatsTemplateText(t *testing.T) {
	store := &loveNoteStoreSpy{randomResult: repository.LoveNote{Text: "Hey %s, you are magic."}}
	svc := NewLoveNoteService(store)

	note, err := svc.RandomNote(context.Background(), "Anna")
	if err != nil {
		t.Fatalf("RandomNote() unexpected error: %v", err)
	}

	if note.Text != "Hey Anna, you are magic." {
		t.Fatalf("unexpected formatted text: %#v", note)
	}
}
