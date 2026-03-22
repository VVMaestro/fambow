package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"fambow/internal/service"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const loveScheduleWizardCallbackPrefix = "lsw:"

func loveScheduleWizardStartHandler(logger *slog.Logger, loveSchedules LoveNoteScheduleProvider, users UserProvider, adminTelegramUserID int64, wizard *loveScheduleWizardState) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if !isAdminMessage(update.Message.From, adminTelegramUserID) {
			sendText(ctx, b, update.Message.Chat.ID, "Only admin can use love note scheduler commands.", logger, "/love_scheduler forbidden")
			return
		}

		if loveSchedules == nil || users == nil || wizard == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Love note scheduler feature is not configured yet.", logger, "/love_scheduler unavailable")
			return
		}

		registeredUsers, err := users.ListUsers(ctx)
		if err != nil {
			logger.Error("failed listing users for love note scheduler wizard", "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not load users right now. Please try again in a moment.", logger, "/love_scheduler users failed")
			return
		}

		if len(registeredUsers) == 0 {
			sendText(ctx, b, update.Message.Chat.ID, "No registered users yet. Create one first with /create_user.", logger, "/love_scheduler no users")
			return
		}

		wizard.Start(update.Message.From.ID)
		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      update.Message.Chat.ID,
			Text:        "Let's set up a daily love note.\n\nStep 1: choose who should receive it.\n(You can type cancel anytime to stop.)",
			ReplyMarkup: loveScheduleUserKeyboard(registeredUsers),
		})
		if err != nil {
			logger.Error("failed to start love note scheduler wizard", "error", err)
		}
	}
}

func loveSchedulesHandler(logger *slog.Logger, loveSchedules LoveNoteScheduleProvider, adminTelegramUserID int64) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if !isAdminMessage(update.Message.From, adminTelegramUserID) {
			sendText(ctx, b, update.Message.Chat.ID, "Only admin can use love note scheduler commands.", logger, "/love_schedulers forbidden")
			return
		}

		if loveSchedules == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Love note scheduler feature is not configured yet.", logger, "/love_schedulers unavailable")
			return
		}

		items, err := loveSchedules.ListLoveNoteSchedules(ctx)
		if err != nil {
			logger.Error("failed listing love note schedulers", "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not load love note schedulers right now. Please try again in a moment.", logger, "/love_schedulers failed")
			return
		}

		if len(items) == 0 {
			sendText(ctx, b, update.Message.Chat.ID, "No active love note schedulers yet. Add one with /love_scheduler.", logger, "/love_schedulers empty")
			return
		}

		lines := make([]string, 0, len(items)+1)
		lines = append(lines, "Active love note schedulers:")
		for _, item := range items {
			lines = append(lines, fmt.Sprintf("- #%d %s (%s) %s", item.ID, item.FirstName, item.UserType, item.ScheduleDisplay))
		}

		sendText(ctx, b, update.Message.Chat.ID, strings.Join(lines, "\n"), logger, "/love_schedulers sent")
	}
}

func loveScheduleRemoveHandler(logger *slog.Logger, loveSchedules LoveNoteScheduleProvider, adminTelegramUserID int64) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if !isAdminMessage(update.Message.From, adminTelegramUserID) {
			sendText(ctx, b, update.Message.Chat.ID, "Only admin can use love note scheduler commands.", logger, "/love_scheduler_remove forbidden")
			return
		}

		if loveSchedules == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Love note scheduler feature is not configured yet.", logger, "/love_scheduler_remove unavailable")
			return
		}

		scheduleID, err := parseLoveScheduleRemovePayload(extractCommandPayload(update.Message.Text, "/love_scheduler_remove"))
		if err != nil {
			sendText(ctx, b, update.Message.Chat.ID, loveScheduleRemoveUsage(), logger, "/love_scheduler_remove usage")
			return
		}

		if err := loveSchedules.RemoveLoveNoteSchedule(ctx, scheduleID); err != nil {
			if errors.Is(err, service.ErrLoveNoteScheduleNotFound) {
				sendText(ctx, b, update.Message.Chat.ID, "That love note scheduler does not exist or is already inactive.", logger, "/love_scheduler_remove missing")
				return
			}

			logger.Error("failed removing love note scheduler", "schedule_id", scheduleID, "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not remove that love note scheduler right now. Please try again in a moment.", logger, "/love_scheduler_remove failed")
			return
		}

		sendText(ctx, b, update.Message.Chat.ID, fmt.Sprintf("Love note scheduler #%d removed.", scheduleID), logger, "/love_scheduler_remove removed")
	}
}

