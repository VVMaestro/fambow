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

	note, err := scanLoveNote(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoveNote{}, nil
		}
		return LoveNote{}, fmt.Errorf("query random love note: %w", err)
	}

	return note, nil
}

func (r *LoveNoteRepository) ListLoveNotes(ctx context.Context) ([]LoveNote, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, text, tag, telegram_file_id, telegram_file_unique_id, created_at
		FROM love_notes
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list love notes: %w", err)
	}
	defer rows.Close()

	notes := make([]LoveNote, 0)
	for rows.Next() {
		note, err := scanLoveNote(rows)
		if err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate listed love notes: %w", err)
	}

	return notes, nil
}

func (r *LoveNoteRepository) DeleteLoveNotes(ctx context.Context, noteIDs []int64) ([]int64, error) {
	if len(noteIDs) == 0 {
		return nil, nil
	}

	query, args := loveNoteInClauseQuery(`
		SELECT id
		FROM love_notes
		WHERE id IN (%s)
	`, noteIDs)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select love notes for delete: %w", err)
	}
	defer rows.Close()

	existingIDs := make([]int64, 0, len(noteIDs))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan love note id for delete: %w", err)
		}
		existingIDs = append(existingIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate love note ids for delete: %w", err)
	}
	if len(existingIDs) == 0 {
		return nil, nil
	}

	deleteQuery, deleteArgs := loveNoteInClauseQuery(`
		DELETE FROM love_notes
		WHERE id IN (%s)
	`, existingIDs)
	if _, err := r.db.ExecContext(ctx, deleteQuery, deleteArgs...); err != nil {
		return nil, fmt.Errorf("delete love notes: %w", err)
	}

	return existingIDs, nil
}

func scanLoveNote(scanner interface {
	Scan(dest ...any) error
}) (LoveNote, error) {
	var (
		note               LoveNote
		tag                sql.NullString
		telegramFileID     sql.NullString
		telegramFileUnique sql.NullString
	)
	if err := scanner.Scan(&note.ID, &note.Text, &tag, &telegramFileID, &telegramFileUnique, &note.CreatedAt); err != nil {
		return LoveNote{}, err
	}

	note.Tag = nullStringValue(tag)
	note.TelegramFileID = nullStringValue(telegramFileID)
	note.TelegramFileUnique = nullStringValue(telegramFileUnique)

	return note, nil
}

func loveNoteInClauseQuery(template string, noteIDs []int64) (string, []any) {
	placeholders := make([]string, 0, len(noteIDs))
	args := make([]any, 0, len(noteIDs))
	for _, id := range noteIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}

	return fmt.Sprintf(template, strings.Join(placeholders, ",")), args
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
