package scheduler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"fambow/internal/service"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type schedulerSenderSpy struct {
	messages                []string
	photos                  []string
	captions                []string
	failCaptionedPhotoFirst bool
}

func (s *schedulerSenderSpy) SendMessage(_ context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	s.messages = append(s.messages, params.Text)
	return &models.Message{ID: 1}, nil
}

func (s *schedulerSenderSpy) SendPhoto(_ context.Context, params *bot.SendPhotoParams) (*models.Message, error) {
	if s.failCaptionedPhotoFirst && params.Caption != "" {
		s.failCaptionedPhotoFirst = false
		return nil, errors.New("caption too long")
	}

	if input, ok := params.Photo.(*models.InputFileString); ok {
		s.photos = append(s.photos, input.Data)
	}
	s.captions = append(s.captions, params.Caption)
	return &models.Message{ID: 1}, nil
}

type loveNoteSchedulerServiceSpy struct {
	dueItems []service.PendingLoveNoteSchedule
	dueErr   error
	marked   []int64
}

func (s *loveNoteSchedulerServiceSpy) DueLoveNoteSchedules(context.Context, time.Time) ([]service.PendingLoveNoteSchedule, error) {
	return s.dueItems, s.dueErr
}

func (s *loveNoteSchedulerServiceSpy) MarkLoveNoteScheduleDispatched(_ context.Context, scheduleID int64, _ time.Time) error {
	s.marked = append(s.marked, scheduleID)
	return nil
}

type reminderSchedulerStub struct{}

func (reminderSchedulerStub) DueReminders(context.Context, time.Time) ([]service.PendingReminder, error) {
	return nil, nil
}

func (reminderSchedulerStub) MarkReminderDispatched(context.Context, int64, time.Time, bool) error {
	return nil
}

type celebrationSchedulerStub struct{}

func (celebrationSchedulerStub) DueCelebrationReminders(context.Context, time.Time) ([]service.PendingCelebrationReminder, error) {
	return nil, nil
}

func (celebrationSchedulerStub) MarkCelebrationReminderDispatched(context.Context, int64, time.Time) error {
	return nil
}

type loveNoteProviderSpy struct {
	note service.LoveNote
	err  error
}

func (s loveNoteProviderSpy) RandomNote(context.Context, string) (service.LoveNote, error) {
	return s.note, s.err
}

func TestCronSchedulerDispatchLoveNoteSchedulesText(t *testing.T) {
	sender := &schedulerSenderSpy{}
	schedules := &loveNoteSchedulerServiceSpy{
		dueItems: []service.PendingLoveNoteSchedule{{
			ID:             5,
			TelegramUserID: 123,
			FirstName:      "Mia",
		}},
	}
	s := &CronScheduler{
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		sender:        sender,
		loveNotes:     loveNoteProviderSpy{note: service.LoveNote{Text: "Hello Mia"}},
		loveSchedules: schedules,
		reminders:     reminderSchedulerStub{},
		celebrations:  celebrationSchedulerStub{},
	}

	s.dispatchLoveNoteSchedules(context.Background(), time.Date(2026, time.March, 22, 8, 30, 0, 0, time.Local))

	if len(sender.messages) != 1 || sender.messages[0] != "Hello Mia" {
		t.Fatalf("unexpected sent messages: %#v", sender.messages)
	}
	if len(schedules.marked) != 1 || schedules.marked[0] != 5 {
		t.Fatalf("unexpected marked schedules: %#v", schedules.marked)
	}
}

func TestCronSchedulerDispatchLoveNoteSchedulesEmptyState(t *testing.T) {
	sender := &schedulerSenderSpy{}
	schedules := &loveNoteSchedulerServiceSpy{
		dueItems: []service.PendingLoveNoteSchedule{{
			ID:             6,
			TelegramUserID: 123,
			FirstName:      "Mia",
		}},
	}
	s := &CronScheduler{
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		sender:        sender,
		loveNotes:     loveNoteProviderSpy{err: service.ErrLoveNotesEmpty},
		loveSchedules: schedules,
		reminders:     reminderSchedulerStub{},
		celebrations:  celebrationSchedulerStub{},
	}

	s.dispatchLoveNoteSchedules(context.Background(), time.Date(2026, time.March, 22, 8, 30, 0, 0, time.Local))

	if len(sender.messages) != 1 || sender.messages[0] != "No love notes yet. Add one with /add_love." {
		t.Fatalf("unexpected empty-state messages: %#v", sender.messages)
	}
	if len(schedules.marked) != 1 || schedules.marked[0] != 6 {
		t.Fatalf("unexpected marked schedules: %#v", schedules.marked)
	}
}

func TestCronSchedulerDispatchLoveNoteSchedulesPhotoFallback(t *testing.T) {
	sender := &schedulerSenderSpy{failCaptionedPhotoFirst: true}
	schedules := &loveNoteSchedulerServiceSpy{
		dueItems: []service.PendingLoveNoteSchedule{{
			ID:             8,
			TelegramUserID: 456,
			FirstName:      "Anna",
		}},
	}
	s := &CronScheduler{
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		sender:        sender,
		loveNotes:     loveNoteProviderSpy{note: service.LoveNote{Text: "Photo caption", TelegramFileID: "photo-1"}},
		loveSchedules: schedules,
		reminders:     reminderSchedulerStub{},
		celebrations:  celebrationSchedulerStub{},
	}

	s.dispatchLoveNoteSchedules(context.Background(), time.Date(2026, time.March, 22, 8, 30, 0, 0, time.Local))

	if len(sender.photos) != 1 || sender.photos[0] != "photo-1" {
		t.Fatalf("unexpected sent photos: %#v", sender.photos)
	}
	if len(sender.messages) != 1 || sender.messages[0] != "Photo caption" {
		t.Fatalf("unexpected fallback messages: %#v", sender.messages)
	}
	if len(schedules.marked) != 1 || schedules.marked[0] != 8 {
		t.Fatalf("unexpected marked schedules: %#v", schedules.marked)
	}
}

func TestCronSchedulerDispatchLoveNoteSchedulesDoesNotMarkOnSendFailure(t *testing.T) {
	schedules := &loveNoteSchedulerServiceSpy{
		dueItems: []service.PendingLoveNoteSchedule{{
			ID:             9,
			TelegramUserID: 789,
			FirstName:      "Anna",
		}},
	}
	s := &CronScheduler{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		sender: failingSchedulerSender{},
		loveNotes: loveNoteProviderSpy{
			note: service.LoveNote{Text: "No send"},
		},
		loveSchedules: schedules,
	}

	s.dispatchLoveNoteSchedules(context.Background(), time.Date(2026, time.March, 22, 8, 30, 0, 0, time.Local))

	if len(schedules.marked) != 0 {
		t.Fatalf("expected no marked schedules, got %#v", schedules.marked)
	}
}

type failingSchedulerSender struct{}

func (failingSchedulerSender) SendMessage(context.Context, *bot.SendMessageParams) (*models.Message, error) {
	return nil, errors.New("boom")
}

func (failingSchedulerSender) SendPhoto(context.Context, *bot.SendPhotoParams) (*models.Message, error) {
	return nil, errors.New("boom")
}
