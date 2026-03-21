package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"fambow/internal/service"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const reminderWizardCallbackPrefix = "rw:"

func remindHandler(logger *slog.Logger, reminders ReminderProvider) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if update.Message.From == nil {
			sendText(ctx, b, update.Message.Chat.ID, "I could not read your profile info for this reminder. Please try again.", logger, "/remind missing sender")
			return
		}

		if reminders == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Reminder feature is not configured yet.", logger, "/remind unavailable")
			return
		}

		payload := extractCommandPayload(update.Message.Text, "/remind")
		targetUserType, targetLabel, targetPayload := extractReminderTarget(payload)

		var (
			reminder service.Reminder
			err      error
		)

		if targetUserType == "" {
			reminder, err = reminders.AddReminder(ctx, update.Message.From.ID, update.Message.From.FirstName, payload)
		} else {
			reminder, err = reminders.AddReminderForUserType(ctx, targetUserType, targetPayload)
		}

		if err != nil {
			if errors.Is(err, service.ErrReminderCommandEmpty) ||
				errors.Is(err, service.ErrReminderInvalidFormat) ||
				errors.Is(err, service.ErrReminderTextEmpty) ||
				errors.Is(err, service.ErrReminderTimeFormat) ||
				errors.Is(err, service.ErrReminderDateTimeFormat) ||
				errors.Is(err, service.ErrReminderTimeInPast) {
				sendText(ctx, b, update.Message.Chat.ID, service.ReminderUsage(), logger, "/remind usage")
				return
			}

			if errors.Is(err, service.ErrReminderTargetNotFound) {
				sendText(ctx, b, update.Message.Chat.ID, "I could not find that partner in the database yet. Ask both users to use the bot once, then try again.", logger, "/remind target missing")
				return
			}

			if errors.Is(err, service.ErrReminderUserNotFound) {
				sendText(ctx, b, update.Message.Chat.ID, "I could not find your user in the database.", logger, "/remind sender missing")
				return
			}

			logger.Error("failed to create reminder", "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not save your reminder right now. Please try again in a moment.", logger, "/remind save failed")
			return
		}

		confirmation := fmt.Sprintf("Reminder saved: %s\n- %s", reminder.ScheduleDisplay, reminder.Text)
		if targetUserType != "" {
			confirmation = fmt.Sprintf("Reminder saved for %s: %s\n- %s", targetLabel, reminder.ScheduleDisplay, reminder.Text)
		}
		sendText(ctx, b, update.Message.Chat.ID, confirmation, logger, "/remind saved")
	}
}

func remindersHandler(logger *slog.Logger, reminders ReminderProvider) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil || update.Message.From == nil {
			return
		}

		if reminders == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Reminder feature is not configured yet.", logger, "/reminders unavailable")
			return
		}

		items, err := reminders.ListReminders(ctx, update.Message.From.ID)
		if err != nil {
			logger.Error("failed listing reminders", "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not load reminders right now. Please try again in a moment.", logger, "/reminders load failed")
			return
		}

		if len(items) == 0 {
			sendText(ctx, b, update.Message.Chat.ID, "No active reminders yet. Add one with /remind.", logger, "/reminders empty")
			return
		}

		lines := make([]string, 0, len(items)+1)
		lines = append(lines, "Your active reminders:")
		for _, item := range items {
			lines = append(lines, fmt.Sprintf("- %s: %s", item.ScheduleDisplay, item.Text))
		}

		sendText(ctx, b, update.Message.Chat.ID, strings.Join(lines, "\n"), logger, "/reminders sent")
	}
}

func extractCommandPayload(message string, command string) string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return ""
	}

	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return ""
	}

	if strings.HasPrefix(parts[0], command) {
		return strings.TrimSpace(strings.TrimPrefix(trimmed, parts[0]))
	}

	return ""
}

func extractReminderTarget(payload string) (string, string, string) {
	parts := strings.Fields(strings.TrimSpace(payload))
	if len(parts) < 2 {
		return "", "", payload
	}

	switch strings.ToLower(parts[0]) {
	case "him":
		return "husband", "him", strings.TrimSpace(strings.Join(parts[1:], " "))
	case "her":
		return "wife", "her", strings.TrimSpace(strings.Join(parts[1:], " "))
	default:
		return "", "", payload
	}
}

