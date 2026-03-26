package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrReminderNotFound = errors.New("reminder not found")

type ReminderDueItem struct {
	ID             int64
	TelegramUserID int64
	FirstName      string
	Text           string
	ScheduleType   string
}

type ReminderRepository struct {
	db *sql.DB
}

func NewReminderRepository(db *sql.DB) *ReminderRepository {
	return &ReminderRepository{db: db}
}

func (r *ReminderRepository) SaveReminder(ctx context.Context, telegramUserID int64, firstName string, text string, scheduleType string, scheduleValue string) (Reminder, error) {
	userID, err := r.ensureUser(ctx, telegramUserID, firstName)
	if err != nil {
		return Reminder{}, err
	}

	return r.saveReminderForUserID(ctx, userID, text, scheduleType, scheduleValue)
}

func (r *ReminderRepository) SaveReminderForUserType(ctx context.Context, userType string, text string, scheduleType string, scheduleValue string) (Reminder, error) {
	userID, err := userIDByType(ctx, r.db, userType)
	if err != nil {
		return Reminder{}, err
	}

	return r.saveReminderForUserID(ctx, userID, text, scheduleType, scheduleValue)
}

func (r *ReminderRepository) saveReminderForUserID(ctx context.Context, userID int64, text string, scheduleType string, scheduleValue string) (Reminder, error) {

	result, err := r.db.ExecContext(ctx, `
		INSERT INTO reminders (user_id, text, schedule_type, schedule_value, is_active)
		VALUES (?, ?, ?, ?, 1)
	`, userID, text, scheduleType, scheduleValue)
	if err != nil {
		return Reminder{}, fmt.Errorf("insert reminder: %w", err)
	}

	reminderID, err := result.LastInsertId()
	if err != nil {
		return Reminder{}, fmt.Errorf("get inserted reminder id: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, text, schedule_type, schedule_value, is_active
		FROM reminders
		WHERE id = ?
	`, reminderID)

	var reminder Reminder
	if err := row.Scan(&reminder.ID, &reminder.UserID, &reminder.Text, &reminder.ScheduleType, &reminder.ScheduleValue, &reminder.IsActive); err != nil {
		return Reminder{}, fmt.Errorf("scan inserted reminder: %w", err)
	}

	return reminder, nil
}

func (r *ReminderRepository) ListActiveReminders(ctx context.Context, telegramUserID int64) ([]Reminder, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.id, r.user_id, r.text, r.schedule_type, r.schedule_value, r.is_active
		FROM reminders r
		JOIN users u ON u.id = r.user_id
		WHERE u.telegram_user_id = ? AND r.is_active = 1
		ORDER BY r.schedule_type ASC, r.schedule_value ASC, r.id ASC
	`, telegramUserID)
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	defer rows.Close()

	reminders := make([]Reminder, 0)
	for rows.Next() {
		var reminder Reminder
		if err := rows.Scan(&reminder.ID, &reminder.UserID, &reminder.Text, &reminder.ScheduleType, &reminder.ScheduleValue, &reminder.IsActive); err != nil {
			return nil, fmt.Errorf("scan reminder: %w", err)
		}
		reminders = append(reminders, reminder)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reminders: %w", err)
	}

	return reminders, nil
}

func (r *ReminderRepository) ListRemindersByActiveState(ctx context.Context, isActive bool) ([]AdminReminderItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.id, u.telegram_user_id, u.first_name, u.type, r.text, r.schedule_type, r.schedule_value, r.is_active
		FROM reminders r
		JOIN users u ON u.id = r.user_id
		WHERE r.is_active = ?
		ORDER BY u.first_name ASC, r.schedule_type ASC, r.schedule_value ASC, r.id ASC
	`, isActive)
	if err != nil {
		return nil, fmt.Errorf("list reminders by active state: %w", err)
	}
	defer rows.Close()

	items := make([]AdminReminderItem, 0)
	for rows.Next() {
		var item AdminReminderItem
		if err := rows.Scan(&item.ID, &item.TelegramUserID, &item.FirstName, &item.UserType, &item.Text, &item.ScheduleType, &item.ScheduleValue, &item.IsActive); err != nil {
			return nil, fmt.Errorf("scan admin reminder item: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate admin reminder items: %w", err)
	}

	return items, nil
}

func (r *ReminderRepository) ListDueOneTimeReminders(ctx context.Context, nowTimestamp string) ([]ReminderDueItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.id, u.telegram_user_id, u.first_name, r.text, r.schedule_type
		FROM reminders r
		JOIN users u ON u.id = r.user_id
		WHERE r.is_active = 1
		  AND r.schedule_type = 'one_time'
		  AND r.schedule_value <= ?
	`, nowTimestamp)
	if err != nil {
		return nil, fmt.Errorf("query due one-time reminders: %w", err)
	}
	defer rows.Close()

	items := make([]ReminderDueItem, 0)
	for rows.Next() {
		var item ReminderDueItem
		if err := rows.Scan(&item.ID, &item.TelegramUserID, &item.FirstName, &item.Text, &item.ScheduleType); err != nil {
			return nil, fmt.Errorf("scan due one-time reminder: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due one-time reminders: %w", err)
	}

	return items, nil
}

func (r *ReminderRepository) ListDueDailyReminders(ctx context.Context, scheduleValue string, dispatchDate string) ([]ReminderDueItem, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT r.id, u.telegram_user_id, u.first_name, r.text, r.schedule_type
		FROM reminders r
		JOIN users u ON u.id = r.user_id
		WHERE r.is_active = 1
		  AND r.schedule_type = 'daily'
		  AND r.schedule_value = ?
		  AND NOT EXISTS (
			SELECT 1
			FROM reminder_dispatches d
			WHERE d.reminder_id = r.id
			  AND d.dispatch_date = ?
		  )
	`, scheduleValue, dispatchDate)
	if err != nil {
		return nil, fmt.Errorf("query due daily reminders: %w", err)
	}
	defer rows.Close()

	items := make([]ReminderDueItem, 0)
	for rows.Next() {
		var item ReminderDueItem
		if err := rows.Scan(&item.ID, &item.TelegramUserID, &item.FirstName, &item.Text, &item.ScheduleType); err != nil {
			return nil, fmt.Errorf("scan due daily reminder: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due daily reminders: %w", err)
	}

	return items, nil
}

func (r *ReminderRepository) MarkReminderDispatched(ctx context.Context, reminderID int64, dispatchDate string, deactivate bool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin reminder dispatch transaction: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO reminder_dispatches (reminder_id, dispatch_date)
		VALUES (?, ?)
		ON CONFLICT(reminder_id, dispatch_date) DO NOTHING
	`, reminderID, dispatchDate); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("insert reminder dispatch record: %w", err)
	}

	if deactivate {
		if _, err := tx.ExecContext(ctx, `
			UPDATE reminders
			SET is_active = 0
			WHERE id = ?
		`, reminderID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("deactivate one-time reminder: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reminder dispatch transaction: %w", err)
	}

	return nil
}

func (r *ReminderRepository) DeactivateReminder(ctx context.Context, reminderID int64) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE reminders
		SET is_active = 0
		WHERE id = ? AND is_active = 1
	`, reminderID)
	if err != nil {
		return fmt.Errorf("deactivate reminder: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deactivated reminder rows: %w", err)
	}
	if rowsAffected == 0 {
		return ErrReminderNotFound
	}

	return nil
}

func (r *ReminderRepository) ensureUser(ctx context.Context, telegramUserID int64, _ string) (int64, error) {
	userID, err := userIDByTelegramUserID(ctx, r.db, telegramUserID)
	if err != nil {
		return 0, fmt.Errorf("fetch user: %w", err)
	}

	return userID, nil
}
