package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"fambow/internal/repository"
)

type reminderStoreSpy struct {
	saveTelegramUserID int64
	saveFirstName      string
	saveText           string
	saveScheduleType   string
	saveScheduleValue  string
	saveResult         repository.Reminder
	saveErr            error

	targetUserType     string
	targetText         string
	targetScheduleType string
	targetScheduleVal  string
	targetResult       repository.Reminder
	targetErr          error

	listTelegramUserID int64
	listResult         []repository.Reminder
	listErr            error
}

func (s *reminderStoreSpy) SaveReminder(_ context.Context, telegramUserID int64, firstName string, text string, scheduleType string, scheduleValue string) (repository.Reminder, error) {
	s.saveTelegramUserID = telegramUserID
	s.saveFirstName = firstName
	s.saveText = text
	s.saveScheduleType = scheduleType
	s.saveScheduleValue = scheduleValue
	return s.saveResult, s.saveErr
}

func (s *reminderStoreSpy) SaveReminderForUserType(_ context.Context, userType string, text string, scheduleType string, scheduleValue string) (repository.Reminder, error) {
	s.targetUserType = userType
	s.targetText = text
	s.targetScheduleType = scheduleType
	s.targetScheduleVal = scheduleValue
	return s.targetResult, s.targetErr
}

func (s *reminderStoreSpy) ListActiveReminders(_ context.Context, telegramUserID int64) ([]repository.Reminder, error) {
	s.listTelegramUserID = telegramUserID
	return s.listResult, s.listErr
}

func (s *reminderStoreSpy) ListDueOneTimeReminders(context.Context, string) ([]repository.ReminderDueItem, error) {
	return nil, nil
}

func (s *reminderStoreSpy) ListDueDailyReminders(context.Context, string, string) ([]repository.ReminderDueItem, error) {
	return nil, nil
}

func (s *reminderStoreSpy) MarkReminderDispatched(context.Context, int64, string, bool) error {
	return nil
}

func TestParseReminderPayload(t *testing.T) {
	now := time.Date(2026, time.March, 21, 10, 0, 0, 0, time.Local)

	tests := []struct {
		name      string
		payload   string
		wantType  string
		wantValue string
		wantText  string
		wantErr   error
	}{
		{
			name:      "daily reminder",
			payload:   "daily 08:00 vitamins",
			wantType:  reminderScheduleDaily,
			wantValue: "08:00",
			wantText:  "vitamins",
		},
		{
			name:      "at time today",
			payload:   "at 19:30 drink tea",
			wantType:  reminderScheduleOneTime,
			wantValue: "2026-03-21 19:30:00",
			wantText:  "drink tea",
		},
		{
			name:      "at time rolls to tomorrow",
			payload:   "at 09:30 drink tea",
			wantType:  reminderScheduleOneTime,
			wantValue: "2026-03-22 09:30:00",
			wantText:  "drink tea",
		},
		{
			name:      "me at syntax",
			payload:   "me at 19:30 to drink tea",
			wantType:  reminderScheduleOneTime,
			wantValue: "2026-03-21 19:30:00",
			wantText:  "drink tea",
		},
		{
			name:      "once explicit date time",
			payload:   "once 2026-03-22 18:45 book tickets",
			wantType:  reminderScheduleOneTime,
			wantValue: "2026-03-22 18:45:00",
			wantText:  "book tickets",
		},
		{
			name:    "invalid daily time",
			payload: "daily nope vitamins",
			wantErr: ErrReminderTimeFormat,
		},
		{
			name:    "once in past",
			payload: "once 2026-03-20 18:45 book tickets",
			wantErr: ErrReminderTimeInPast,
		},
		{
			name:    "missing reminder text",
			payload: "at 19:30",
			wantErr: ErrReminderInvalidFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotValue, gotText, err := parseReminderPayload(tt.payload, now)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
			if tt.wantErr != nil {
				return
			}
			if gotType != tt.wantType || gotValue != tt.wantValue || gotText != tt.wantText {
				t.Fatalf("unexpected parse result: type=%q value=%q text=%q", gotType, gotValue, gotText)
			}
		})
	}
}

func TestReminderServiceAddReminder(t *testing.T) {
	store := &reminderStoreSpy{
		saveResult: repository.Reminder{
			ID:            1,
			Text:          "vitamins",
			ScheduleType:  reminderScheduleDaily,
			ScheduleValue: "08:00",
		},
	}
	svc := NewReminderService(store)

	reminder, err := svc.AddReminder(context.Background(), 11, " Anna ", "daily 08:00 vitamins")
	if err != nil {
		t.Fatalf("AddReminder() unexpected error: %v", err)
	}

	if store.saveTelegramUserID != 11 || store.saveFirstName != "Anna" {
		t.Fatalf("unexpected SaveReminder inputs: id=%d name=%q", store.saveTelegramUserID, store.saveFirstName)
	}
	if store.saveText != "vitamins" || store.saveScheduleType != reminderScheduleDaily || store.saveScheduleValue != "08:00" {
		t.Fatalf("unexpected SaveReminder payload: text=%q type=%q value=%q", store.saveText, store.saveScheduleType, store.saveScheduleValue)
	}
	if reminder.ScheduleDisplay != "Daily at 08:00" {
		t.Fatalf("unexpected ScheduleDisplay: %q", reminder.ScheduleDisplay)
	}
}

func TestReminderServiceMapsRepositoryErrors(t *testing.T) {
	t.Run("sender not found", func(t *testing.T) {
		store := &reminderStoreSpy{saveErr: repository.ErrUserNotFound}
		svc := NewReminderService(store)

		_, err := svc.AddReminder(context.Background(), 11, "Anna", "daily 08:00 vitamins")
		if !errors.Is(err, ErrReminderUserNotFound) {
			t.Fatalf("expected ErrReminderUserNotFound, got %v", err)
		}
	})

	t.Run("target not found", func(t *testing.T) {
		store := &reminderStoreSpy{targetErr: repository.ErrUserTypeNotFound}
		svc := NewReminderService(store)

		_, err := svc.AddReminderForUserType(context.Background(), "wife", "daily 08:00 vitamins")
		if !errors.Is(err, ErrReminderTargetNotFound) {
			t.Fatalf("expected ErrReminderTargetNotFound, got %v", err)
		}
	})
}

func TestReminderServiceListRemindersFormatsDisplay(t *testing.T) {
	store := &reminderStoreSpy{
		listResult: []repository.Reminder{
			{ID: 1, Text: "vitamins", ScheduleType: reminderScheduleDaily, ScheduleValue: "08:00"},
			{ID: 2, Text: "call mom", ScheduleType: reminderScheduleOneTime, ScheduleValue: "2026-03-21 19:30:00"},
		},
	}
	svc := NewReminderService(store)

	reminders, err := svc.ListReminders(context.Background(), 22)
	if err != nil {
		t.Fatalf("ListReminders() unexpected error: %v", err)
	}

	if store.listTelegramUserID != 22 {
		t.Fatalf("expected ListActiveReminders for user 22, got %d", store.listTelegramUserID)
	}
	if len(reminders) != 2 {
		t.Fatalf("expected 2 reminders, got %d", len(reminders))
	}
	if reminders[0].ScheduleDisplay != "Daily at 08:00" {
		t.Fatalf("unexpected first ScheduleDisplay: %q", reminders[0].ScheduleDisplay)
	}
	if reminders[1].ScheduleDisplay != "Once at 2026-03-21 19:30" {
		t.Fatalf("unexpected second ScheduleDisplay: %q", reminders[1].ScheduleDisplay)
	}
}
