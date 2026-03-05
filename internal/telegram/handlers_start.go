package telegram

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"

	"fambow/internal/service"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func registerCoreHandlers(b *bot.Bot, logger *slog.Logger, loveNotes LoveNoteProvider, memories MemoryProvider, reminders ReminderProvider, celebrations CelebrationProvider, users UserProvider, adminTelegramUserID int64) {
	guard := accessGuard(logger, users, adminTelegramUserID)

	b.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, guard(startHandler(logger)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/help", bot.MatchTypeExact, guard(helpHandler(logger)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/love", bot.MatchTypeExact, guard(loveHandler(logger, loveNotes)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/add_love", bot.MatchTypePrefix, guard(addLoveNoteHandler(logger, loveNotes)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/memory", bot.MatchTypePrefix, guard(memoryHandler(logger, memories)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/memories", bot.MatchTypeExact, guard(memoriesHandler(logger, memories)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/reminders", bot.MatchTypeExact, guard(remindersHandler(logger, reminders)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/remind", bot.MatchTypePrefix, guard(remindHandler(logger, reminders)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/events", bot.MatchTypeExact, guard(eventsHandler(logger, celebrations)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/event", bot.MatchTypePrefix, guard(eventHandler(logger, celebrations)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/create_user", bot.MatchTypePrefix, createUserHandler(logger, users, adminTelegramUserID))
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
			Text:   "Available commands:\n/start - welcome message\n/help - command list\n/love - instant love note\n/memory <text> - save a sweet memory\n/memories - show recent memories\n/remind ... - create a reminder\n/remind him at HH:MM to ... - reminder for husband\n/remind her at HH:MM to ... - reminder for wife\n/reminders - list active reminders\n/event add ... - add celebration date\n/events - list celebration dates\n/create_user <telegram_id> <first_name> <husband|wife> - admin only",
		})
		if err != nil {
			logger.Error("failed to send /help response", "error", err)
		}
	}
}

func accessGuard(logger *slog.Logger, users UserProvider, adminTelegramUserID int64) func(next bot.HandlerFunc) bot.HandlerFunc {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, update *models.Update) {
			if update.Message == nil || update.Message.From == nil {
				return
			}

			if update.Message.From.ID == adminTelegramUserID {
				next(ctx, b, update)
				return
			}

			if users == nil {
				sendText(ctx, b, update.Message.Chat.ID, "User access control is not configured yet.", logger, "access users nil")
				return
			}

			allowed, err := users.IsRegistered(ctx, update.Message.From.ID)
			if err != nil {
				logger.Error("failed to check user access", "telegram_user_id", update.Message.From.ID, "error", err)
				sendText(ctx, b, update.Message.Chat.ID, "I could not verify your access right now. Please try again in a moment.", logger, "access check failed")
				return
			}

			if !allowed {
				sendText(ctx, b, update.Message.Chat.ID, "Access denied. Ask the admin to create your user with /create_user.", logger, "access denied")
				return
			}

			next(ctx, b, update)
		}
	}
}

func createUserHandler(logger *slog.Logger, users UserProvider, adminTelegramUserID int64) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if update.Message.From == nil {
			sendText(ctx, b, update.Message.Chat.ID, "I could not read your profile info for this command. Please try again.", logger, "/create_user missing sender")
			return
		}

		if update.Message.From.ID != adminTelegramUserID {
			sendText(ctx, b, update.Message.Chat.ID, "Only admin can use /create_user.", logger, "/create_user forbidden")
			return
		}

		if users == nil {
			sendText(ctx, b, update.Message.Chat.ID, "User management is not configured yet.", logger, "/create_user users nil")
			return
		}

		telegramUserID, firstName, userType, err := parseCreateUserPayload(extractCommandPayload(update.Message.Text, "/create_user"))
		if err != nil {
			sendText(ctx, b, update.Message.Chat.ID, createUserUsage(), logger, "/create_user usage")
			return
		}

		created, err := users.CreateUser(ctx, telegramUserID, firstName, userType)
		if err != nil {
			if errors.Is(err, service.ErrUserAlreadyExists) {
				sendText(ctx, b, update.Message.Chat.ID, "This Telegram user already exists in the database.", logger, "/create_user already exists")
				return
			}
			if errors.Is(err, service.ErrUserTelegramIDInvalid) || errors.Is(err, service.ErrUserFirstNameEmpty) || errors.Is(err, service.ErrUserTypeInvalid) {
				sendText(ctx, b, update.Message.Chat.ID, createUserUsage(), logger, "/create_user invalid input")
				return
			}

			logger.Error("failed to create user", "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not create user right now. Please try again in a moment.", logger, "/create_user failed")
			return
		}

		sendText(ctx, b, update.Message.Chat.ID, "User created: "+created.FirstName+" ("+created.Type+")", logger, "/create_user created")
	}
}

func parseCreateUserPayload(payload string) (int64, string, string, error) {
	parts := strings.Fields(strings.TrimSpace(payload))
	if len(parts) != 3 {
		return 0, "", "", service.ErrUserTypeInvalid
	}

	telegramUserID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, "", "", service.ErrUserTelegramIDInvalid
	}

	return telegramUserID, strings.TrimSpace(parts[1]), strings.TrimSpace(strings.ToLower(parts[2])), nil
}

func createUserUsage() string {
	return "Use this format:\n/create_user <telegram_user_id> <first_name> <husband|wife>"
}
