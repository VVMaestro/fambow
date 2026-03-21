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
