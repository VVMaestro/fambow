package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"

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

func RunMigrationFile(ctx context.Context, db *sql.DB, migrationFile string) error {
	sqlBytes, err := os.ReadFile(migrationFile)
	if err != nil {
		return fmt.Errorf("read migration file: %w", err)
	}

	if _, err := db.ExecContext(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("apply migration file: %w", err)
	}

	return nil
}
