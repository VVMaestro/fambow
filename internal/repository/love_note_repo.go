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
