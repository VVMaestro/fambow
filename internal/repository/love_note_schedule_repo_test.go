package repository

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestLoveNoteScheduleRepositoryLifecycle(t *testing.T) {
	ctx := context.Background()
	db := openLoveNoteScheduleTestDB(t)
	repo := NewLoveNoteScheduleRepository(db)

	createLoveNoteScheduleTestUser(t, ctx, db, 101, "Anna", UserTypeWife)
	createLoveNoteScheduleTestUser(t, ctx, db, 202, "Misha", UserTypeHusband)

	first, err := repo.SaveLoveNoteSchedule(ctx, 101, "08:30")
	if err != nil {
		t.Fatalf("SaveLoveNoteSchedule() first unexpected error: %v", err)
	}
	second, err := repo.SaveLoveNoteSchedule(ctx, 101, "08:30")
	if err != nil {
		t.Fatalf("SaveLoveNoteSchedule() second unexpected error: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("expected distinct schedules, got same id %d", first.ID)
	}

	items, err := repo.ListActiveLoveNoteSchedules(ctx)
	if err != nil {
		t.Fatalf("ListActiveLoveNoteSchedules() unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 active schedules, got %d", len(items))
	}

	due, err := repo.ListDueLoveNoteSchedules(ctx, "08:30", "2026-03-22")
	if err != nil {
		t.Fatalf("ListDueLoveNoteSchedules() unexpected error: %v", err)
	}
	if len(due) != 2 {
		t.Fatalf("expected 2 due schedules, got %d", len(due))
	}

	if err := repo.MarkLoveNoteScheduleDispatched(ctx, first.ID, "2026-03-22"); err != nil {
		t.Fatalf("MarkLoveNoteScheduleDispatched() unexpected error: %v", err)
	}
	if err := repo.MarkLoveNoteScheduleDispatched(ctx, first.ID, "2026-03-22"); err != nil {
		t.Fatalf("MarkLoveNoteScheduleDispatched() second unexpected error: %v", err)
	}

	due, err = repo.ListDueLoveNoteSchedules(ctx, "08:30", "2026-03-22")
	if err != nil {
		t.Fatalf("ListDueLoveNoteSchedules() after dispatch unexpected error: %v", err)
	}
	if len(due) != 1 || due[0].ID != second.ID {
		t.Fatalf("unexpected due schedules after dispatch: %#v", due)
	}

	if err := repo.DeactivateLoveNoteSchedule(ctx, second.ID); err != nil {
		t.Fatalf("DeactivateLoveNoteSchedule() unexpected error: %v", err)
	}

	items, err = repo.ListActiveLoveNoteSchedules(ctx)
	if err != nil {
		t.Fatalf("ListActiveLoveNoteSchedules() after deactivate unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].ID != first.ID {
		t.Fatalf("unexpected active schedules after deactivate: %#v", items)
	}

	err = repo.DeactivateLoveNoteSchedule(ctx, second.ID)
	if !errors.Is(err, ErrLoveNoteScheduleNotFound) {
		t.Fatalf("expected ErrLoveNoteScheduleNotFound, got %v", err)
	}
}

func openLoveNoteScheduleTestDB(t *testing.T) *sql.DB {
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

func createLoveNoteScheduleTestUser(t *testing.T, ctx context.Context, db *sql.DB, telegramUserID int64, firstName string, userType string) int64 {
	t.Helper()

	result, err := db.ExecContext(ctx, `
		INSERT INTO users (telegram_user_id, first_name, type)
		VALUES (?, ?, ?)
	`, telegramUserID, firstName, userType)
	if err != nil {
		t.Fatalf("insert love note schedule test user: %v", err)
	}

	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId(): %v", err)
	}

	return userID
}
