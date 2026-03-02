package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"fambow/internal/repository"
)

const (
	reminderScheduleOneTime = "one_time"
	reminderScheduleDaily   = "daily"
	dateLayout              = "2006-01-02"
	timeLayout              = "15:04"
	dbDateTimeLayout        = "2006-01-02 15:04:05"
)

var (
	ErrReminderCommandEmpty   = errors.New("reminder command cannot be empty")
	ErrReminderInvalidFormat  = errors.New("invalid reminder format")
	ErrReminderTimeFormat     = errors.New("invalid time format")
	ErrReminderDateTimeFormat = errors.New("invalid datetime format")
	ErrReminderTimeInPast     = errors.New("reminder time is in the past")
	ErrReminderTextEmpty      = errors.New("reminder text cannot be empty")
)

type Reminder struct {
	ID              int64
	Text            string
	ScheduleType    string
	ScheduleDisplay string
}

type PendingReminder struct {
	ID             int64
	TelegramUserID int64
	FirstName      string
	Text           string
	ScheduleType   string
}

type ReminderStore interface {
	SaveReminder(ctx context.Context, telegramUserID int64, firstName string, text string, scheduleType string, scheduleValue string) (repository.Reminder, error)
	ListActiveReminders(ctx context.Context, telegramUserID int64) ([]repository.Reminder, error)
	ListDueOneTimeReminders(ctx context.Context, nowTimestamp string) ([]repository.ReminderDueItem, error)
	ListDueDailyReminders(ctx context.Context, scheduleValue string, dispatchDate string) ([]repository.ReminderDueItem, error)
	MarkReminderDispatched(ctx context.Context, reminderID int64, dispatchDate string, deactivate bool) error
}

type ReminderService struct {
	store ReminderStore
}

func NewReminderService(store ReminderStore) *ReminderService {
	return &ReminderService{store: store}
}

func (s *ReminderService) AddReminder(ctx context.Context, telegramUserID int64, firstName string, command string) (Reminder, error) {
	payload := strings.TrimSpace(command)
	if payload == "" {
		return Reminder{}, ErrReminderCommandEmpty
	}

	scheduleType, scheduleValue, text, err := parseReminderPayload(payload, time.Now())
	if err != nil {
		return Reminder{}, err
	}

	record, err := s.store.SaveReminder(ctx, telegramUserID, strings.TrimSpace(firstName), text, scheduleType, scheduleValue)
	if err != nil {
		return Reminder{}, err
	}

	return Reminder{
		ID:              record.ID,
		Text:            record.Text,
		ScheduleType:    record.ScheduleType,
		ScheduleDisplay: formatReminderSchedule(record.ScheduleType, record.ScheduleValue),
	}, nil
}

func (s *ReminderService) ListReminders(ctx context.Context, telegramUserID int64) ([]Reminder, error) {
	records, err := s.store.ListActiveReminders(ctx, telegramUserID)
	if err != nil {
		return nil, err
	}

	reminders := make([]Reminder, 0, len(records))
	for _, record := range records {
		reminders = append(reminders, Reminder{
			ID:              record.ID,
			Text:            record.Text,
			ScheduleType:    record.ScheduleType,
			ScheduleDisplay: formatReminderSchedule(record.ScheduleType, record.ScheduleValue),
		})
	}

	return reminders, nil
}

func (s *ReminderService) DueReminders(ctx context.Context, now time.Time) ([]PendingReminder, error) {
	nowLocal := now.Local()

	oneTime, err := s.store.ListDueOneTimeReminders(ctx, nowLocal.Format(dbDateTimeLayout))
	if err != nil {
		return nil, err
	}

	daily, err := s.store.ListDueDailyReminders(ctx, nowLocal.Format(timeLayout), nowLocal.Format(dateLayout))
	if err != nil {
		return nil, err
	}

	items := make([]PendingReminder, 0, len(oneTime)+len(daily))
	for _, item := range oneTime {
		items = append(items, PendingReminder{
			ID:             item.ID,
			TelegramUserID: item.TelegramUserID,
			FirstName:      item.FirstName,
			Text:           item.Text,
			ScheduleType:   item.ScheduleType,
		})
	}

	for _, item := range daily {
		items = append(items, PendingReminder{
			ID:             item.ID,
			TelegramUserID: item.TelegramUserID,
			FirstName:      item.FirstName,
			Text:           item.Text,
			ScheduleType:   item.ScheduleType,
		})
	}

	return items, nil
}

