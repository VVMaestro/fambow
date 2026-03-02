package telegram

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func registerCoreHandlers(b *bot.Bot, logger *slog.Logger, loveNotes LoveNoteProvider, memories MemoryProvider, reminders ReminderProvider, celebrations CelebrationProvider) {
	b.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, startHandler(logger))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/help", bot.MatchTypeExact, helpHandler(logger))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/love", bot.MatchTypeExact, loveHandler(logger, loveNotes))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/add_love", bot.MatchTypePrefix, addLoveNoteHandler(logger, loveNotes))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/memory", bot.MatchTypePrefix, memoryHandler(logger, memories))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/memories", bot.MatchTypeExact, memoriesHandler(logger, memories))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/reminders", bot.MatchTypeExact, remindersHandler(logger, reminders))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/remind", bot.MatchTypePrefix, remindHandler(logger, reminders))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/events", bot.MatchTypeExact, eventsHandler(logger, celebrations))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/event", bot.MatchTypePrefix, eventHandler(logger, celebrations))
}

func startHandler(logger *slog.Logger) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Hi my love! This is your cozy companion bot.\n\nUse /help to see what I can do so far.",
		})
		if err != nil {
			logger.Error("failed to send /start response", "error", err)
		}
	}
}

func helpHandler(logger *slog.Logger) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Available commands:\n/start - welcome message\n/help - command list\n/love - instant love note\n/memory <text> - save a sweet memory\n/memories - show recent memories\n/remind ... - create a reminder\n/reminders - list active reminders\n/event add ... - add celebration date\n/events - list celebration dates",
		})
		if err != nil {
			logger.Error("failed to send /help response", "error", err)
		}
	}
}