func loveScheduleWizardCallbackHandler(logger *slog.Logger, loveSchedules LoveNoteScheduleProvider, users UserProvider, adminTelegramUserID int64, wizard *loveScheduleWizardState) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.CallbackQuery == nil {
			return
		}

		chatID := chatIDFromUpdate(update)
		user := senderFromUpdate(update)
		if user == nil {
			answerCallbackQuery(ctx, b, update, logger, "")
			return
		}

		if user.ID != adminTelegramUserID {
			if chatID != 0 {
				sendText(ctx, b, chatID, "Only admin can use love note scheduler commands.", logger, "love scheduler callback forbidden")
			}
			answerCallbackQuery(ctx, b, update, logger, "")
			return
		}

		if loveSchedules == nil || users == nil || wizard == nil {
			if chatID != 0 {
				sendText(ctx, b, chatID, "Love note scheduler feature is not configured yet.", logger, "love scheduler callback unavailable")
			}
			answerCallbackQuery(ctx, b, update, logger, "")
			return
		}

		session, ok := wizard.Get(user.ID)
		if !ok {
			if chatID != 0 {
				sendText(ctx, b, chatID, "Love note scheduler flow expired. Send /love_scheduler to start again.", logger, "love scheduler missing session")
			}
			answerCallbackQuery(ctx, b, update, logger, "")
			return
		}

		payload := strings.TrimPrefix(update.CallbackQuery.Data, loveScheduleWizardCallbackPrefix)
		action := payload
		value := ""
		if idx := strings.Index(payload, ":"); idx >= 0 {
			action = payload[:idx]
			value = payload[idx+1:]
		}

		switch action {
		case "cancel":
			wizard.Delete(user.ID)
			if chatID != 0 {
				sendText(ctx, b, chatID, "Love note scheduler flow canceled.", logger, "love scheduler cancel")
			}
		case "user":
			loveScheduleWizardHandleUser(ctx, b, logger, chatID, user.ID, value, users, wizard, session)
		case "time":
			loveScheduleWizardHandleTimeChoice(ctx, b, logger, chatID, user.ID, value, loveSchedules, wizard, session)
		default:
			if chatID != 0 {
				sendText(ctx, b, chatID, "I did not understand that choice. Please try again.", logger, "love scheduler unknown action")
			}
		}

		answerCallbackQuery(ctx, b, update, logger, "")
	}
}

func loveScheduleWizardMessageHandler(logger *slog.Logger, loveSchedules LoveNoteScheduleProvider, adminTelegramUserID int64, wizard *loveScheduleWizardState) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil || update.Message.From == nil {
			return
		}

		if update.Message.From.ID != adminTelegramUserID {
			return
		}

		if loveSchedules == nil || wizard == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Love note scheduler feature is not configured yet.", logger, "love scheduler message unavailable")
			return
		}

		session, ok := wizard.Get(update.Message.From.ID)
		if !ok {
			return
		}

		text := strings.TrimSpace(update.Message.Text)
		if isLoveScheduleWizardCancel(text) {
			wizard.Delete(update.Message.From.ID)
			sendText(ctx, b, update.Message.Chat.ID, "Love note scheduler flow canceled.", logger, "love scheduler text cancel")
			return
		}

		if session.Step == loveScheduleWizardStepAwaitTime {
			loveScheduleWizardHandleManualTime(ctx, b, logger, update, loveSchedules, wizard, session)
		}
	}
}

