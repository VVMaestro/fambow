package telegram

import (
	"context"
	"fmt"
	"log/slog"

	"fambow/internal/service"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type BotRunner interface {
	Start(ctx context.Context)
}

type LoveNoteProvider interface {
	RandomNote(ctx context.Context, firstName string) (string, error)
	AddLoveNote(ctx context.Context, note string) error
}

type MemoryProvider interface {
	AddMemory(ctx context.Context, telegramUserID int64, firstName string, text string) (service.Memory, error)
	RecentMemories(ctx context.Context, telegramUserID int64, limit int) ([]service.Memory, error)
}

type ReminderProvider interface {
	AddReminder(ctx context.Context, telegramUserID int64, firstName string, command string) (service.Reminder, error)
	ListReminders(ctx context.Context, telegramUserID int64) ([]service.Reminder, error)
}

type CelebrationProvider interface {
	AddEvent(ctx context.Context, telegramUserID int64, firstName string, command string) (service.CelebrationEvent, error)
	ListEvents(ctx context.Context, telegramUserID int64) ([]service.CelebrationEvent, error)
}

func NewBot(token string, logger *slog.Logger, loveNotes LoveNoteProvider, memories MemoryProvider, reminders ReminderProvider, celebrations CelebrationProvider) (*bot.Bot, error) {
	b, err := bot.New(token, bot.WithDefaultHandler(defaultHandler(logger)))
	if err != nil {
		return nil, fmt.Errorf("init telegram client: %w", err)
	}

	registerCoreHandlers(b, logger, loveNotes, memories, reminders, celebrations)
	registerMenuCommands(context.Background(), b, logger)
	return b, nil
}

func defaultHandler(logger *slog.Logger) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		logger.Info("unhandled update", "update_id", update.ID)
	}
}
