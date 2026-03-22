package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"fambow/internal/repository"
)

var ErrLoveNoteScheduleTimeFormat = errors.New("invalid love note schedule time format")
var ErrLoveNoteScheduleTargetNotFound = errors.New("love note schedule target user not found")
var ErrLoveNoteScheduleNotFound = errors.New("love note schedule not found")

type LoveNoteSchedule struct {
	ID              int64
	TelegramUserID  int64
	FirstName       string
	UserType        string
	ScheduleTime    string
	ScheduleDisplay string
}

type PendingLoveNoteSchedule struct {
	ID             int64
	TelegramUserID int64
	FirstName      string
	UserType       string
}

type LoveNoteScheduleStore interface {
	SaveLoveNoteSchedule(ctx context.Context, telegramUserID int64, scheduleTime string) (repository.LoveNoteScheduleItem, error)
	ListActiveLoveNoteSchedules(ctx context.Context) ([]repository.LoveNoteScheduleItem, error)
	DeactivateLoveNoteSchedule(ctx context.Context, scheduleID int64) error
	ListDueLoveNoteSchedules(ctx context.Context, scheduleTime string, dispatchDate string) ([]repository.LoveNoteScheduleItem, error)
	MarkLoveNoteScheduleDispatched(ctx context.Context, scheduleID int64, dispatchDate string) error
}

type LoveNoteScheduleService struct {
	store LoveNoteScheduleStore
}

func NewLoveNoteScheduleService(store LoveNoteScheduleStore) *LoveNoteScheduleService {
	return &LoveNoteScheduleService{store: store}
}

func (s *LoveNoteScheduleService) AddLoveNoteSchedule(ctx context.Context, telegramUserID int64, scheduleTime string) (LoveNoteSchedule, error) {
	normalizedTime, err := parseLoveNoteScheduleTime(scheduleTime)
	if err != nil {
		return LoveNoteSchedule{}, err
	}

	record, err := s.store.SaveLoveNoteSchedule(ctx, telegramUserID, normalizedTime)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return LoveNoteSchedule{}, ErrLoveNoteScheduleTargetNotFound
		}
		return LoveNoteSchedule{}, err
	}

	return mapLoveNoteSchedule(record), nil
}

func (s *LoveNoteScheduleService) ListLoveNoteSchedules(ctx context.Context) ([]LoveNoteSchedule, error) {
	records, err := s.store.ListActiveLoveNoteSchedules(ctx)
	if err != nil {
		return nil, err
	}

	schedules := make([]LoveNoteSchedule, 0, len(records))
	for _, record := range records {
		schedules = append(schedules, mapLoveNoteSchedule(record))
	}

	return schedules, nil
}

func (s *LoveNoteScheduleService) RemoveLoveNoteSchedule(ctx context.Context, scheduleID int64) error {
	if err := s.store.DeactivateLoveNoteSchedule(ctx, scheduleID); err != nil {
		if errors.Is(err, repository.ErrLoveNoteScheduleNotFound) {
			return ErrLoveNoteScheduleNotFound
		}
		return err
	}

	return nil
}

func (s *LoveNoteScheduleService) DueLoveNoteSchedules(ctx context.Context, now time.Time) ([]PendingLoveNoteSchedule, error) {
	nowLocal := now.Local()
	records, err := s.store.ListDueLoveNoteSchedules(ctx, nowLocal.Format(timeLayout), nowLocal.Format(dateLayout))
	if err != nil {
		return nil, err
	}

	items := make([]PendingLoveNoteSchedule, 0, len(records))
	for _, record := range records {
		items = append(items, PendingLoveNoteSchedule{
			ID:             record.ID,
			TelegramUserID: record.TelegramUserID,
			FirstName:      record.FirstName,
			UserType:       record.UserType,
		})
	}

	return items, nil
}

func (s *LoveNoteScheduleService) MarkLoveNoteScheduleDispatched(ctx context.Context, scheduleID int64, now time.Time) error {
	return s.store.MarkLoveNoteScheduleDispatched(ctx, scheduleID, now.Local().Format(dateLayout))
}

func parseLoveNoteScheduleTime(scheduleTime string) (string, error) {
	trimmed := strings.TrimSpace(scheduleTime)
	if trimmed == "" {
		return "", ErrLoveNoteScheduleTimeFormat
	}

	if _, err := time.Parse(timeLayout, trimmed); err != nil {
		return "", ErrLoveNoteScheduleTimeFormat
	}

	return trimmed, nil
}

func mapLoveNoteSchedule(record repository.LoveNoteScheduleItem) LoveNoteSchedule {
	return LoveNoteSchedule{
		ID:              record.ID,
		TelegramUserID:  record.TelegramUserID,
		FirstName:       record.FirstName,
		UserType:        record.UserType,
		ScheduleTime:    record.ScheduleTime,
		ScheduleDisplay: "at " + record.ScheduleTime,
	}
}
