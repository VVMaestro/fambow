package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type DueEventReminder struct {
	EventID         int64
	TelegramUserID  int64
	FirstName       string
	Title           string
	EventDate       time.Time
	DaysUntilEvent  int
	RemindDate      string
	RemindDaysPrior int
}

type CelebrationRepository struct {
	db *sql.DB
}

func NewCelebrationRepository(db *sql.DB) *CelebrationRepository {
	return &CelebrationRepository{db: db}
}

func (r *CelebrationRepository) SaveEvent(ctx context.Context, telegramUserID int64, firstName string, title string, eventDate string, remindDaysBefore int) (Event, error) {
	userID, err := r.ensureUser(ctx, telegramUserID, firstName)
	if err != nil {
		return Event{}, err
	}

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO events (user_id, title, event_date, remind_days_before)
		VALUES (?, ?, ?, ?)
	`, userID, title, eventDate, remindDaysBefore)
	if err != nil {
		return Event{}, fmt.Errorf("insert event: %w", err)
	}

	eventID, err := result.LastInsertId()
	if err != nil {
		return Event{}, fmt.Errorf("get inserted event id: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, title, event_date, remind_days_before
		FROM events
		WHERE id = ?
	`, eventID)

	var event Event
	var eventDateRaw string
	if err := row.Scan(&event.ID, &event.UserID, &event.Title, &eventDateRaw, &event.RemindDaysBefore); err != nil {
		return Event{}, fmt.Errorf("scan inserted event: %w", err)
	}

	parsedEventDate, err := time.ParseInLocation("2006-01-02", eventDateRaw, time.Local)
	if err != nil {
		return Event{}, fmt.Errorf("parse inserted event date: %w", err)
	}
	event.EventDate = parsedEventDate

	return event, nil
}

func (r *CelebrationRepository) ListEventsByTelegramUser(ctx context.Context, telegramUserID int64) ([]Event, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT e.id, e.user_id, e.title, e.event_date, e.remind_days_before
		FROM events e
		JOIN users u ON u.id = e.user_id
		WHERE u.telegram_user_id = ?
		ORDER BY e.event_date ASC
	`, telegramUserID)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	events := make([]Event, 0)
	for rows.Next() {
		var event Event
		var eventDateRaw string
		if err := rows.Scan(&event.ID, &event.UserID, &event.Title, &eventDateRaw, &event.RemindDaysBefore); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}

		eventDate, err := time.ParseInLocation("2006-01-02", eventDateRaw, time.Local)
		if err != nil {
			return nil, fmt.Errorf("parse event date: %w", err)
		}
		event.EventDate = eventDate
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}

	return events, nil
}

func (r *CelebrationRepository) ListDueEventReminders(ctx context.Context, dispatchDate string) ([]DueEventReminder, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			e.id,
			u.telegram_user_id,
			u.first_name,
			e.title,
			e.event_date,
			CAST(julianday(e.event_date) - julianday(?) AS INTEGER) AS days_until_event,
			e.remind_days_before
		FROM events e
		JOIN users u ON u.id = e.user_id
		WHERE date(e.event_date, printf('-%d day', e.remind_days_before)) = date(?)
		  AND NOT EXISTS (
			SELECT 1
			FROM event_dispatches d
			WHERE d.event_id = e.id
			  AND d.dispatch_date = date(?)
		  )
	`, dispatchDate, dispatchDate, dispatchDate)
	if err != nil {
		return nil, fmt.Errorf("query due event reminders: %w", err)
	}
	defer rows.Close()

	items := make([]DueEventReminder, 0)
	for rows.Next() {
		var item DueEventReminder
		var eventDateRaw string
		if err := rows.Scan(
			&item.EventID,
			&item.TelegramUserID,
			&item.FirstName,
			&item.Title,
			&eventDateRaw,
			&item.DaysUntilEvent,
			&item.RemindDaysPrior,
		); err != nil {
			return nil, fmt.Errorf("scan due event reminder: %w", err)
		}

		eventDate, err := time.ParseInLocation("2006-01-02", eventDateRaw, time.Local)
		if err != nil {
			return nil, fmt.Errorf("parse due event date: %w", err)
		}
		item.EventDate = eventDate
		item.RemindDate = dispatchDate
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due event reminders: %w", err)
	}

	return items, nil
}

func (r *CelebrationRepository) MarkEventReminderDispatched(ctx context.Context, eventID int64, dispatchDate string) error {
	if _, err := r.db.ExecContext(ctx, `
		INSERT INTO event_dispatches (event_id, dispatch_date)
		VALUES (?, ?)
		ON CONFLICT(event_id, dispatch_date) DO NOTHING
	`, eventID, dispatchDate); err != nil {
		return fmt.Errorf("insert event dispatch record: %w", err)
	}

	return nil
}

func (r *CelebrationRepository) ensureUser(ctx context.Context, telegramUserID int64, firstName string) (int64, error) {
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