func reminderWizardStartHandler(logger *slog.Logger, reminders ReminderProvider, wizard *reminderWizardState, users UserProvider) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil || update.Message.From == nil {
			return
		}

		if reminders == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Reminder feature is not configured yet.", logger, "/reminder unavailable")
			return
		}

		if wizard != nil {
			wizard.Start(update.Message.From.ID)
		}

		var userType string
		if users != nil {
			profile, err := users.GetUser(ctx, update.Message.From.ID)
			if err != nil && !errors.Is(err, service.ErrUserNotFound) && !errors.Is(err, service.ErrUserTelegramIDInvalid) {
				logger.Warn("failed to load user profile for reminder wizard", "error", err)
			}
			if err == nil {
				userType = profile.Type
			}
		}

		text := "Let's craft a reminder together.\n\nStep 1: who is this reminder for?\n(You can type cancel anytime to stop.)"
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      update.Message.Chat.ID,
			Text:        text,
			ReplyMarkup: reminderWizardTargetKeyboard(userType),
		})
		if err != nil {
			logger.Error("failed to send reminder wizard start", "error", err)
		}
	}
}

func reminderWizardCallbackHandler(logger *slog.Logger, reminders ReminderProvider, wizard *reminderWizardState) bot.HandlerFunc {
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

		if reminders == nil {
			if chatID != 0 {
				sendText(ctx, b, chatID, "Reminder feature is not configured yet.", logger, "reminder wizard unavailable")
			}
			answerCallbackQuery(ctx, b, update, logger, "")
			return
		}

		payload := strings.TrimPrefix(update.CallbackQuery.Data, reminderWizardCallbackPrefix)
		action := payload
		value := ""
		if idx := strings.Index(payload, ":"); idx >= 0 {
			action = payload[:idx]
			value = payload[idx+1:]
		}

		session, ok := wizard.Get(user.ID)
		if !ok {
			if chatID != 0 {
				sendText(ctx, b, chatID, "Reminder flow expired. Send /reminder to start again.", logger, "reminder wizard missing session")
			}
			answerCallbackQuery(ctx, b, update, logger, "")
			return
		}

		switch action {
		case "cancel":
			wizard.Delete(user.ID)
			if chatID != 0 {
				sendText(ctx, b, chatID, "Reminder flow canceled.", logger, "reminder wizard cancel")
			}
		case "target":
			session = reminderWizardHandleTarget(ctx, b, logger, chatID, user.ID, value, session, wizard)
		case "schedule":
			session = reminderWizardHandleSchedule(ctx, b, logger, chatID, user.ID, value, session, wizard)
		case "time_daily":
			session = reminderWizardHandleDailyTime(ctx, b, logger, chatID, user.ID, value, session, wizard)
		case "time_once":
			session = reminderWizardHandleOnceTime(ctx, b, logger, chatID, user.ID, value, session, wizard)
		default:
			if chatID != 0 {
				sendText(ctx, b, chatID, "I did not understand that choice. Please try again.", logger, "reminder wizard unknown action")
			}
		}

		answerCallbackQuery(ctx, b, update, logger, "")
	}
}

func reminderWizardHandleTarget(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64, userID int64, value string, session reminderWizardSession, wizard *reminderWizardState) reminderWizardSession {
	switch value {
	case "self":
		session.TargetUserType = ""
		session.TargetLabel = "you"
	case "husband":
		session.TargetUserType = "husband"
		session.TargetLabel = "him"
	case "wife":
		session.TargetUserType = "wife"
		session.TargetLabel = "her"
	case "cancel":
		wizard.Delete(userID)
		if chatID != 0 {
			sendText(ctx, b, chatID, "Reminder flow canceled.", logger, "reminder wizard cancel")
		}
		return session
	default:
		if chatID != 0 {
			sendText(ctx, b, chatID, "Pick who should receive the reminder.", logger, "reminder wizard target")
		}
		return session
	}

	session.Step = reminderWizardStepSchedule
	wizard.Set(userID, session)
	if chatID != 0 {
		target := session.TargetLabel
		if target == "" {
			target = "you"
		}
		text := fmt.Sprintf("Great! We'll remind %s.\nStep 2: how often should it run?", target)
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: reminderWizardScheduleKeyboard(),
		})
		if err != nil {
			logger.Error("failed to send reminder schedule keyboard", "error", err)
		}
	}

	return session
}

