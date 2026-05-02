package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const memoryDateTimeLayout = "2006-01-02 15:04:05"

var ErrMemoryNotFound = errors.New("memory not found")

type MemoryRepository struct {
	db *sql.DB
}

func NewMemoryRepository(db *sql.DB) *MemoryRepository {
	return &MemoryRepository{db: db}
}

func (r *MemoryRepository) SaveMemory(ctx context.Context, telegramUserID int64, firstName string, text string, telegramFileID string, telegramFileUnique string, createdAt *time.Time) (Memory, error) {
	userID, err := r.ensureUser(ctx, telegramUserID, firstName)
	if err != nil {
		return Memory{}, err
	}

	query := `
		INSERT INTO memories (user_id, text, telegram_file_id, telegram_file_unique_id)
		VALUES (?, ?, ?, ?)
	`
	args := []any{userID, text, telegramFileID, telegramFileUnique}
	if createdAt != nil {
		query = `
			INSERT INTO memories (user_id, text, telegram_file_id, telegram_file_unique_id, created_at)
			VALUES (?, ?, ?, ?, ?)
		`
		args = append(args, createdAt.Local().Format(memoryDateTimeLayout))
	}

	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return Memory{}, fmt.Errorf("insert memory: %w", err)
	}

	memoryID, err := result.LastInsertId()
	if err != nil {
		return Memory{}, fmt.Errorf("get inserted memory id: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, text, telegram_file_id, telegram_file_unique_id, created_at
		FROM memories
		WHERE id = ?
	`, memoryID)

	var memory Memory
	if err := row.Scan(&memory.ID, &memory.UserID, &memory.Text, &memory.TelegramFileID, &memory.TelegramFileUnique, &memory.CreatedAt); err != nil {
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
		SELECT id, user_id, text, telegram_file_id, telegram_file_unique_id, created_at
		FROM memories
		WHERE user_id = ?
		ORDER BY created_at DESC, id DESC
		LIMIT ?
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()

	memories := make([]Memory, 0, limit)
	for rows.Next() {
		var memory Memory
		if err := rows.Scan(&memory.ID, &memory.UserID, &memory.Text, &memory.TelegramFileID, &memory.TelegramFileUnique, &memory.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		memories = append(memories, memory)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memories: %w", err)
	}

	return memories, nil
}

func (r *MemoryRepository) RandomMemory(ctx context.Context) (Memory, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, text, telegram_file_id, telegram_file_unique_id, created_at
		FROM memories
		ORDER BY RANDOM()
		LIMIT 1
	`)

	var memory Memory
	if err := row.Scan(&memory.ID, &memory.UserID, &memory.Text, &memory.TelegramFileID, &memory.TelegramFileUnique, &memory.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Memory{}, ErrMemoryNotFound
		}
		return Memory{}, fmt.Errorf("query random memory: %w", err)
	}

	return memory, nil
}

func (r *MemoryRepository) NextRandomMemoryForUser(ctx context.Context, telegramUserID int64) (Memory, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Memory{}, fmt.Errorf("begin memory cycle transaction: %w", err)
	}
	defer tx.Rollback()

	userID, err := userIDByTelegramUserID(ctx, tx, telegramUserID)
	if err != nil {
		return Memory{}, err
	}

	memory, err := nextRandomMemoryForUser(ctx, tx, userID)
	if err != nil {
		return Memory{}, err
	}

	if err := tx.Commit(); err != nil {
		return Memory{}, fmt.Errorf("commit memory cycle transaction: %w", err)
	}

	return memory, nil
}

func (r *MemoryRepository) ensureUser(ctx context.Context, telegramUserID int64, firstName string) (int64, error) {
	_ = firstName
	userID, err := userIDByTelegramUserID(ctx, r.db, telegramUserID)
	if err != nil {
		return 0, fmt.Errorf("fetch user: %w", err)
	}

	return userID, nil
}

func nextRandomMemoryForUser(ctx context.Context, tx *sql.Tx, userID int64) (Memory, error) {
	memory, found, err := selectUnseenMemory(ctx, tx, userID)
	if err != nil {
		return Memory{}, err
	}
	if found {
		return memory, nil
	}

	var hasMemories bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM memories)
	`).Scan(&hasMemories); err != nil {
		return Memory{}, fmt.Errorf("check memories existence: %w", err)
	}
	if !hasMemories {
		return Memory{}, ErrMemoryNotFound
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM memory_user_cycle
		WHERE user_id = ?
	`, userID); err != nil {
		return Memory{}, fmt.Errorf("reset memory cycle: %w", err)
	}

	memory, found, err = selectUnseenMemory(ctx, tx, userID)
	if err != nil {
		return Memory{}, err
	}
	if !found {
		return Memory{}, ErrMemoryNotFound
	}

	return memory, nil
}

func selectUnseenMemory(ctx context.Context, tx *sql.Tx, userID int64) (Memory, bool, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT m.id, m.user_id, m.text, m.telegram_file_id, m.telegram_file_unique_id, m.created_at
		FROM memories m
		WHERE NOT EXISTS (
			SELECT 1
			FROM memory_user_cycle c
			WHERE c.user_id = ? AND c.memory_id = m.id
		)
		ORDER BY RANDOM()
		LIMIT 1
	`, userID)

	var memory Memory
	if err := row.Scan(&memory.ID, &memory.UserID, &memory.Text, &memory.TelegramFileID, &memory.TelegramFileUnique, &memory.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Memory{}, false, nil
		}
		return Memory{}, false, fmt.Errorf("query unseen memory: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO memory_user_cycle (user_id, memory_id)
		VALUES (?, ?)
	`, userID, memory.ID); err != nil {
		return Memory{}, false, fmt.Errorf("insert memory cycle row: %w", err)
	}

	return memory, true, nil
}
