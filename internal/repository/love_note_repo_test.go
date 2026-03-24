package repository

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestLoveNoteRepositoryRandomNoteSupportsLegacyTextRows(t *testing.T) {
	ctx := context.Background()
	db := openLoveNoteTestDB(t)
	repo := NewLoveNoteRepository(db)

	if _, err := db.ExecContext(ctx, `
		INSERT INTO love_notes (text, tag)
		VALUES (?, ?)
	`, "Text only note", "default"); err != nil {
		t.Fatalf("insert legacy love note: %v", err)
	}

	note, err := repo.RandomNote(ctx)
	if err != nil {
		t.Fatalf("RandomNote() unexpected error: %v", err)
	}
	if note.Text != "Text only note" {
		t.Fatalf("unexpected text-only note: %#v", note)
	}
	if note.TelegramFileID != "" || note.TelegramFileUnique != "" {
		t.Fatalf("expected empty photo fields for legacy row, got %#v", note)
	}
}

func TestLoveNoteRepositoryRoundTripsPhotoNote(t *testing.T) {
	ctx := context.Background()
	db := openLoveNoteTestDB(t)
	repo := NewLoveNoteRepository(db)

	if err := repo.AddLoveNote(ctx, LoveNote{
		Text:               "Photo note",
		TelegramFileID:     "photo-id",
		TelegramFileUnique: "photo-uniq",
	}); err != nil {
		t.Fatalf("AddLoveNote() unexpected error: %v", err)
	}

	note, err := repo.RandomNote(ctx)
	if err != nil {
		t.Fatalf("RandomNote() unexpected error: %v", err)
	}
	if note.Text != "Photo note" {
		t.Fatalf("unexpected photo note text: %#v", note)
	}
	if note.TelegramFileID != "photo-id" || note.TelegramFileUnique != "photo-uniq" {
		t.Fatalf("unexpected photo note fields: %#v", note)
	}
}

func TestLoveNoteRepositoryListLoveNotesReturnsNewestFirst(t *testing.T) {
	ctx := context.Background()
	db := openLoveNoteTestDB(t)
	repo := NewLoveNoteRepository(db)

	if err := repo.AddLoveNote(ctx, LoveNote{Text: "First note"}); err != nil {
		t.Fatalf("AddLoveNote(first) unexpected error: %v", err)
	}
	if err := repo.AddLoveNote(ctx, LoveNote{Text: "Second note", TelegramFileID: "photo-id", TelegramFileUnique: "photo-uniq"}); err != nil {
		t.Fatalf("AddLoveNote(second) unexpected error: %v", err)
	}

	notes, err := repo.ListLoveNotes(ctx)
	if err != nil {
		t.Fatalf("ListLoveNotes() unexpected error: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
	if notes[0].Text != "Second note" || notes[1].Text != "First note" {
		t.Fatalf("expected newest first, got %#v", notes)
	}
	if notes[0].TelegramFileID != "photo-id" || notes[1].TelegramFileID != "" {
		t.Fatalf("unexpected list photo fields: %#v", notes)
	}
}

func TestLoveNoteRepositoryDeleteLoveNotesDeletesExistingOnly(t *testing.T) {
	ctx := context.Background()
	db := openLoveNoteTestDB(t)
	repo := NewLoveNoteRepository(db)

	if err := repo.AddLoveNote(ctx, LoveNote{Text: "Keep me"}); err != nil {
		t.Fatalf("AddLoveNote(keep) unexpected error: %v", err)
	}
	if err := repo.AddLoveNote(ctx, LoveNote{Text: "Delete me"}); err != nil {
		t.Fatalf("AddLoveNote(delete) unexpected error: %v", err)
	}
	if err := repo.AddLoveNote(ctx, LoveNote{TelegramFileID: "photo-id", TelegramFileUnique: "photo-uniq"}); err != nil {
		t.Fatalf("AddLoveNote(photo only) unexpected error: %v", err)
	}

	notesBefore, err := repo.ListLoveNotes(ctx)
	if err != nil {
		t.Fatalf("ListLoveNotes(before) unexpected error: %v", err)
	}
	deleteIDs := []int64{notesBefore[0].ID, notesBefore[1].ID, 999}

	deletedIDs, err := repo.DeleteLoveNotes(ctx, deleteIDs)
	if err != nil {
		t.Fatalf("DeleteLoveNotes() unexpected error: %v", err)
	}
	if len(deletedIDs) != 2 {
		t.Fatalf("expected 2 deleted ids, got %#v", deletedIDs)
	}

	notesAfter, err := repo.ListLoveNotes(ctx)
	if err != nil {
		t.Fatalf("ListLoveNotes(after) unexpected error: %v", err)
	}
	if len(notesAfter) != 1 || notesAfter[0].Text != "Keep me" {
		t.Fatalf("unexpected notes after delete: %#v", notesAfter)
	}
}

func openLoveNoteTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite(): %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	migrationsDir := filepath.Join("..", "..", "migrations")
	if err := RunMigrations(context.Background(), db, migrationsDir); err != nil {
		t.Fatalf("RunMigrations(): %v", err)
	}

	return db
}
