package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"fambow/internal/service"
	"fambow/internal/telegram"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/robfig/cron/v3"
)

type MessageSender interface {
	SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)
	SendPhoto(ctx context.Context, params *bot.SendPhotoParams) (*models.Message, error)
}

type LoveNoteSchedulerService interface {
	DueLoveNoteSchedules(ctx context.Context, now time.Time) ([]service.PendingLoveNoteSchedule, error)
	MarkLoveNoteScheduleDispatched(ctx context.Context, scheduleID int64, now time.Time) error
}

type LoveNoteProvider interface {
	RandomNote(ctx context.Context, firstName string) (service.LoveNote, error)
}

type ReminderSchedulerService interface {
	DueReminders(ctx context.Context, now time.Time) ([]service.PendingReminder, error)
	MarkReminderDispatched(ctx context.Context, reminderID int64, now time.Time, deactivate bool) error
}

type CelebrationSchedulerService interface {
	DueCelebrationReminders(ctx context.Context, now time.Time) ([]service.PendingCelebrationReminder, error)
	MarkCelebrationReminderDispatched(ctx context.Context, eventID int64, now time.Time) error
}

type CronScheduler struct {
	logger        *slog.Logger
	sender        MessageSender
	loveNotes     LoveNoteProvider
	loveSchedules LoveNoteSchedulerService
	reminders     ReminderSchedulerService
	celebrations  CelebrationSchedulerService
	cron          *cron.Cron
}

func NewCronScheduler(logger *slog.Logger, sender MessageSender, loveNotes LoveNoteProvider, loveSchedules LoveNoteSchedulerService, reminders ReminderSchedulerService, celebrations CelebrationSchedulerService) (*CronScheduler, error) {
	c := cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))

	scheduler := &CronScheduler{
		logger:        logger,
		sender:        sender,
		loveNotes:     loveNotes,
		loveSchedules: loveSchedules,
		reminders:     reminders,
		celebrations:  celebrations,
		cron:          c,
	}

	if _, err := c.AddFunc("@every 1m", scheduler.runTick); err != nil {
		return nil, fmt.Errorf("register scheduler tick: %w", err)
	}

	return scheduler, nil
}

func (s *CronScheduler) Start(ctx context.Context) {
	if s == nil || s.cron == nil {
		return
	}

	s.logger.Info("starting scheduler")
	s.cron.Start()

	<-ctx.Done()

	shutdownCtx := s.cron.Stop()
	<-shutdownCtx.Done()
	s.logger.Info("scheduler stopped")
}

func (s *CronScheduler) runTick() {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	now := time.Now()
	s.dispatchLoveNoteSchedules(ctx, now)
	s.dispatchReminders(ctx, now)
	s.dispatchCelebrations(ctx, now)
}

func (s *CronScheduler) dispatchLoveNoteSchedules(ctx context.Context, now time.Time) {
	if s.loveSchedules == nil || s.loveNotes == nil || s.sender == nil {
		return
	}

	items, err := s.loveSchedules.DueLoveNoteSchedules(ctx, now)
	if err != nil {
		s.logger.Error("failed loading due love note schedules", "error", err)
		return
	}

	for _, item := range items {
		note, err := s.loveNotes.RandomNote(ctx, item.FirstName)
		if err != nil {
			if errors.Is(err, service.ErrLoveNotesEmpty) {
				if !telegram.SendLoveNotesEmptyState(ctx, s.sender, item.TelegramUserID, nil, s.logger) {
					continue
				}
				if err := s.loveSchedules.MarkLoveNoteScheduleDispatched(ctx, item.ID, now); err != nil {
					s.logger.Error("failed marking love note schedule dispatched", "schedule_id", item.ID, "error", err)
				}
				continue
			}

			s.logger.Error("failed fetching scheduled love note", "schedule_id", item.ID, "chat_id", item.TelegramUserID, "error", err)
			continue
		}

		if !telegram.SendLoveNote(ctx, s.sender, item.TelegramUserID, note, nil, s.logger) {
			continue
		}

		if err := s.loveSchedules.MarkLoveNoteScheduleDispatched(ctx, item.ID, now); err != nil {
			s.logger.Error("failed marking love note schedule dispatched", "schedule_id", item.ID, "error", err)
		}
	}
}

func (s *CronScheduler) dispatchReminders(ctx context.Context, now time.Time) {
	if s.reminders == nil || s.sender == nil {
		return
	}

	items, err := s.reminders.DueReminders(ctx, now)
	if err != nil {
		s.logger.Error("failed loading due reminders", "error", err)
		return
	}

	for _, item := range items {
		message := item.Text
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(message)), "reminder:") {
			message = "Reminder: " + item.Text
		}

		if _, err := s.sender.SendMessage(ctx, &bot.SendMessageParams{ChatID: item.TelegramUserID, Text: message}); err != nil {
			s.logger.Error("failed sending reminder message", "reminder_id", item.ID, "chat_id", item.TelegramUserID, "error", err)
			continue
		}

		deactivate := item.ScheduleType == "one_time"
		if err := s.reminders.MarkReminderDispatched(ctx, item.ID, now, deactivate); err != nil {
			s.logger.Error("failed marking reminder dispatched", "reminder_id", item.ID, "error", err)
		}
	}
}

func (s *CronScheduler) dispatchCelebrations(ctx context.Context, now time.Time) {
	if s.celebrations == nil || s.sender == nil {
		return
	}

	items, err := s.celebrations.DueCelebrationReminders(ctx, now)
	if err != nil {
		s.logger.Error("failed loading due celebration reminders", "error", err)
		return
	}

	for _, item := range items {
		message := service.FormatCelebrationReminderMessage(item)
		if _, err := s.sender.SendMessage(ctx, &bot.SendMessageParams{ChatID: item.TelegramUserID, Text: message}); err != nil {
			s.logger.Error("failed sending celebration reminder", "event_id", item.EventID, "chat_id", item.TelegramUserID, "error", err)
			continue
		}

		if err := s.celebrations.MarkCelebrationReminderDispatched(ctx, item.EventID, now); err != nil {
			s.logger.Error("failed marking celebration reminder dispatched", "event_id", item.EventID, "error", err)
		}
	}
}
