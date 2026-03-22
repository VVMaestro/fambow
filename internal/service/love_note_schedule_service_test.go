package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"fambow/internal/repository"
)

type loveNoteScheduleStoreSpy struct {
	saveTelegramUserID int64
	saveScheduleTime   string
	saveResult         repository.LoveNoteScheduleItem
	saveErr            error

	listResult []repository.LoveNoteScheduleItem
	listErr    error

	deactivateID  int64
	deactivateErr error

	dueTime   string
	dueDate   string
	dueResult []repository.LoveNoteScheduleItem
	dueErr    error

	dispatchID   int64
	dispatchDate string
	dispatchErr  error
}

func (s *loveNoteScheduleStoreSpy) SaveLoveNoteSchedule(_ context.Context, telegramUserID int64, scheduleTime string) (repository.LoveNoteScheduleItem, error) {
	s.saveTelegramUserID = telegramUserID
	s.saveScheduleTime = scheduleTime
	return s.saveResult, s.saveErr
}

func (s *loveNoteScheduleStoreSpy) ListActiveLoveNoteSchedules(context.Context) ([]repository.LoveNoteScheduleItem, error) {
	return s.listResult, s.listErr
}

func (s *loveNoteScheduleStoreSpy) DeactivateLoveNoteSchedule(_ context.Context, scheduleID int64) error {
	s.deactivateID = scheduleID
	return s.deactivateErr
}

func (s *loveNoteScheduleStoreSpy) ListDueLoveNoteSchedules(_ context.Context, scheduleTime string, dispatchDate string) ([]repository.LoveNoteScheduleItem, error) {
	s.dueTime = scheduleTime
	s.dueDate = dispatchDate
	return s.dueResult, s.dueErr
}

func (s *loveNoteScheduleStoreSpy) MarkLoveNoteScheduleDispatched(_ context.Context, scheduleID int64, dispatchDate string) error {
	s.dispatchID = scheduleID
	s.dispatchDate = dispatchDate
	return s.dispatchErr
}

func TestLoveNoteScheduleServiceAddLoveNoteSchedule(t *testing.T) {
	store := &loveNoteScheduleStoreSpy{
		saveResult: repository.LoveNoteScheduleItem{
			ID:             7,
			TelegramUserID: 42,
			FirstName:      "Mia",
			UserType:       "wife",
			ScheduleTime:   "08:30",
		},
	}
	svc := NewLoveNoteScheduleService(store)

	schedule, err := svc.AddLoveNoteSchedule(context.Background(), 42, " 08:30 ")
	if err != nil {
		t.Fatalf("AddLoveNoteSchedule() unexpected error: %v", err)
	}

	if store.saveTelegramUserID != 42 || store.saveScheduleTime != "08:30" {
		t.Fatalf("unexpected save args: user=%d time=%q", store.saveTelegramUserID, store.saveScheduleTime)
	}
	if schedule.ScheduleDisplay != "at 08:30" || schedule.FirstName != "Mia" || schedule.UserType != "wife" {
		t.Fatalf("unexpected schedule result: %#v", schedule)
	}
}

func TestLoveNoteScheduleServiceAddLoveNoteScheduleRejectsInvalidTime(t *testing.T) {
	svc := NewLoveNoteScheduleService(&loveNoteScheduleStoreSpy{})

	_, err := svc.AddLoveNoteSchedule(context.Background(), 42, "25:99")
	if !errors.Is(err, ErrLoveNoteScheduleTimeFormat) {
		t.Fatalf("expected ErrLoveNoteScheduleTimeFormat, got %v", err)
	}
}

func TestLoveNoteScheduleServiceListFormatsSchedules(t *testing.T) {
	store := &loveNoteScheduleStoreSpy{
		listResult: []repository.LoveNoteScheduleItem{{
			ID:             3,
			TelegramUserID: 55,
			FirstName:      "Mia",
			UserType:       "wife",
			ScheduleTime:   "08:30",
		}},
	}
	svc := NewLoveNoteScheduleService(store)

	items, err := svc.ListLoveNoteSchedules(context.Background())
	if err != nil {
		t.Fatalf("ListLoveNoteSchedules() unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 schedule, got %d", len(items))
	}
	if items[0].ScheduleDisplay != "at 08:30" || items[0].ID != 3 {
		t.Fatalf("unexpected listed schedule: %#v", items[0])
	}
}

func TestLoveNoteScheduleServiceRemoveMapsNotFound(t *testing.T) {
	store := &loveNoteScheduleStoreSpy{deactivateErr: repository.ErrLoveNoteScheduleNotFound}
	svc := NewLoveNoteScheduleService(store)

	err := svc.RemoveLoveNoteSchedule(context.Background(), 9)
	if !errors.Is(err, ErrLoveNoteScheduleNotFound) {
		t.Fatalf("expected ErrLoveNoteScheduleNotFound, got %v", err)
	}
}

func TestLoveNoteScheduleServiceDueAndDispatchUseLocalMinute(t *testing.T) {
	store := &loveNoteScheduleStoreSpy{
		dueResult: []repository.LoveNoteScheduleItem{{
			ID:             11,
			TelegramUserID: 77,
			FirstName:      "Anna",
			UserType:       "wife",
			ScheduleTime:   "19:30",
		}},
	}
	svc := NewLoveNoteScheduleService(store)
	now := time.Date(2026, time.March, 22, 19, 30, 5, 0, time.Local)

	items, err := svc.DueLoveNoteSchedules(context.Background(), now)
	if err != nil {
		t.Fatalf("DueLoveNoteSchedules() unexpected error: %v", err)
	}
	if store.dueTime != "19:30" || store.dueDate != "2026-03-22" {
		t.Fatalf("unexpected due query args: time=%q date=%q", store.dueTime, store.dueDate)
	}
	if len(items) != 1 || items[0].TelegramUserID != 77 {
		t.Fatalf("unexpected due items: %#v", items)
	}

	if err := svc.MarkLoveNoteScheduleDispatched(context.Background(), 11, now); err != nil {
		t.Fatalf("MarkLoveNoteScheduleDispatched() unexpected error: %v", err)
	}
	if store.dispatchID != 11 || store.dispatchDate != "2026-03-22" {
		t.Fatalf("unexpected dispatch args: id=%d date=%q", store.dispatchID, store.dispatchDate)
	}
}
