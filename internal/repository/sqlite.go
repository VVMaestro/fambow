package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	_ "modernc.org/sqlite"
)

func OpenSQLite(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	return db, nil
}

func RunMigrations(ctx context.Context, db *sql.DB, migrationsDir string) error {
	if err := ensureSchemaMigrationsTable(ctx, db); err != nil {
		return err
	}

	migrationFiles, err := listMigrationFiles(migrationsDir)
	if err != nil {
		return err
	}

	applied, err := listAppliedMigrations(ctx, db)
	if err != nil {
		return err
	}

	for _, migrationFile := range migrationFiles {
		migrationName := filepath.Base(migrationFile)
		if _, exists := applied[migrationName]; exists {
			continue
		}

		if err := applyMigration(ctx, db, migrationFile, migrationName); err != nil {
			return err
		}
	}

	return nil
}

func ensureSchemaMigrationsTable(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	return nil
}

func listMigrationFiles(migrationsDir string) ([]string, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("read migrations directory: %w", err)
	}

	var migrationFiles []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".sql") {
			continue
		}

		migrationFiles = append(migrationFiles, filepath.Join(migrationsDir, entry.Name()))
	}

	if len(migrationFiles) == 0 {
		return nil, fmt.Errorf("no .sql migration files found in %q", migrationsDir)
	}

	slices.Sort(migrationFiles)

	return migrationFiles, nil
}

func listAppliedMigrations(ctx context.Context, db *sql.DB) (map[string]struct{}, error) {
	rows, err := db.QueryContext(ctx, `SELECT filename FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("list applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]struct{})
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[filename] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}

	return applied, nil
}

func applyMigration(ctx context.Context, db *sql.DB, migrationFile string, migrationName string) error {
	sqlBytes, err := os.ReadFile(migrationFile)
	if err != nil {
		return fmt.Errorf("read migration file %q: %w", migrationName, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration transaction %q: %w", migrationName, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("apply migration file %q: %w", migrationName, err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO schema_migrations (filename)
		VALUES (?)
	`, migrationName); err != nil {
		return fmt.Errorf("record migration file %q: %w", migrationName, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration file %q: %w", migrationName, err)
	}

	return nil
}