func reminderWizardHandleSchedule(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64, userID int64, value string, session reminderWizardSession, wizard *reminderWizardState) reminderWizardSession {
	switch value {
	case "daily":
		session.ScheduleType = "daily"
		session.Step = reminderWizardStepSchedule
		wizard.Set(userID, session)
		return reminderWizardSendDailyTimePrompt(ctx, b, logger, chatID, session)
	case "once":
		session.ScheduleType = "once"
		session.Step = reminderWizardStepSchedule
		wizard.Set(userID, session)
		return reminderWizardSendOnceTimePrompt(ctx, b, logger, chatID, session)
	case "cancel":
		wizard.Delete(userID)
		if chatID != 0 {
			sendText(ctx, b, chatID, "Reminder flow canceled.", logger, "reminder wizard cancel")
		}
		return session
	default:
		if chatID != 0 {
			sendText(ctx, b, chatID, "Please choose daily or one-time reminder.", logger, "reminder wizard schedule invalid")
		}
		return session
	}
}

func reminderWizardHandleDailyTime(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64, userID int64, value string, session reminderWizardSession, wizard *reminderWizardState) reminderWizardSession {
	if session.ScheduleType != "daily" {
		return session
	}

	if value == "manual" {
		session.Step = reminderWizardStepAwaitTime
		wizard.Set(userID, session)
		if chatID != 0 {
			sendText(ctx, b, chatID, "Send the time in HH:MM (24h). Example: 08:30", logger, "reminder wizard daily manual")
		}
		return session
	}

	parsed, err := parseReminderWizardDailyTime(value)
	if err != nil {
		if chatID != 0 {
			sendText(ctx, b, chatID, "Pick a valid time like 07:30.", logger, "reminder wizard daily invalid")
		}
		return session
	}

	session.TimeValue = parsed
	session.Step = reminderWizardStepText
	wizard.Set(userID, session)
	if chatID != 0 {
		sendText(ctx, b, chatID, "Beautiful. Final step: send the reminder text (e.g., 'drink water').", logger, "reminder wizard daily time set")
	}
	return session
}

func reminderWizardHandleOnceTime(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64, userID int64, value string, session reminderWizardSession, wizard *reminderWizardState) reminderWizardSession {
	if session.ScheduleType != "once" {
		return session
	}

	if value == "manual" {
		session.Step = reminderWizardStepAwaitTime
		wizard.Set(userID, session)
		if chatID != 0 {
			sendText(ctx, b, chatID, "Send date and time like 2026-03-14 19:30 or just 19:30 for the next occurrence.", logger, "reminder wizard once manual")
		}
		return session
	}

	formatted := strings.ReplaceAll(strings.TrimSpace(value), "T", " ")
	parsed, err := parseReminderWizardOnceTime(formatted)
	if err != nil {
		if chatID != 0 {
			sendText(ctx, b, chatID, "That time is no longer available. Pick another option.", logger, "reminder wizard once invalid")
		}
		return session
	}

	session.TimeValue = parsed
	session.Step = reminderWizardStepText
	wizard.Set(userID, session)
	if chatID != 0 {
		sendText(ctx, b, chatID, "Beautiful. Final step: send the reminder text (e.g., 'call mom').", logger, "reminder wizard once time set")
	}
	return session
}

func reminderWizardSendDailyTimePrompt(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64, session reminderWizardSession) reminderWizardSession {
	if chatID != 0 {
		text := "Step 3: pick a time for the daily reminder."
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: reminderWizardDailyTimeKeyboard(),
		})
		if err != nil {
			logger.Error("failed to send daily time keyboard", "error", err)
		}
	}

	return session
}