func loveScheduleWizardMatch(wizard *loveScheduleWizardState) bot.MatchFunc {
	return func(update *models.Update) bool {
		if wizard == nil || update.Message == nil || update.Message.From == nil {
			return false
		}

		session, ok := wizard.Get(update.Message.From.ID)
		if !ok {
			return false
		}

		text := strings.TrimSpace(update.Message.Text)
		if isLoveScheduleWizardCancel(text) {
			return true
		}

		if session.Step != loveScheduleWizardStepAwaitTime {
			return false
		}

		if text == "" {
			return true
		}

		if strings.HasPrefix(text, "/") && !isLoveScheduleWizardCancel(text) {
			return false
		}

		return true
	}
}

func loveScheduleWizardHandleUser(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64, adminUserID int64, value string, users UserProvider, wizard *loveScheduleWizardState, session loveScheduleWizardSession) {
	telegramUserID, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || telegramUserID <= 0 {
		sendText(ctx, b, chatID, "Please choose a valid user for the love note scheduler.", logger, "love scheduler invalid user")
		return
	}

	targetUser, err := users.GetUser(ctx, telegramUserID)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) || errors.Is(err, service.ErrUserTelegramIDInvalid) {
			wizard.Delete(adminUserID)
			sendText(ctx, b, chatID, "That user is no longer available. Send /love_scheduler to start again.", logger, "love scheduler target missing")
			return
		}

		logger.Error("failed loading target user for love note scheduler", "telegram_user_id", telegramUserID, "error", err)
		sendText(ctx, b, chatID, "I could not load that user right now. Please try again in a moment.", logger, "love scheduler target failed")
		return
	}

	session.TargetTelegramID = targetUser.TelegramUserID
	session.TargetFirstName = targetUser.FirstName
	session.TargetUserType = targetUser.Type
	session.Step = loveScheduleWizardStepAwaitTime
	wizard.Set(adminUserID, session)

	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        fmt.Sprintf("Great. %s (%s) will receive a daily love note.\nStep 2: pick a time.", targetUser.FirstName, targetUser.Type),
		ReplyMarkup: loveScheduleTimeKeyboard(),
	})
	if err != nil {
		logger.Error("failed to send love note scheduler time keyboard", "error", err)
	}
}

func loveScheduleWizardHandleTimeChoice(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64, adminUserID int64, value string, loveSchedules LoveNoteScheduleProvider, wizard *loveScheduleWizardState, session loveScheduleWizardSession) {
	if value == "manual" {
		session.Step = loveScheduleWizardStepAwaitTime
		wizard.Set(adminUserID, session)
		sendText(ctx, b, chatID, "Send the time in HH:MM (24h). Example: 08:30", logger, "love scheduler manual time")
		return
	}

	scheduleTime, err := parseReminderWizardDailyTime(value)
	if err != nil {
		sendText(ctx, b, chatID, "Pick a valid time like 08:30.", logger, "love scheduler preset time invalid")
		return
	}

	loveScheduleWizardSave(ctx, b, logger, chatID, adminUserID, loveSchedules, wizard, session, scheduleTime)
}

func loveScheduleWizardHandleManualTime(ctx context.Context, b *bot.Bot, logger *slog.Logger, update *models.Update, loveSchedules LoveNoteScheduleProvider, wizard *loveScheduleWizardState, session loveScheduleWizardSession) {
	scheduleTime, err := parseReminderWizardDailyTime(update.Message.Text)
	if err != nil {
		sendText(ctx, b, update.Message.Chat.ID, "Please send time like 08:30.", logger, "love scheduler manual time invalid")
		return
	}

	loveScheduleWizardSave(ctx, b, logger, update.Message.Chat.ID, update.Message.From.ID, loveSchedules, wizard, session, scheduleTime)
}

