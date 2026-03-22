package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrLoveNoteScheduleNotFound = errors.New("love note schedule not found")

type LoveNoteScheduleItem struct {
	ID             int64
	TelegramUserID int64
	FirstName      string
	UserType       string
	ScheduleTime   string
}

type LoveNoteScheduleRepository struct {
	db *sql.DB
}

func NewLoveNoteScheduleRepository(db *sql.DB) *LoveNoteScheduleRepository {
	return &LoveNoteScheduleRepository{db: db}
}

func (r *LoveNoteScheduleRepository) SaveLoveNoteSchedule(ctx context.Context, telegramUserID int64, scheduleTime string) (LoveNoteScheduleItem, error) {
	userID, err := userIDByTelegramUserID(ctx, r.db, telegramUserID)
	if err != nil {
		return LoveNoteScheduleItem{}, err
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO love_note_schedules (user_id, schedule_time, is_active)
		VALUES (?, ?, 1)
	`, userID, scheduleTime)
	if err != nil {
		return LoveNoteScheduleItem{}, fmt.Errorf("insert love note schedule: %w", err)
	}

	scheduleID, err := result.LastInsertId()
	if err != nil {
		return LoveNoteScheduleItem{}, fmt.Errorf("get inserted love note schedule id: %w", err)
	}

	return r.findLoveNoteScheduleItemByID(ctx, scheduleID)
}

func (r *LoveNoteScheduleRepository) ListActiveLoveNoteSchedules(ctx context.Context) ([]LoveNoteScheduleItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, u.telegram_user_id, u.first_name, u.type, s.schedule_time
		FROM love_note_schedules s
		JOIN users u ON u.id = s.user_id
		WHERE s.is_active = 1
		ORDER BY s.schedule_time ASC, lower(u.first_name) ASC, s.id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list active love note schedules: %w", err)
	}
	defer rows.Close()

	items := make([]LoveNoteScheduleItem, 0)
	for rows.Next() {
		item, err := scanLoveNoteScheduleItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active love note schedules: %w", err)
	}

	return items, nil
}

func (r *LoveNoteScheduleRepository) DeactivateLoveNoteSchedule(ctx context.Context, scheduleID int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE love_note_schedules
		SET is_active = 0
		WHERE id = ? AND is_active = 1
	`, scheduleID)
	if err != nil {
		return fmt.Errorf("deactivate love note schedule: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deactivated love note schedule rows: %w", err)
	}
	if rowsAffected == 0 {
		return ErrLoveNoteScheduleNotFound
	}

	return nil
}

func (r *LoveNoteScheduleRepository) ListDueLoveNoteSchedules(ctx context.Context, scheduleTime string, dispatchDate string) ([]LoveNoteScheduleItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT s.id, u.telegram_user_id, u.first_name, u.type, s.schedule_time
		FROM love_note_schedules s
		JOIN users u ON u.id = s.user_id
		WHERE s.is_active = 1
		  AND s.schedule_time = ?
		  AND NOT EXISTS (
			SELECT 1
			FROM love_note_schedule_dispatches d
			WHERE d.schedule_id = s.id
			  AND d.dispatch_date = ?
		  )
		ORDER BY s.id ASC
	`, scheduleTime, dispatchDate)
	if err != nil {
		return nil, fmt.Errorf("query due love note schedules: %w", err)
	}
	defer rows.Close()

	items := make([]LoveNoteScheduleItem, 0)
	for rows.Next() {
		item, err := scanLoveNoteScheduleItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due love note schedules: %w", err)
	}

	return items, nil
}

func (r *LoveNoteScheduleRepository) MarkLoveNoteScheduleDispatched(ctx context.Context, scheduleID int64, dispatchDate string) error {
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO love_note_schedule_dispatches (schedule_id, dispatch_date)
		VALUES (?, ?)
		ON CONFLICT(schedule_id, dispatch_date) DO NOTHING
	`, scheduleID, dispatchDate); err != nil {
		return fmt.Errorf("insert love note schedule dispatch: %w", err)
	}

	return nil
}

func (r *LoveNoteScheduleRepository) findLoveNoteScheduleItemByID(ctx context.Context, scheduleID int64) (LoveNoteScheduleItem, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT s.id, u.telegram_user_id, u.first_name, u.type, s.schedule_time
		FROM love_note_schedules s
		JOIN users u ON u.id = s.user_id
		WHERE s.id = ?
		LIMIT 1
	`, scheduleID)

	var item LoveNoteScheduleItem
	if err := row.Scan(&item.ID, &item.TelegramUserID, &item.FirstName, &item.UserType, &item.ScheduleTime); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LoveNoteScheduleItem{}, ErrLoveNoteScheduleNotFound
		}
		return LoveNoteScheduleItem{}, fmt.Errorf("find love note schedule by id: %w", err)
	}

	return item, nil
}

func scanLoveNoteScheduleItem(scanner interface {
	Scan(dest ...any) error
}) (LoveNoteScheduleItem, error) {
	var item LoveNoteScheduleItem
	if err := scanner.Scan(&item.ID, &item.TelegramUserID, &item.FirstName, &item.UserType, &item.ScheduleTime); err != nil {
		return LoveNoteScheduleItem{}, fmt.Errorf("scan love note schedule item: %w", err)
	}

	return item, nil
}
