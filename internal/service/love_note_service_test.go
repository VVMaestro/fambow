package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"fambow/internal/repository"
)

type loveNoteStoreSpy struct {
	addedNote    repository.LoveNote
	addErr       error
	randomResult repository.LoveNote
	randomErr    error
	listResult   []repository.LoveNote
	listErr      error
	deleteIDs    []int64
	deleteResult []int64
	deleteErr    error
}

func (s *loveNoteStoreSpy) AddLoveNote(_ context.Context, note repository.LoveNote) error {
	s.addedNote = note
	return s.addErr
}

func (s *loveNoteStoreSpy) RandomNote(_ context.Context) (repository.LoveNote, error) {
	return s.randomResult, s.randomErr
}

func (s *loveNoteStoreSpy) ListLoveNotes(context.Context) ([]repository.LoveNote, error) {
	return s.listResult, s.listErr
}

func (s *loveNoteStoreSpy) DeleteLoveNotes(_ context.Context, noteIDs []int64) ([]int64, error) {
	s.deleteIDs = append([]int64(nil), noteIDs...)
	return s.deleteResult, s.deleteErr
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

func TestRandomLoveNoteReturnsEmptyErrorWhenStoreHasNoNotes(t *testing.T) {
	store := &loveNoteStoreSpy{}
	svc := NewLoveNoteService(store)

	_, err := svc.RandomNote(context.Background(), "Anna")
	if err != ErrLoveNotesEmpty {
		t.Fatalf("expected ErrLoveNotesEmpty, got %v", err)
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

func TestListLoveNotesMapsAdminFields(t *testing.T) {
	store := &loveNoteStoreSpy{
		listResult: []repository.LoveNote{
			{
				ID:             5,
				Text:           "  Sunset walk  ",
				TelegramFileID: "photo-1",
				CreatedAt:      time.Date(2026, time.March, 20, 18, 15, 0, 0, time.Local),
			},
		},
	}
	svc := NewLoveNoteService(store)

	notes, err := svc.ListLoveNotes(context.Background())
	if err != nil {
		t.Fatalf("ListLoveNotes() unexpected error: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	if notes[0].ID != 5 || notes[0].Text != "Sunset walk" || !notes[0].HasPhoto {
		t.Fatalf("unexpected admin love note: %#v", notes[0])
	}
}

func TestDeleteLoveNotesDeduplicatesAndTracksMissing(t *testing.T) {
	store := &loveNoteStoreSpy{deleteResult: []int64{2, 7}}
	svc := NewLoveNoteService(store)

	result, err := svc.DeleteLoveNotes(context.Background(), []int64{2, 7, 2, 10})
	if err != nil {
		t.Fatalf("DeleteLoveNotes() unexpected error: %v", err)
	}
	if len(store.deleteIDs) != 3 || store.deleteIDs[0] != 2 || store.deleteIDs[1] != 7 || store.deleteIDs[2] != 10 {
		t.Fatalf("unexpected normalized delete ids: %#v", store.deleteIDs)
	}
	if len(result.DeletedIDs) != 2 || result.DeletedIDs[0] != 2 || result.DeletedIDs[1] != 7 {
		t.Fatalf("unexpected deleted ids: %#v", result.DeletedIDs)
	}
	if len(result.MissingIDs) != 1 || result.MissingIDs[0] != 10 {
		t.Fatalf("unexpected missing ids: %#v", result.MissingIDs)
	}
}

func TestDeleteLoveNotesRejectsInvalidInput(t *testing.T) {
	svc := NewLoveNoteService(&loveNoteStoreSpy{})

	tests := [][]int64{
		nil,
		{},
		{1, 0},
		{-1},
	}

	for _, noteIDs := range tests {
		_, err := svc.DeleteLoveNotes(context.Background(), noteIDs)
		if !errors.Is(err, ErrLoveNoteIDsInvalid) {
			t.Fatalf("expected ErrLoveNoteIDsInvalid for %#v, got %v", noteIDs, err)
		}
	}
}