func reminderWizardSendOnceTimePrompt(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64, session reminderWizardSession) reminderWizardSession {
	if chatID != 0 {
		text := "Step 3: pick when the one-time reminder should fire."
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      chatID,
			Text:        text,
			ReplyMarkup: reminderWizardOneTimeKeyboard(time.Now()),
		})
		if err != nil {
			logger.Error("failed to send once time keyboard", "error", err)
		}
	}

	return session
}

func reminderWizardMatch(wizard *reminderWizardState) bot.MatchFunc {
	return func(update *models.Update) bool {
		if wizard == nil || update.Message == nil || update.Message.From == nil {
			return false
		}

		session, ok := wizard.Get(update.Message.From.ID)
		if !ok {
			return false
		}

		if session.Step != reminderWizardStepAwaitTime && session.Step != reminderWizardStepText {
			return false
		}

		text := strings.TrimSpace(update.Message.Text)
		if text == "" {
			return true
		}

		if strings.HasPrefix(text, "/") && !isReminderWizardCancel(text) {
			return false
		}

		return true
	}
}

func reminderWizardMessageHandler(logger *slog.Logger, reminders ReminderProvider, wizard *reminderWizardState) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil || update.Message.From == nil {
			return
		}

		if reminders == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Reminder feature is not configured yet.", logger, "reminder wizard unavailable")
			return
		}

		session, ok := wizard.Get(update.Message.From.ID)
		if !ok {
			return
		}

		text := strings.TrimSpace(update.Message.Text)
		if isReminderWizardCancel(text) {
			wizard.Delete(update.Message.From.ID)
			sendText(ctx, b, update.Message.Chat.ID, "Reminder flow canceled.", logger, "reminder wizard cancel")
			return
		}

		switch session.Step {
		case reminderWizardStepAwaitTime:
			wizardHandleManualTime(ctx, b, logger, update.Message.Chat.ID, update.Message.From.ID, session, text, wizard)
		case reminderWizardStepText:
			wizardHandleReminderText(ctx, b, logger, update, text, reminders, wizard, session)
		}
	}
}

func wizardHandleManualTime(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64, userID int64, session reminderWizardSession, text string, wizard *reminderWizardState) {
	switch session.ScheduleType {
	case "daily":
		parsed, err := parseReminderWizardDailyTime(text)
		if err != nil {
			sendText(ctx, b, chatID, "Please send time like 21:15.", logger, "reminder wizard daily manual invalid")
			return
		}

		session.TimeValue = parsed
		session.Step = reminderWizardStepText
		wizard.Set(userID, session)
		sendText(ctx, b, chatID, "Perfect. Now send the reminder text (e.g., 'stretch for 2 minutes').", logger, "reminder wizard daily manual set")
	case "once":
		parsed, err := parseReminderWizardOnceTime(text)
		if err != nil {
			sendText(ctx, b, chatID, "Send date/time like 2026-03-14 19:30 or 21:00.", logger, "reminder wizard once manual invalid")
			return
		}

		session.TimeValue = parsed
		session.Step = reminderWizardStepText
		wizard.Set(userID, session)
		sendText(ctx, b, chatID, "Perfect. Now send the reminder text (e.g., 'call mom').", logger, "reminder wizard once manual set")
	default:
		sendText(ctx, b, chatID, "I lost track of the reminder time. Start again with /reminder.", logger, "reminder wizard manual unknown")
		wizard.Delete(userID)
	}
}

