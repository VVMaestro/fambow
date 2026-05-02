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

func TestLoveNoteRepositoryNextRandomNoteForUserReturnsUniqueUntilExhausted(t *testing.T) {
	ctx := context.Background()
	db := openLoveNoteTestDB(t)
	repo := NewLoveNoteRepository(db)
	telegramUserID := seedUser(t, db, 101, "Anna", UserTypeWife)

	for _, text := range []string{"First", "Second", "Third"} {
		if err := repo.AddLoveNote(ctx, LoveNote{Text: text}); err != nil {
			t.Fatalf("AddLoveNote(%q) unexpected error: %v", text, err)
		}
	}

	seen := make(map[int64]struct{})
	for range 3 {
		note, err := repo.NextRandomNoteForUser(ctx, telegramUserID)
		if err != nil {
			t.Fatalf("NextRandomNoteForUser() unexpected error: %v", err)
		}
		if _, exists := seen[note.ID]; exists {
			t.Fatalf("received duplicate note before exhaustion: %#v", note)
		}
		seen[note.ID] = struct{}{}
	}

	if len(seen) != 3 {
		t.Fatalf("expected 3 unique notes, got %d", len(seen))
	}

	var cycleCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM love_note_user_cycle WHERE user_id = (SELECT id FROM users WHERE telegram_user_id = ?)`, telegramUserID).Scan(&cycleCount); err != nil {
		t.Fatalf("count love note cycle rows: %v", err)
	}
	if cycleCount != 3 {
		t.Fatalf("expected 3 cycle rows after exhaustion, got %d", cycleCount)
	}
}

func TestLoveNoteRepositoryNextRandomNoteForUserResetsAfterExhaustion(t *testing.T) {
	ctx := context.Background()
	db := openLoveNoteTestDB(t)
	repo := NewLoveNoteRepository(db)
	telegramUserID := seedUser(t, db, 101, "Anna", UserTypeWife)

	for _, text := range []string{"First", "Second"} {
		if err := repo.AddLoveNote(ctx, LoveNote{Text: text}); err != nil {
			t.Fatalf("AddLoveNote(%q) unexpected error: %v", text, err)
		}
	}

	firstCycle := make(map[int64]struct{})
	for range 2 {
		note, err := repo.NextRandomNoteForUser(ctx, telegramUserID)
		if err != nil {
			t.Fatalf("NextRandomNoteForUser() unexpected error: %v", err)
		}
		firstCycle[note.ID] = struct{}{}
	}

	note, err := repo.NextRandomNoteForUser(ctx, telegramUserID)
	if err != nil {
		t.Fatalf("NextRandomNoteForUser(reset) unexpected error: %v", err)
	}
	if _, ok := firstCycle[note.ID]; !ok {
		t.Fatalf("expected reset note from existing pool, got %#v", note)
	}

	var cycleCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM love_note_user_cycle WHERE user_id = (SELECT id FROM users WHERE telegram_user_id = ?)`, telegramUserID).Scan(&cycleCount); err != nil {
		t.Fatalf("count love note cycle rows after reset: %v", err)
	}
	if cycleCount != 1 {
		t.Fatalf("expected cycle to restart with 1 row, got %d", cycleCount)
	}
}

func TestLoveNoteRepositoryNextRandomNoteForUserIncludesNewItemsMidCycle(t *testing.T) {
	ctx := context.Background()
	db := openLoveNoteTestDB(t)
	repo := NewLoveNoteRepository(db)
	telegramUserID := seedUser(t, db, 101, "Anna", UserTypeWife)

	for _, text := range []string{"First", "Second"} {
		if err := repo.AddLoveNote(ctx, LoveNote{Text: text}); err != nil {
			t.Fatalf("AddLoveNote(%q) unexpected error: %v", text, err)
		}
	}

	first, err := repo.NextRandomNoteForUser(ctx, telegramUserID)
	if err != nil {
		t.Fatalf("NextRandomNoteForUser(first) unexpected error: %v", err)
	}

	if err := repo.AddLoveNote(ctx, LoveNote{Text: "Third"}); err != nil {
		t.Fatalf("AddLoveNote(third) unexpected error: %v", err)
	}

	second, err := repo.NextRandomNoteForUser(ctx, telegramUserID)
	if err != nil {
		t.Fatalf("NextRandomNoteForUser(second) unexpected error: %v", err)
	}
	third, err := repo.NextRandomNoteForUser(ctx, telegramUserID)
	if err != nil {
		t.Fatalf("NextRandomNoteForUser(third) unexpected error: %v", err)
	}

	seen := map[int64]struct{}{
		first.ID:  {},
		second.ID: {},
		third.ID:  {},
	}
	if len(seen) != 3 {
		t.Fatalf("expected newly added note to appear before reset, got ids %#v", seen)
	}
}

