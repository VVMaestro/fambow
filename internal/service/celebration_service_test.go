package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"fambow/internal/repository"
)

type celebrationStoreSpy struct {
	saveTelegramUserID int64
	saveFirstName      string
	saveTitle          string
	saveEventDate      string
	saveRemindDays     int
	saveResult         repository.Event
	saveErr            error
}

func (s *celebrationStoreSpy) SaveEvent(_ context.Context, telegramUserID int64, firstName string, title string, eventDate string, remindDaysBefore int) (repository.Event, error) {
	s.saveTelegramUserID = telegramUserID
	s.saveFirstName = firstName
	s.saveTitle = title
	s.saveEventDate = eventDate
	s.saveRemindDays = remindDaysBefore
	return s.saveResult, s.saveErr
}

func (s *celebrationStoreSpy) ListEventsByTelegramUser(context.Context, int64) ([]repository.Event, error) {
	return nil, nil
}

func (s *celebrationStoreSpy) ListDueEventReminders(context.Context, string) ([]repository.DueEventReminder, error) {
	return nil, nil
}

func (s *celebrationStoreSpy) MarkEventReminderDispatched(context.Context, int64, string) error {
	return nil
}

func TestParseEventPayload(t *testing.T) {
	now := time.Date(2026, time.March, 21, 10, 0, 0, 0, time.Local)

	tests := []struct {
		name      string
		payload   string
		wantTitle string
		wantDate  string
		wantDays  int
		wantErr   error
	}{
		{
			name:      "valid event with explicit days",
			payload:   "add 2026-09-12 | Anniversary dinner | 3",
			wantTitle: "Anniversary dinner",
			wantDate:  "2026-09-12",
			wantDays:  3,
		},
		{
			name:      "valid event with default days",
			payload:   "add 2026-03-22 | Mom birthday",
			wantTitle: "Mom birthday",
			wantDate:  "2026-03-22",
			wantDays:  defaultRemindDaysPrior,
		},
		{
			name:    "requires add subcommand",
			payload: "list 2026-03-22 | Mom birthday",
			wantErr: ErrEventNotAddCommand,
		},
		{
			name:    "rejects past date",
			payload: "add 2026-03-20 | Mom birthday",
			wantErr: ErrEventDateInPast,
		},
		{
			name:    "rejects missing title",
			payload: "add 2026-03-22 |   ",
			wantErr: ErrEventTitleEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, eventDate, remindDays, err := parseEventPayload(tt.payload, now)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
			if tt.wantErr != nil {
				return
			}
			if title != tt.wantTitle || eventDate.Format(dateLayout) != tt.wantDate || remindDays != tt.wantDays {
				t.Fatalf("unexpected event parse result: title=%q date=%q days=%d", title, eventDate.Format(dateLayout), remindDays)
			}
		})
	}
}

func TestCelebrationServiceAddEventUsesParsedPayload(t *testing.T) {
	store := &celebrationStoreSpy{
		saveResult: repository.Event{
			ID:               1,
			Title:            "Anniversary dinner",
			EventDate:        time.Date(2026, time.September, 12, 0, 0, 0, 0, time.Local),
			RemindDaysBefore: 3,
		},
	}
	svc := NewCelebrationService(store)

	event, err := svc.AddEvent(context.Background(), 22, " Anna ", "add 2026-09-12 | Anniversary dinner | 3")
	if err != nil {
		t.Fatalf("AddEvent() unexpected error: %v", err)
	}

	if store.saveTelegramUserID != 22 || store.saveFirstName != "Anna" {
		t.Fatalf("unexpected SaveEvent user inputs: id=%d name=%q", store.saveTelegramUserID, store.saveFirstName)
	}
	if store.saveTitle != "Anniversary dinner" || store.saveEventDate != "2026-09-12" || store.saveRemindDays != 3 {
		t.Fatalf("unexpected SaveEvent payload: title=%q date=%q days=%d", store.saveTitle, store.saveEventDate, store.saveRemindDays)
	}
	if event.Title != "Anniversary dinner" || event.RemindDaysBefore != 3 {
		t.Fatalf("unexpected returned event: %#v", event)
	}
}