func wizardHandleReminderText(ctx context.Context, b *bot.Bot, logger *slog.Logger, update *models.Update, text string, reminders ReminderProvider, wizard *reminderWizardState, session reminderWizardSession) {
	if text == "" {
		sendText(ctx, b, update.Message.Chat.ID, "Reminder text cannot be empty.", logger, "reminder wizard empty text")
		return
	}

	if session.ScheduleType == "" || session.TimeValue == "" {
		sendText(ctx, b, update.Message.Chat.ID, "I need a time before saving. Start again with /reminder.", logger, "reminder wizard missing state")
		wizard.Delete(update.Message.From.ID)
		return
	}

	command := buildReminderWizardCommand(session, text)
	if command == "" {
		sendText(ctx, b, update.Message.Chat.ID, "I could not understand that reminder. Start again with /reminder.", logger, "reminder wizard command empty")
		wizard.Delete(update.Message.From.ID)
		return
	}

	var (
		reminder service.Reminder
		err      error
	)
	if session.TargetUserType == "" {
		reminder, err = reminders.AddReminder(ctx, update.Message.From.ID, update.Message.From.FirstName, command)
	} else {
		reminder, err = reminders.AddReminderForUserType(ctx, session.TargetUserType, command)
	}

	if err != nil {
		if errors.Is(err, service.ErrReminderTimeInPast) {
			sendText(ctx, b, update.Message.Chat.ID, "That time is in the past. Send a new time.", logger, "reminder wizard time past")
			session.Step = reminderWizardStepAwaitTime
			wizard.Set(update.Message.From.ID, session)
			return
		}

		logger.Error("failed to save wizard reminder", "error", err)
		sendText(ctx, b, update.Message.Chat.ID, "I could not save that reminder. Please try again with /reminder.", logger, "reminder wizard save failed")
		wizard.Delete(update.Message.From.ID)
		return
	}

	wizard.Delete(update.Message.From.ID)
	confirmation := fmt.Sprintf("Reminder saved: %s\n- %s", reminder.ScheduleDisplay, reminder.Text)
	if session.TargetUserType != "" && session.TargetLabel != "" {
		confirmation = fmt.Sprintf("Reminder saved for %s: %s\n- %s", session.TargetLabel, reminder.ScheduleDisplay, reminder.Text)
	}
	sendText(ctx, b, update.Message.Chat.ID, confirmation, logger, "reminder wizard saved")
}

func buildReminderWizardCommand(session reminderWizardSession, text string) string {
	trimmedText := strings.TrimSpace(text)
	if trimmedText == "" {
		return ""
	}

	trimmedTime := strings.TrimSpace(session.TimeValue)
	if session.ScheduleType == "daily" {
		return fmt.Sprintf("daily %s %s", trimmedTime, trimmedText)
	}
	if session.ScheduleType == "once" {
		return fmt.Sprintf("once %s %s", trimmedTime, trimmedText)
	}
	return ""
}

func reminderWizardTargetKeyboard(userType string) *models.InlineKeyboardMarkup {
	lower := strings.TrimSpace(strings.ToLower(userType))
	row := []models.InlineKeyboardButton{{Text: "For Me", CallbackData: reminderWizardCallbackPrefix + "target:self"}}
	switch lower {
	case "husband":
		row = append(row, models.InlineKeyboardButton{Text: "For Her", CallbackData: reminderWizardCallbackPrefix + "target:wife"})
	case "wife":
		row = append(row, models.InlineKeyboardButton{Text: "For Him", CallbackData: reminderWizardCallbackPrefix + "target:husband"})
	default:
		row = append(row,
			models.InlineKeyboardButton{Text: "For Him", CallbackData: reminderWizardCallbackPrefix + "target:husband"},
			models.InlineKeyboardButton{Text: "For Her", CallbackData: reminderWizardCallbackPrefix + "target:wife"},
		)
	}

	buttons := [][]models.InlineKeyboardButton{row, {
		{Text: "Cancel", CallbackData: reminderWizardCallbackPrefix + "cancel"},
	}}

	return &models.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

func reminderWizardScheduleKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "Daily", CallbackData: reminderWizardCallbackPrefix + "schedule:daily"},
			{Text: "One-Time", CallbackData: reminderWizardCallbackPrefix + "schedule:once"},
		},
		{
			{Text: "Cancel", CallbackData: reminderWizardCallbackPrefix + "cancel"},
		},
	}}
}

