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

func registerCoreHandlers(b *bot.Bot, logger *slog.Logger, loveNotes LoveNoteProvider, memories MemoryProvider, reminders ReminderProvider, loveSchedules LoveNoteScheduleProvider, celebrations CelebrationProvider, products ProductProvider, users UserProvider, adminTelegramUserID int64, memoryWizard *memoryWizardState, reminderWizard *reminderWizardState, eventWizard *eventWizardState, loveScheduleWizard *loveScheduleWizardState) {
	guard := accessGuard(logger, users, adminTelegramUserID)

	b.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, guard(startHandler(logger)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/help", bot.MatchTypeExact, guard(helpHandler(logger)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/love", bot.MatchTypeExact, guard(loveHandler(logger, loveNotes)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "Love Note", bot.MatchTypePrefix, guard(loveHandler(logger, loveNotes)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "Memory", bot.MatchTypePrefix, guard(memoryWizardStartHandler(logger, memories, memoryWizard)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "Memories", bot.MatchTypePrefix, guard(memoriesHandler(logger, memories)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "Surprise Memory", bot.MatchTypePrefix, guard(surpriseMemoryHandler(logger, memories)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "Reminder", bot.MatchTypePrefix, guard(reminderWizardStartHandler(logger, reminders, reminderWizard, users)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "My Reminders", bot.MatchTypePrefix, guard(remindersHandler(logger, reminders)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "Event", bot.MatchTypePrefix, guard(eventWizardStartHandler(logger, celebrations, eventWizard)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "List Events", bot.MatchTypePrefix, guard(eventsHandler(logger, celebrations)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/add_love", bot.MatchTypePrefix, guard(addLoveNoteHandler(logger, loveNotes)))
	b.RegisterHandler(bot.HandlerTypePhotoCaption, "/add_love", bot.MatchTypePrefix, guard(addLoveNotePhotoHandler(logger, loveNotes)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/memory", bot.MatchTypePrefix, guard(memoryHandler(logger, memories, memoryWizard)))
	b.RegisterHandler(bot.HandlerTypePhotoCaption, "/memory", bot.MatchTypePrefix, guard(memoryPhotoHandler(logger, memories, memoryWizard)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/memories", bot.MatchTypeExact, guard(memoriesHandler(logger, memories)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/surprise_memory", bot.MatchTypeExact, guard(surpriseMemoryHandler(logger, memories)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/reminder", bot.MatchTypeExact, guard(reminderWizardStartHandler(logger, reminders, reminderWizard, users)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/reminders", bot.MatchTypeExact, guard(remindersHandler(logger, reminders)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/remind", bot.MatchTypePrefix, guard(remindHandler(logger, reminders)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/events", bot.MatchTypeExact, guard(eventsHandler(logger, celebrations)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/event", bot.MatchTypePrefix, guard(eventHandler(logger, celebrations, eventWizard)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/stuffs", bot.MatchTypeExact, guard(productsHandler(logger, products, users)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "Market", bot.MatchTypePrefix, guard(productsHandler(logger, products, users)))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/create_user", bot.MatchTypePrefix, createUserHandler(logger, users, adminTelegramUserID))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/money_set", bot.MatchTypePrefix, moneySetHandler(logger, users, adminTelegramUserID))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/stuff_add", bot.MatchTypePrefix, productAddHandler(logger, products, adminTelegramUserID))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/stuff_remove", bot.MatchTypePrefix, productRemoveHandler(logger, products, adminTelegramUserID))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/love_scheduler", bot.MatchTypeExact, loveScheduleWizardStartHandler(logger, loveSchedules, users, adminTelegramUserID, loveScheduleWizard))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/love_schedulers", bot.MatchTypeExact, loveSchedulesHandler(logger, loveSchedules, adminTelegramUserID))
	b.RegisterHandler(bot.HandlerTypeMessageText, "/love_scheduler_remove", bot.MatchTypePrefix, loveScheduleRemoveHandler(logger, loveSchedules, adminTelegramUserID))
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, marketCallbackPrefix, bot.MatchTypePrefix, guard(productBuyCallbackHandler(logger, products)))
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, memoryWizardCallbackPrefix, bot.MatchTypePrefix, guard(memoryWizardCallbackHandler(logger, memories, memoryWizard)))
	b.RegisterHandlerMatchFunc(memoryWizardMatch(memoryWizard), guard(memoryWizardMessageHandler(logger, memories, memoryWizard)))
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, reminderWizardCallbackPrefix, bot.MatchTypePrefix, guard(reminderWizardCallbackHandler(logger, reminders, reminderWizard)))
	b.RegisterHandlerMatchFunc(reminderWizardMatch(reminderWizard), guard(reminderWizardMessageHandler(logger, reminders, reminderWizard)))
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, eventWizardCallbackPrefix, bot.MatchTypePrefix, guard(eventWizardCallbackHandler(logger, celebrations, eventWizard)))
	b.RegisterHandlerMatchFunc(eventWizardMatch(eventWizard), guard(eventWizardMessageHandler(logger, celebrations, eventWizard)))
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, loveScheduleWizardCallbackPrefix, bot.MatchTypePrefix, loveScheduleWizardCallbackHandler(logger, loveSchedules, users, adminTelegramUserID, loveScheduleWizard))
	b.RegisterHandlerMatchFunc(loveScheduleWizardMatch(loveScheduleWizard), loveScheduleWizardMessageHandler(logger, loveSchedules, adminTelegramUserID, loveScheduleWizard))
}

func startHandler(logger *slog.Logger) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      update.Message.Chat.ID,
			Text:        "Hi my love! This is your cozy companion bot.\n\nUse /help to see what I can do so far, or tap one of the buttons below for Love Notes, Memories, Reminder, My Reminders, Event, or Events. You can also save a photo love note with a photo caption `/add_love`, `/memory` and `/event` start guided flows, and `/stuffs` opens the family market.",
			ReplyMarkup: commandKeyboard(),
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
			ChatID:      update.Message.Chat.ID,
			Text:        "Available commands:\n/start - welcome message\n/help - command list\n/love - instant love note\n/add_love - add a love note\nPhoto shortcut: send a photo with caption /add_love <optional note>\n/memory - guided memory creator\nShortcut: /memory <text>\nShortcut: /memory YYYY-MM-DD | <text>\nPhoto shortcut: send a photo with caption /memory <optional note> or /memory YYYY-MM-DD | <optional note>\n/memories - show recent memories\n/surprise_memory - share a random memory\n/reminder - guided reminder creator\n/remind ... - create a reminder via text\n/remind him at HH:MM to ... - reminder for husband\n/remind her at HH:MM to ... - reminder for wife\n/reminders - list active reminders\n/event - guided celebration creator\nShortcut: /event add YYYY-MM-DD | Title | Days\n/events - list celebration dates\n/stuffs - show available market stuff\n/create_user <telegram_id> <first_name> <husband|wife> - admin only\n/money_set <telegram_id> <amount in pan-coins> - admin only\n/stuff_add <name> | <cost in pan-coins> - admin only\n/stuff_remove <id> - admin only\n/love_scheduler - admin daily love note scheduler wizard\n/love_schedulers - admin list active love note schedulers\n/love_scheduler_remove <id> - admin remove active love note scheduler\n\nQuick buttons:\nTap Love Note, Memory, Memories, Surprise Memory, Reminder, My Reminders, Event, or Events for shortcuts.",
			ReplyMarkup: commandKeyboard(),
		})
		if err != nil {
			logger.Error("failed to send /help response", "error", err)
		}
	}
}

func accessGuard(logger *slog.Logger, users UserProvider, adminTelegramUserID int64) func(next bot.HandlerFunc) bot.HandlerFunc {
	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, update *models.Update) {
			fromUser := senderFromUpdate(update)
			chatID := chatIDFromUpdate(update)
			if fromUser == nil || chatID == 0 {
				answerCallbackQuery(ctx, b, update, logger, "")
				return
			}

			if fromUser.ID == adminTelegramUserID {
				next(ctx, b, update)
				return
			}

			if users == nil {
				sendText(ctx, b, chatID, "User access control is not configured yet.", logger, "access users nil")
				answerCallbackQuery(ctx, b, update, logger, "")
				return
			}

			allowed, err := users.IsRegistered(ctx, fromUser.ID)
			if err != nil {
				logger.Error("failed to check user access", "telegram_user_id", fromUser.ID, "error", err)
				sendText(ctx, b, chatID, "I could not verify your access right now. Please try again in a moment.", logger, "access check failed")
				answerCallbackQuery(ctx, b, update, logger, "")
				return
			}

			if !allowed {
				sendText(ctx, b, chatID, "Access denied. Ask the admin to create your user with /create_user.", logger, "access denied")
				answerCallbackQuery(ctx, b, update, logger, "")
				return
			}

			next(ctx, b, update)
		}
	}
}

func senderFromUpdate(update *models.Update) *models.User {
	if update == nil {
		return nil
	}

	if update.Message != nil {
		return update.Message.From
	}

	if update.CallbackQuery != nil {
		return &update.CallbackQuery.From
	}

	return nil
}

func chatIDFromUpdate(update *models.Update) int64 {
	if update == nil {
		return 0
	}

	if update.Message != nil {
		return update.Message.Chat.ID
	}

	if update.CallbackQuery != nil {
		if update.CallbackQuery.Message.Message != nil {
			return update.CallbackQuery.Message.Message.Chat.ID
		}
		if update.CallbackQuery.Message.InaccessibleMessage != nil {
			return update.CallbackQuery.Message.InaccessibleMessage.Chat.ID
		}
	}

	return 0
}

func answerCallbackQuery(ctx context.Context, b *bot.Bot, update *models.Update, logger *slog.Logger, text string) {
	if update == nil || update.CallbackQuery == nil {
		return
	}

	params := &bot.AnswerCallbackQueryParams{CallbackQueryID: update.CallbackQuery.ID}
	if text != "" {
		params.Text = text
	}

	if _, err := b.AnswerCallbackQuery(ctx, params); err != nil {
		logger.Warn("failed to answer callback query", "error", err)
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
