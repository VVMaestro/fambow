package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRunMigrationsAppliesSQLFilesOnceInOrder(t *testing.T) {
	ctx := context.Background()
	db, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite(): %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	migrationsDir := t.TempDir()
	writeTestFile(t, filepath.Join(migrationsDir, "001_create_items.sql"), `
		CREATE TABLE items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL
		);
	`)
	writeTestFile(t, filepath.Join(migrationsDir, "002_seed_items.sql"), `
		INSERT INTO items (name) VALUES ('first');
	`)

	if err := RunMigrations(ctx, db, migrationsDir); err != nil {
		t.Fatalf("RunMigrations() first run: %v", err)
	}
	if err := RunMigrations(ctx, db, migrationsDir); err != nil {
		t.Fatalf("RunMigrations() second run: %v", err)
	}

	var itemCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items`).Scan(&itemCount); err != nil {
		t.Fatalf("count items: %v", err)
	}
	if itemCount != 1 {
		t.Fatalf("expected 1 seeded item after rerun, got %d", itemCount)
	}

	var migrationCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&migrationCount); err != nil {
		t.Fatalf("count schema migrations: %v", err)
	}
	if migrationCount != 2 {
		t.Fatalf("expected 2 recorded migrations, got %d", migrationCount)
	}
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}
