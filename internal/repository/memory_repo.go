package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type MemoryRepository struct {
	db *sql.DB
}

func NewMemoryRepository(db *sql.DB) *MemoryRepository {
	return &MemoryRepository{db: db}
}

func (r *MemoryRepository) SaveMemory(ctx context.Context, telegramUserID int64, firstName string, text string) (Memory, error) {
	userID, err := r.ensureUser(ctx, telegramUserID, firstName)
	if err != nil {
		return Memory{}, err
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO memories (user_id, text)
		VALUES (?, ?)
	`, userID, text)
	if err != nil {
		return Memory{}, fmt.Errorf("insert memory: %w", err)
	}

	memoryID, err := result.LastInsertId()
	if err != nil {
		return Memory{}, fmt.Errorf("get inserted memory id: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, text, created_at
		FROM memories
		WHERE id = ?
	`, memoryID)

	var memory Memory
	if err := row.Scan(&memory.ID, &memory.UserID, &memory.Text, &memory.CreatedAt); err != nil {
		return Memory{}, fmt.Errorf("scan inserted memory: %w", err)
	}

	return memory, nil
}

func (r *MemoryRepository) ListRecentMemories(ctx context.Context, telegramUserID int64, limit int) ([]Memory, error) {
	if limit <= 0 {
		limit = 5
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT id
		FROM users
		WHERE telegram_user_id = ?
	`, telegramUserID)

	var userID int64
	if err := row.Scan(&userID); err != nil {
		if err == sql.ErrNoRows {
			return []Memory{}, nil
		}
		return nil, fmt.Errorf("fetch user for memories: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, text, created_at
		FROM memories
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()

	memories := make([]Memory, 0, limit)
	for rows.Next() {
		var memory Memory
		if err := rows.Scan(&memory.ID, &memory.UserID, &memory.Text, &memory.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		memories = append(memories, memory)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memories: %w", err)
	}

	return memories, nil
}

func (r *MemoryRepository) ensureUser(ctx context.Context, telegramUserID int64, firstName string) (int64, error) {
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO users (telegram_user_id, first_name)
		VALUES (?, ?)
		ON CONFLICT(telegram_user_id) DO UPDATE SET first_name = excluded.first_name
	`, telegramUserID, firstName); err != nil {
		return 0, fmt.Errorf("upsert user: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT id
		FROM users
		WHERE telegram_user_id = ?
	`, telegramUserID)

	var userID int64
	if err := row.Scan(&userID); err != nil {
		return 0, fmt.Errorf("fetch user id: %w", err)
	}

	return userID, nil
}
