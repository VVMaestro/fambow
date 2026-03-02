package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type LoveNoteRepository struct {
	db *sql.DB
}

func NewLoveNoteRepository(db *sql.DB) *LoveNoteRepository {
	return &LoveNoteRepository{db: db}
}

func (r *LoveNoteRepository) AddDefaultNotes(ctx context.Context, notes []string) error {
	count, err := r.countNotes(ctx)
	if err != nil {
		return err
	}

	if count > 0 {
		return nil
	}

	r.AddNotes(ctx, notes)

	return nil
}

func (r *LoveNoteRepository) AddNotes(ctx context.Context, notes []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin love notes transaction: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO love_notes (text, tag)
		VALUES (?, ?)
	`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare love note insert: %w", err)
	}
	defer stmt.Close()

	for _, note := range notes {
		trimmed := strings.TrimSpace(note)
		if trimmed == "" {
			continue
		}

		if _, err := stmt.ExecContext(ctx, trimmed, "default"); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert love note: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit love note transaction: %w", err)
	}

	return nil
}

func (r *LoveNoteRepository) RandomNoteText(ctx context.Context) (string, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT text
		FROM love_notes
		ORDER BY RANDOM()
		LIMIT 1
	`)

	var text string
	if err := row.Scan(&text); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("query random love note: %w", err)
	}

	return text, nil
}

func (r *LoveNoteRepository) countNotes(ctx context.Context) (int, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM love_notes
	`)

	var count int
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("count love notes: %w", err)
	}

	return count, nil
}