func (s *ReminderService) MarkReminderDispatched(ctx context.Context, reminderID int64, now time.Time, deactivate bool) error {
	return s.store.MarkReminderDispatched(ctx, reminderID, now.Local().Format(dateLayout), deactivate)
}

func parseReminderPayload(payload string, now time.Time) (string, string, string, error) {
	parts := strings.Fields(payload)
	if len(parts) < 2 {
		return "", "", "", ErrReminderInvalidFormat
	}

	mode := strings.ToLower(parts[0])
	if mode == "daily" {
		timeValue := strings.TrimSpace(parts[1])
		if _, err := time.Parse(timeLayout, timeValue); err != nil {
			return "", "", "", ErrReminderTimeFormat
		}

		text := strings.TrimSpace(strings.Join(parts[2:], " "))
		if text == "" {
			return "", "", "", ErrReminderTextEmpty
		}

		return reminderScheduleDaily, timeValue, text, nil
	}

	if mode == "at" {
		if len(parts) < 3 {
			return "", "", "", ErrReminderInvalidFormat
		}

		timeToken := parts[1]
		textStart := 2
		when := time.Time{}

		if len(parts) >= 4 {
			dt, err := time.ParseInLocation("2006-01-02 15:04", parts[1]+" "+parts[2], now.Location())
			if err == nil {
				if !dt.After(now) {
					return "", "", "", ErrReminderTimeInPast
				}
				when = dt
				textStart = 3
			}
		}

		if when.IsZero() {
			onlyTime, err := time.ParseInLocation(timeLayout, timeToken, now.Location())
			if err != nil {
				return "", "", "", ErrReminderDateTimeFormat
			}

			when = time.Date(now.Year(), now.Month(), now.Day(), onlyTime.Hour(), onlyTime.Minute(), 0, 0, now.Location())
			if !when.After(now) {
				when = when.Add(24 * time.Hour)
			}
		}

		text := strings.TrimSpace(strings.Join(parts[textStart:], " "))
		if text == "" {
			return "", "", "", ErrReminderTextEmpty
		}

		return reminderScheduleOneTime, when.Format(dbDateTimeLayout), text, nil
	}

	if mode == "me" && len(parts) >= 5 && strings.EqualFold(parts[1], "at") {
		t, err := time.ParseInLocation(timeLayout, parts[2], now.Location())
		if err != nil {
			return "", "", "", ErrReminderDateTimeFormat
		}

		if !strings.EqualFold(parts[3], "to") {
			return "", "", "", ErrReminderInvalidFormat
		}

		text := strings.TrimSpace(strings.Join(parts[4:], " "))
		if text == "" {
			return "", "", "", ErrReminderTextEmpty
		}

		when := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
		if !when.After(now) {
			when = when.Add(24 * time.Hour)
		}

		return reminderScheduleOneTime, when.Format(dbDateTimeLayout), text, nil
	}

	if mode == "once" {
		if len(parts) < 4 {
			return "", "", "", ErrReminderInvalidFormat
		}

		dt, err := time.ParseInLocation("2006-01-02 15:04", parts[1]+" "+parts[2], now.Location())
		if err != nil {
			return "", "", "", ErrReminderDateTimeFormat
		}
		if !dt.After(now) {
			return "", "", "", ErrReminderTimeInPast
		}

		text := strings.TrimSpace(strings.Join(parts[3:], " "))
		if text == "" {
			return "", "", "", ErrReminderTextEmpty
		}

		return reminderScheduleOneTime, dt.Format(dbDateTimeLayout), text, nil
	}

	return "", "", "", ErrReminderInvalidFormat
}

func formatReminderSchedule(scheduleType string, scheduleValue string) string {
	if scheduleType == reminderScheduleDaily {
		return "Daily at " + scheduleValue
	}

	if scheduleType == reminderScheduleOneTime {
		timeValue, err := time.ParseInLocation(dbDateTimeLayout, scheduleValue, time.Local)
		if err == nil {
			return "Once at " + timeValue.Format("2006-01-02 15:04")
		}

		return "Once at " + scheduleValue
	}

	return fmt.Sprintf("%s (%s)", scheduleType, scheduleValue)
}

func ReminderUsage() string {
	lines := []string{
		"Use one of these formats:",
		"/remind at 19:30 drink tea",
		"/remind at 2026-03-15 19:30 call mom",
		"/remind daily 08:00 vitamins",
		"/remind me at 19:30 to drink tea",
		"/remind once 2026-03-15 19:30 book tickets",
	}

	return strings.Join(lines, "\n")
}

func ParsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}

	return parsed
}
