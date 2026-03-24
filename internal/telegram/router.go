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
	RandomNote(ctx context.Context, firstName string) (service.LoveNote, error)
	AddLoveNote(ctx context.Context, input service.LoveNoteInput) error
	ListLoveNotes(ctx context.Context) ([]service.AdminLoveNote, error)
	DeleteLoveNotes(ctx context.Context, noteIDs []int64) (service.DeleteLoveNotesResult, error)
}

type MemoryProvider interface {
	AddMemory(ctx context.Context, telegramUserID int64, firstName string, input service.MemoryInput) (service.Memory, error)
	RecentMemories(ctx context.Context, telegramUserID int64, limit int) ([]service.Memory, error)
	RandomMemory(ctx context.Context) (service.Memory, error)
}

type ReminderProvider interface {
	AddReminder(ctx context.Context, telegramUserID int64, firstName string, command string) (service.Reminder, error)
	AddReminderForUserType(ctx context.Context, userType string, command string) (service.Reminder, error)
	ListReminders(ctx context.Context, telegramUserID int64) ([]service.Reminder, error)
}

type LoveNoteScheduleProvider interface {
	AddLoveNoteSchedule(ctx context.Context, telegramUserID int64, scheduleTime string) (service.LoveNoteSchedule, error)
	ListLoveNoteSchedules(ctx context.Context) ([]service.LoveNoteSchedule, error)
	RemoveLoveNoteSchedule(ctx context.Context, scheduleID int64) error
}

type CelebrationProvider interface {
	AddEvent(ctx context.Context, telegramUserID int64, firstName string, command string) (service.CelebrationEvent, error)
	ListEvents(ctx context.Context, telegramUserID int64) ([]service.CelebrationEvent, error)
}

type ProductProvider interface {
	AddProduct(ctx context.Context, command string) (service.Product, error)
	RemoveProduct(ctx context.Context, command string) (int64, error)
	ListProducts(ctx context.Context) ([]service.Product, error)
	BuyProduct(ctx context.Context, buyerTelegramUserID int64, productID int64) (service.PurchaseResult, error)
}

type UserProvider interface {
	IsRegistered(ctx context.Context, telegramUserID int64) (bool, error)
	CreateUser(ctx context.Context, telegramUserID int64, firstName string, userType string) (service.User, error)
	GetUser(ctx context.Context, telegramUserID int64) (service.User, error)
	ListUsers(ctx context.Context) ([]service.User, error)
	SetMoney(ctx context.Context, telegramUserID int64, money int64) (service.User, error)
}

func NewBot(token string, logger *slog.Logger, loveNotes LoveNoteProvider, memories MemoryProvider, reminders ReminderProvider, loveSchedules LoveNoteScheduleProvider, celebrations CelebrationProvider, products ProductProvider, users UserProvider, adminTelegramUserID int64) (*bot.Bot, error) {
	b, err := bot.New(token, bot.WithDefaultHandler(defaultHandler(logger)))
	if err != nil {
		return nil, fmt.Errorf("init telegram client: %w", err)
	}

	memoryWizard := newMemoryWizardState()
	reminderWizard := newReminderWizardState()
	eventWizard := newEventWizardState()
	loveScheduleWizard := newLoveScheduleWizardState()
	registerCoreHandlers(b, logger, loveNotes, memories, reminders, loveSchedules, celebrations, products, users, adminTelegramUserID, memoryWizard, reminderWizard, eventWizard, loveScheduleWizard)
	registerMenuCommands(context.Background(), b, logger)
	return b, nil
}

func defaultHandler(logger *slog.Logger) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		logger.Info("unhandled update", "update_id", update.ID)
	}
}
