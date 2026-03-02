package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fambow/internal/repository"
)

var (
	ErrEventCommandEmpty   = errors.New("event command cannot be empty")
	ErrEventInvalidFormat  = errors.New("invalid event format")
	ErrEventDateFormat     = errors.New("invalid event date format")
	ErrEventTitleEmpty     = errors.New("event title cannot be empty")
	ErrEventDaysBefore     = errors.New("invalid remind days before value")
	ErrEventNotAddCommand  = errors.New("only add event command is supported")
	ErrEventDateInPast     = errors.New("event date cannot be in the past")
	defaultRemindDaysPrior = 1
)

type CelebrationEvent struct {
	ID               int64
	Title            string
	EventDate        time.Time
	RemindDaysBefore int
}

type PendingCelebrationReminder struct {
	EventID         int64
	TelegramUserID  int64
	FirstName       string
	Title           string
	EventDate       time.Time
	DaysUntilEvent  int
	RemindDate      string
	RemindDaysPrior int
}

type CelebrationStore interface {
	SaveEvent(ctx context.Context, telegramUserID int64, firstName string, title string, eventDate string, remindDaysBefore int) (repository.Event, error)
	ListEventsByTelegramUser(ctx context.Context, telegramUserID int64) ([]repository.Event, error)
	ListDueEventReminders(ctx context.Context, dispatchDate string) ([]repository.DueEventReminder, error)
	MarkEventReminderDispatched(ctx context.Context, eventID int64, dispatchDate string) error
}

type CelebrationService struct {
	store CelebrationStore
}

func NewCelebrationService(store CelebrationStore) *CelebrationService {
	return &CelebrationService{store: store}
}

func (s *CelebrationService) AddEvent(ctx context.Context, telegramUserID int64, firstName string, command string) (CelebrationEvent, error) {
	payload := strings.TrimSpace(command)
	if payload == "" {
		return CelebrationEvent{}, ErrEventCommandEmpty
	}

	title, eventDate, remindDaysBefore, err := parseEventPayload(payload, time.Now())
	if err != nil {
		return CelebrationEvent{}, err
	}

	record, err := s.store.SaveEvent(ctx, telegramUserID, strings.TrimSpace(firstName), title, eventDate.Format(dateLayout), remindDaysBefore)
	if err != nil {
		return CelebrationEvent{}, err
	}

	return CelebrationEvent{
		ID:               record.ID,
		Title:            record.Title,
		EventDate:        record.EventDate,
		RemindDaysBefore: record.RemindDaysBefore,
	}, nil
}

func (s *CelebrationService) ListEvents(ctx context.Context, telegramUserID int64) ([]CelebrationEvent, error) {
	records, err := s.store.ListEventsByTelegramUser(ctx, telegramUserID)
	if err != nil {
		return nil, err
	}

	events := make([]CelebrationEvent, 0, len(records))
	for _, record := range records {
		events = append(events, CelebrationEvent{
			ID:               record.ID,
			Title:            record.Title,
			EventDate:        record.EventDate,
			RemindDaysBefore: record.RemindDaysBefore,
		})
	}

	return events, nil
}

func (s *CelebrationService) DueCelebrationReminders(ctx context.Context, now time.Time) ([]PendingCelebrationReminder, error) {
	items, err := s.store.ListDueEventReminders(ctx, now.Local().Format(dateLayout))
	if err != nil {
		return nil, err
	}

	pending := make([]PendingCelebrationReminder, 0, len(items))
	for _, item := range items {
		pending = append(pending, PendingCelebrationReminder{
			EventID:         item.EventID,
			TelegramUserID:  item.TelegramUserID,
			FirstName:       item.FirstName,
			Title:           item.Title,
			EventDate:       item.EventDate,
			DaysUntilEvent:  item.DaysUntilEvent,
			RemindDate:      item.RemindDate,
			RemindDaysPrior: item.RemindDaysPrior,
		})
	}

	return pending, nil
}

func (s *CelebrationService) MarkCelebrationReminderDispatched(ctx context.Context, eventID int64, now time.Time) error {
	return s.store.MarkEventReminderDispatched(ctx, eventID, now.Local().Format(dateLayout))
}

func parseEventPayload(payload string, now time.Time) (string, time.Time, int, error) {
	if !strings.HasPrefix(strings.ToLower(payload), "add ") {
		return "", time.Time{}, 0, ErrEventNotAddCommand
	}

	body := strings.TrimSpace(strings.TrimSpace(payload[3:]))
	if body == "" {
		return "", time.Time{}, 0, ErrEventInvalidFormat
	}

	parts := strings.Split(body, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	if len(parts) < 2 {
		return "", time.Time{}, 0, ErrEventInvalidFormat
	}

	eventDate, err := time.ParseInLocation(dateLayout, parts[0], now.Location())
	if err != nil {
		return "", time.Time{}, 0, ErrEventDateFormat
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if eventDate.Before(today) {
		return "", time.Time{}, 0, ErrEventDateInPast
	}

	title := strings.TrimSpace(parts[1])
	if title == "" {
		return "", time.Time{}, 0, ErrEventTitleEmpty
	}

	remindDaysBefore := defaultRemindDaysPrior
	if len(parts) >= 3 && parts[2] != "" {
		remindDaysBefore = ParsePositiveInt(parts[2], 0)
		if remindDaysBefore <= 0 {
			return "", time.Time{}, 0, ErrEventDaysBefore
		}
	}

	return title, eventDate, remindDaysBefore, nil
}

func EventUsage() string {
	lines := []string{
		"Use this format:",
		"/event add 2026-09-12 | Anniversary dinner | 3",
		"/event add 2026-03-20 | Mom birthday",
		"Tip: third value is optional days-before reminder.",
	}

	return strings.Join(lines, "\n")
}

func FormatCelebrationReminderMessage(item PendingCelebrationReminder) string {
	if item.DaysUntilEvent <= 0 {
		return fmt.Sprintf("Today is %s. Happy celebration day!", item.Title)
	}

	if item.DaysUntilEvent == 1 {
		return fmt.Sprintf("Tomorrow is %s. A sweet reminder to get ready.", item.Title)
	}

	return fmt.Sprintf("%s is in %d days (%s).", item.Title, item.DaysUntilEvent, item.EventDate.Format(dateLayout))
}
