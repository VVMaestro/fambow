package repository

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestReminderRepositoryListByActiveStateAndDeactivate(t *testing.T) {
	ctx := context.Background()
	db := openReminderTestDB(t)
	repo := NewReminderRepository(db)

	annaUserID := createReminderTestUser(t, ctx, db, 101, "Anna", UserTypeWife)
	mishaUserID := createReminderTestUser(t, ctx, db, 202, "Misha", UserTypeHusband)

	activeDaily := createReminderRow(t, ctx, db, annaUserID, "vitamins", "daily", "08:00", true)
	createReminderRow(t, ctx, db, mishaUserID, "call mom", "one_time", "2026-03-21 19:30:00", true)
	inactiveID := createReminderRow(t, ctx, db, annaUserID, "old reminder", "daily", "21:00", false)

	activeItems, err := repo.ListRemindersByActiveState(ctx, true)
	if err != nil {
		t.Fatalf("ListRemindersByActiveState(active) unexpected error: %v", err)
	}
	if len(activeItems) != 2 {
		t.Fatalf("expected 2 active reminders, got %d", len(activeItems))
	}
	if activeItems[0].ID != activeDaily || activeItems[0].FirstName != "Anna" || activeItems[0].UserType != UserTypeWife {
		t.Fatalf("unexpected first active item: %#v", activeItems[0])
	}
	if activeItems[1].FirstName != "Misha" || activeItems[1].UserType != UserTypeHusband {
		t.Fatalf("unexpected second active item: %#v", activeItems[1])
	}

	inactiveItems, err := repo.ListRemindersByActiveState(ctx, false)
	if err != nil {
		t.Fatalf("ListRemindersByActiveState(inactive) unexpected error: %v", err)
	}
	if len(inactiveItems) != 1 || inactiveItems[0].ID != inactiveID || inactiveItems[0].IsActive {
		t.Fatalf("unexpected inactive items: %#v", inactiveItems)
	}

	if err := repo.DeactivateReminder(ctx, activeDaily); err != nil {
		t.Fatalf("DeactivateReminder() unexpected error: %v", err)
	}

	inactiveItems, err = repo.ListRemindersByActiveState(ctx, false)
	if err != nil {
		t.Fatalf("ListRemindersByActiveState(inactive after deactivate) unexpected error: %v", err)
	}
	if len(inactiveItems) != 2 {
		t.Fatalf("expected 2 inactive reminders after deactivate, got %d", len(inactiveItems))
	}

	err = repo.DeactivateReminder(ctx, activeDaily)
	if !errors.Is(err, ErrReminderNotFound) {
		t.Fatalf("expected ErrReminderNotFound for already inactive reminder, got %v", err)
	}

	err = repo.DeactivateReminder(ctx, 999)
	if !errors.Is(err, ErrReminderNotFound) {
		t.Fatalf("expected ErrReminderNotFound for missing reminder, got %v", err)
	}
}

func openReminderTestDB(t *testing.T) *sql.DB {
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

func createReminderTestUser(t *testing.T, ctx context.Context, db *sql.DB, telegramUserID int64, firstName string, userType string) int64 {
	t.Helper()

	result, err := db.ExecContext(ctx, `
		INSERT INTO users (telegram_user_id, first_name, type)
		VALUES (?, ?, ?)
	`, telegramUserID, firstName, userType)
	if err != nil {
		t.Fatalf("insert reminder test user: %v", err)
	}

	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId(): %v", err)
	}

	return userID
}

func createReminderRow(t *testing.T, ctx context.Context, db *sql.DB, userID int64, text string, scheduleType string, scheduleValue string, isActive bool) int64 {
	t.Helper()

	result, err := db.ExecContext(ctx, `
		INSERT INTO reminders (user_id, text, schedule_type, schedule_value, is_active)
		VALUES (?, ?, ?, ?, ?)
	`, userID, text, scheduleType, scheduleValue, isActive)
	if err != nil {
		t.Fatalf("insert reminder row: %v", err)
	}

	reminderID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId(): %v", err)
	}

	return reminderID
}