func TestLoveNoteRepositoryDeleteLoveNotesRemovesCycleRows(t *testing.T) {
	ctx := context.Background()
	db := openLoveNoteTestDB(t)
	repo := NewLoveNoteRepository(db)
	telegramUserID := seedUser(t, db, 101, "Anna", UserTypeWife)

	if err := repo.AddLoveNote(ctx, LoveNote{Text: "Keep me"}); err != nil {
		t.Fatalf("AddLoveNote(keep) unexpected error: %v", err)
	}
	if err := repo.AddLoveNote(ctx, LoveNote{Text: "Delete me"}); err != nil {
		t.Fatalf("AddLoveNote(delete) unexpected error: %v", err)
	}

	first, err := repo.NextRandomNoteForUser(ctx, telegramUserID)
	if err != nil {
		t.Fatalf("NextRandomNoteForUser(first) unexpected error: %v", err)
	}
	second, err := repo.NextRandomNoteForUser(ctx, telegramUserID)
	if err != nil {
		t.Fatalf("NextRandomNoteForUser(second) unexpected error: %v", err)
	}

	if _, err := repo.DeleteLoveNotes(ctx, []int64{first.ID}); err != nil {
		t.Fatalf("DeleteLoveNotes() unexpected error: %v", err)
	}

	var deletedCycleCount int
	if err := db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM love_note_user_cycle
		WHERE love_note_id = ?
	`, first.ID).Scan(&deletedCycleCount); err != nil {
		t.Fatalf("count deleted note cycle rows: %v", err)
	}
	if deletedCycleCount != 0 {
		t.Fatalf("expected deleted note cycle rows to be removed, got %d", deletedCycleCount)
	}

	next, err := repo.NextRandomNoteForUser(ctx, telegramUserID)
	if err != nil {
		t.Fatalf("NextRandomNoteForUser(after delete) unexpected error: %v", err)
	}
	if next.ID != second.ID {
		t.Fatalf("expected remaining note after delete, got %#v (second %#v)", next, second)
	}
}

func TestLoveNoteRepositoryNextRandomNoteForUserKeepsCyclesIndependent(t *testing.T) {
	ctx := context.Background()
	db := openLoveNoteTestDB(t)
	repo := NewLoveNoteRepository(db)
	firstUser := seedUser(t, db, 101, "Anna", UserTypeWife)
	secondUser := seedUser(t, db, 202, "Mia", UserTypeWife)

	for _, text := range []string{"First", "Second"} {
		if err := repo.AddLoveNote(ctx, LoveNote{Text: text}); err != nil {
			t.Fatalf("AddLoveNote(%q) unexpected error: %v", text, err)
		}
	}

	firstNote, err := repo.NextRandomNoteForUser(ctx, firstUser)
	if err != nil {
		t.Fatalf("NextRandomNoteForUser(first user) unexpected error: %v", err)
	}
	secondNote, err := repo.NextRandomNoteForUser(ctx, secondUser)
	if err != nil {
		t.Fatalf("NextRandomNoteForUser(second user) unexpected error: %v", err)
	}

	var firstCycleCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM love_note_user_cycle WHERE user_id = (SELECT id FROM users WHERE telegram_user_id = ?)`, firstUser).Scan(&firstCycleCount); err != nil {
		t.Fatalf("count first user cycle rows: %v", err)
	}
	var secondCycleCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM love_note_user_cycle WHERE user_id = (SELECT id FROM users WHERE telegram_user_id = ?)`, secondUser).Scan(&secondCycleCount); err != nil {
		t.Fatalf("count second user cycle rows: %v", err)
	}
	if firstCycleCount != 1 || secondCycleCount != 1 {
		t.Fatalf("expected independent cycles with 1 row each, got first=%d second=%d", firstCycleCount, secondCycleCount)
	}
	if firstNote.ID == 0 || secondNote.ID == 0 {
		t.Fatalf("expected both users to receive notes, got first=%#v second=%#v", firstNote, secondNote)
	}
}

func seedUser(t *testing.T, db *sql.DB, telegramUserID int64, firstName string, userType string) int64 {
	t.Helper()

	if _, err := db.ExecContext(context.Background(), `
		INSERT INTO users (telegram_user_id, first_name, type)
		VALUES (?, ?, ?)
	`, telegramUserID, firstName, userType); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	return telegramUserID
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