func reminderWizardDailyTimeKeyboard() *models.InlineKeyboardMarkup {
	rows := [][]string{
		{"07:30", "08:30", "09:00"},
		{"12:00", "15:00", "19:30"},
		{"21:00"},
	}

	buttons := make([][]models.InlineKeyboardButton, 0, len(rows)+1)
	for _, row := range rows {
		line := make([]models.InlineKeyboardButton, 0, len(row))
		for _, value := range row {
			line = append(line, models.InlineKeyboardButton{Text: value, CallbackData: reminderWizardCallbackPrefix + "time_daily:" + value})
		}
		buttons = append(buttons, line)
	}

	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: "Type Time", CallbackData: reminderWizardCallbackPrefix + "time_daily:manual"},
		{Text: "Cancel", CallbackData: reminderWizardCallbackPrefix + "cancel"},
	})

	return &models.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

func reminderWizardOneTimeKeyboard(now time.Time) *models.InlineKeyboardMarkup {
	options := reminderWizardOneTimeOptions(now)
	buttons := make([][]models.InlineKeyboardButton, 0, 3)
	row := make([]models.InlineKeyboardButton, 0, 2)
	for i, option := range options {
		row = append(row, models.InlineKeyboardButton{Text: option.Label, CallbackData: reminderWizardCallbackPrefix + "time_once:" + option.Value})
		if (i+1)%2 == 0 {
			buttons = append(buttons, row)
			row = make([]models.InlineKeyboardButton, 0, 2)
		}
	}
	if len(row) > 0 {
		buttons = append(buttons, row)
	}

	buttons = append(buttons, []models.InlineKeyboardButton{
		{Text: "Custom Date", CallbackData: reminderWizardCallbackPrefix + "time_once:manual"},
		{Text: "Cancel", CallbackData: reminderWizardCallbackPrefix + "cancel"},
	})

	return &models.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

type reminderWizardOneTimeOption struct {
	Label string
	Value string
}

func reminderWizardOneTimeOptions(now time.Time) []reminderWizardOneTimeOption {
	local := now.Local()
	inOneHour := local.Add(time.Hour)
	tonight := time.Date(local.Year(), local.Month(), local.Day(), 21, 0, 0, 0, local.Location())
	if !tonight.After(local.Add(15 * time.Minute)) {
		tonight = tonight.Add(24 * time.Hour)
	}
	mananaMorning := time.Date(local.Year(), local.Month(), local.Day(), 9, 0, 0, 0, local.Location()).Add(24 * time.Hour)

	return []reminderWizardOneTimeOption{
		{Label: fmt.Sprintf("In 1 hour (%s)", inOneHour.Format("15:04")), Value: formatReminderWizardOnceValue(inOneHour)},
		{Label: fmt.Sprintf("Tonight %s", tonight.Format("15:04")), Value: formatReminderWizardOnceValue(tonight)},
		{Label: fmt.Sprintf("Tomorrow %s", mananaMorning.Format("15:04")), Value: formatReminderWizardOnceValue(mananaMorning)},
	}
}

func formatReminderWizardOnceValue(t time.Time) string {
	return t.Format("2006-01-02T15:04")
}

func parseReminderWizardDailyTime(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("empty time")
	}

	if _, err := time.Parse("15:04", trimmed); err != nil {
		return "", err
	}

	return trimmed, nil
}

func parseReminderWizardOnceTime(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("empty time")
	}

	now := time.Now()
	if strings.Count(trimmed, " ") >= 1 {
		dt, err := time.ParseInLocation("2006-01-02 15:04", trimmed, now.Location())
		if err != nil {
			return "", err
		}
		if !dt.After(now) {
			return "", fmt.Errorf("time in past")
		}
		return dt.Format("2006-01-02 15:04"), nil
	}

	timeOnly, err := time.ParseInLocation("15:04", trimmed, now.Location())
	if err != nil {
		return "", err
	}
	candidate := time.Date(now.Year(), now.Month(), now.Day(), timeOnly.Hour(), timeOnly.Minute(), 0, 0, now.Location())
	if !candidate.After(now) {
		candidate = candidate.Add(24 * time.Hour)
	}
	return candidate.Format("2006-01-02 15:04"), nil
}

func isReminderWizardCancel(text string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(text))
	return trimmed == "cancel" || trimmed == "/cancel"
}