func loveScheduleWizardSave(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64, adminUserID int64, loveSchedules LoveNoteScheduleProvider, wizard *loveScheduleWizardState, session loveScheduleWizardSession, scheduleTime string) {
	if session.TargetTelegramID <= 0 || strings.TrimSpace(session.TargetFirstName) == "" || strings.TrimSpace(session.TargetUserType) == "" {
		wizard.Delete(adminUserID)
		sendText(ctx, b, chatID, "I lost track of the selected user. Start again with /love_scheduler.", logger, "love scheduler missing target")
		return
	}

	schedule, err := loveSchedules.AddLoveNoteSchedule(ctx, session.TargetTelegramID, scheduleTime)
	if err != nil {
		if errors.Is(err, service.ErrLoveNoteScheduleTimeFormat) {
			sendText(ctx, b, chatID, "Please send time like 08:30.", logger, "love scheduler time format")
			return
		}
		if errors.Is(err, service.ErrLoveNoteScheduleTargetNotFound) {
			wizard.Delete(adminUserID)
			sendText(ctx, b, chatID, "That user is no longer available. Send /love_scheduler to start again.", logger, "love scheduler save target missing")
			return
		}

		logger.Error("failed saving love note scheduler", "telegram_user_id", session.TargetTelegramID, "error", err)
		wizard.Delete(adminUserID)
		sendText(ctx, b, chatID, "I could not save that love note scheduler right now. Please try again in a moment.", logger, "love scheduler save failed")
		return
	}

	wizard.Delete(adminUserID)
	sendText(ctx, b, chatID, fmt.Sprintf("Love note scheduler saved: #%d %s (%s) %s.", schedule.ID, schedule.FirstName, schedule.UserType, schedule.ScheduleDisplay), logger, "love scheduler saved")
}

func loveScheduleUserKeyboard(users []service.User) *models.InlineKeyboardMarkup {
	buttons := make([][]models.InlineKeyboardButton, 0, len(users)+1)
	for _, user := range users {
		label := fmt.Sprintf("%s (%s)", user.FirstName, user.Type)
		buttons = append(buttons, []models.InlineKeyboardButton{{
			Text:         label,
			CallbackData: loveScheduleWizardCallbackPrefix + "user:" + strconv.FormatInt(user.TelegramUserID, 10),
		}})
	}

	buttons = append(buttons, []models.InlineKeyboardButton{{
		Text:         "Cancel",
		CallbackData: loveScheduleWizardCallbackPrefix + "cancel",
	}})

	return &models.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

func loveScheduleTimeKeyboard() *models.InlineKeyboardMarkup {
	rows := [][]string{
		{"07:30", "08:30", "09:00"},
		{"12:00", "15:00", "19:30"},
		{"21:00"},
	}

	buttons := make([][]models.InlineKeyboardButton, 0, len(rows)+1)
	for _, row := range rows {
		line := make([]models.InlineKeyboardButton, 0, len(row))
		for _, value := range row {
			line = append(line, models.InlineKeyboardButton{Text: value, CallbackData: loveScheduleWizardCallbackPrefix + "time:" + value})
		}
		buttons = append(buttons, line)
	}

	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: "Type Time", CallbackData: loveScheduleWizardCallbackPrefix + "time:manual"},
		{Text: "Cancel", CallbackData: loveScheduleWizardCallbackPrefix + "cancel"},
	})

	return &models.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

func parseLoveScheduleRemovePayload(payload string) (int64, error) {
	parts := strings.Fields(strings.TrimSpace(payload))
	if len(parts) != 1 {
		return 0, fmt.Errorf("invalid remove payload")
	}

	scheduleID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || scheduleID <= 0 {
		return 0, fmt.Errorf("invalid schedule id")
	}

	return scheduleID, nil
}

func loveScheduleRemoveUsage() string {
	return "Use this format:\n/love_scheduler_remove <id>"
}

func isAdminMessage(user *models.User, adminTelegramUserID int64) bool {
	return user != nil && user.ID == adminTelegramUserID
}

func isLoveScheduleWizardCancel(text string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(text))
	return trimmed == "cancel" || trimmed == "/cancel"
}
