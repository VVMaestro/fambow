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

	if err := r.addTextNotes(ctx, notes, "default"); err != nil {
		return err
	}

	return nil
}

func (r *LoveNoteRepository) AddLoveNote(ctx context.Context, note LoveNote) error {
	trimmedText := strings.TrimSpace(note.Text)
	telegramFileID := strings.TrimSpace(note.TelegramFileID)
	telegramFileUnique := strings.TrimSpace(note.TelegramFileUnique)

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO love_notes (text, tag, telegram_file_id, telegram_file_unique_id)
		VALUES (?, ?, ?, ?)
	`, trimmedText, "custom", nullableString(telegramFileID), nullableString(telegramFileUnique))
	if err != nil {
		return fmt.Errorf("insert love note: %w", err)
	}

	return nil
}

func (r *LoveNoteRepository) addTextNotes(ctx context.Context, notes []string, tag string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin love notes transaction: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO love_notes (text, tag, telegram_file_id, telegram_file_unique_id)
		VALUES (?, ?, NULL, NULL)
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

		if _, err := stmt.ExecContext(ctx, trimmed, tag); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert love note: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit love note transaction: %w", err)
	}

	return nil
}

func (r *LoveNoteRepository) RandomNote(ctx context.Context) (LoveNote, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, text, tag, telegram_file_id, telegram_file_unique_id, created_at
		FROM love_notes
		ORDER BY RANDOM()
		LIMIT 1
	`)

	var (
		note               LoveNote
		tag                sql.NullString
		telegramFileID     sql.NullString
		telegramFileUnique sql.NullString
	)
	if err := row.Scan(&note.ID, &note.Text, &tag, &telegramFileID, &telegramFileUnique, &note.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoveNote{}, nil
		}
		return LoveNote{}, fmt.Errorf("query random love note: %w", err)
	}

	note.Tag = nullStringValue(tag)
	note.TelegramFileID = nullStringValue(telegramFileID)
	note.TelegramFileUnique = nullStringValue(telegramFileUnique)

	return note, nil
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

func nullableString(value string) any {
	if value == "" {
		return nil
	}

	return value
}

func nullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}

	return value.String
}
