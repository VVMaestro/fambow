package repository

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestCelebrationRepositorySaveEventNormalizesTimestampDate(t *testing.T) {
	ctx := context.Background()
	db := openCelebrationTestDB(t)
	repo := NewCelebrationRepository(db)

	createCelebrationTestUser(t, ctx, db, 101, "Anna", UserTypeWife)

	event, err := repo.SaveEvent(ctx, 101, "Anna", "Anniversary dinner", "2026-03-27T00:00:00Z", 3)
	if err != nil {
		t.Fatalf("SaveEvent() unexpected error: %v", err)
	}

	if got := event.EventDate.Format("2006-01-02"); got != "2026-03-27" {
		t.Fatalf("expected normalized event date, got %q", got)
	}
}

func TestCelebrationRepositoryReadsTimestampDatesFromSQLite(t *testing.T) {
	ctx := context.Background()
	db := openCelebrationTestDB(t)
	repo := NewCelebrationRepository(db)

	userID := createCelebrationTestUser(t, ctx, db, 202, "Mila", UserTypeWife)
	if _, err := db.ExecContext(ctx, `
		INSERT INTO events (user_id, title, event_date, remind_days_before)
		VALUES (?, ?, ?, ?)
	`, userID, "Birthday", "2026-03-27T00:00:00Z", 3); err != nil {
		t.Fatalf("insert event: %v", err)
	}

	events, err := repo.ListEventsByTelegramUser(ctx, 202)
	if err != nil {
		t.Fatalf("ListEventsByTelegramUser() unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if got := events[0].EventDate.Format("2006-01-02"); got != "2026-03-27" {
		t.Fatalf("expected normalized listed event date, got %q", got)
	}

	due, err := repo.ListDueEventReminders(ctx, "2026-03-24")
	if err != nil {
		t.Fatalf("ListDueEventReminders() unexpected error: %v", err)
	}
	if len(due) != 1 {
		t.Fatalf("expected 1 due reminder, got %d", len(due))
	}
	if got := due[0].EventDate.Format("2006-01-02"); got != "2026-03-27" {
		t.Fatalf("expected normalized due event date, got %q", got)
	}
}

func openCelebrationTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite(): %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	migrationFile := filepath.Join("..", "..", "migrations", "001_init.sql")
	if err := RunMigrationFile(context.Background(), db, migrationFile); err != nil {
		t.Fatalf("RunMigrationFile(): %v", err)
	}

	return db
}

func createCelebrationTestUser(t *testing.T, ctx context.Context, db *sql.DB, telegramUserID int64, firstName string, userType string) int64 {
	t.Helper()

	result, err := db.ExecContext(ctx, `
		INSERT INTO users (telegram_user_id, first_name, type)
		VALUES (?, ?, ?)
	`, telegramUserID, firstName, userType)
	if err != nil {
		t.Fatalf("insert test user: %v", err)
	}

	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId(): %v", err)
	}

	return userID
}
