package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"fambow/internal/service"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const eventWizardCallbackPrefix = "ew:"

func eventHandler(logger *slog.Logger, celebrations CelebrationProvider, wizard *eventWizardState) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if update.Message.From == nil {
			sendText(ctx, b, update.Message.Chat.ID, "I could not read your profile info for this event. Please try again.", logger, "/event missing sender")
			return
		}

		if celebrations == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Celebration feature is not configured yet.", logger, "/event unavailable")
			return
		}

		payload := extractCommandPayload(update.Message.Text, "/event")
		if payload == "" {
			startEventWizard(ctx, b, logger, update.Message.Chat.ID, update.Message.From.ID, celebrations, wizard)
			return
		}

		event, err := celebrations.AddEvent(ctx, update.Message.From.ID, update.Message.From.FirstName, payload)
		if err != nil {
			if errors.Is(err, service.ErrEventCommandEmpty) ||
				errors.Is(err, service.ErrEventInvalidFormat) ||
				errors.Is(err, service.ErrEventDateFormat) ||
				errors.Is(err, service.ErrEventTitleEmpty) ||
				errors.Is(err, service.ErrEventDaysBefore) ||
				errors.Is(err, service.ErrEventNotAddCommand) ||
				errors.Is(err, service.ErrEventDateInPast) {
				sendText(ctx, b, update.Message.Chat.ID, service.EventUsage(), logger, "/event usage")
				return
			}

			logger.Error("failed to add event", "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not save your event right now. Please try again in a moment.", logger, "/event save failed")
			return
		}

		if wizard != nil {
			wizard.Delete(update.Message.From.ID)
		}

		message := fmt.Sprintf("Event saved: %s on %s (remind %d day(s) before).", event.Title, event.EventDate.Format("2006-01-02"), event.RemindDaysBefore)
		sendText(ctx, b, update.Message.Chat.ID, message, logger, "/event saved")
	}
}

func eventWizardStartHandler(logger *slog.Logger, celebrations CelebrationProvider, wizard *eventWizardState) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if update.Message.From == nil {
			sendText(ctx, b, update.Message.Chat.ID, "I could not read your profile info for this event. Please try again.", logger, "event button missing sender")
			return
		}

		startEventWizard(ctx, b, logger, update.Message.Chat.ID, update.Message.From.ID, celebrations, wizard)
	}
}

func startEventWizard(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64, userID int64, celebrations CelebrationProvider, wizard *eventWizardState) {
	if celebrations == nil {
		sendText(ctx, b, chatID, "Celebration feature is not configured yet.", logger, "/event unavailable")
		return
	}
	if wizard == nil {
		sendText(ctx, b, chatID, "Celebration feature is not configured yet.", logger, "/event wizard unavailable")
		return
	}

	wizard.Start(userID)
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        "Let's plan a celebration together.\n\nStep 1: when is the event?\n(You can type cancel anytime to stop.)",
		ReplyMarkup: eventWizardDateKeyboard(),
	})
	if err != nil {
		logger.Error("failed to send event wizard start", "error", err)
	}
}

func eventsHandler(logger *slog.Logger, celebrations CelebrationProvider) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil || update.Message.From == nil {
			return
		}

		if celebrations == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Celebration feature is not configured yet.", logger, "/events unavailable")
			return
		}

		events, err := celebrations.ListEvents(ctx, update.Message.From.ID)
		if err != nil {
			logger.Error("failed listing events", "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not load events right now. Please try again in a moment.", logger, "/events load failed")
			return
		}

		if len(events) == 0 {
			sendText(ctx, b, update.Message.Chat.ID, "No celebration dates yet. Add one with /event add.", logger, "/events empty")
			return
		}

		lines := make([]string, 0, len(events)+1)
		lines = append(lines, "Your celebration dates:")
		for _, event := range events {
			lines = append(lines, fmt.Sprintf("- %s on %s (remind %d day(s) before)", event.Title, event.EventDate.Format("2006-01-02"), event.RemindDaysBefore))
		}

		sendText(ctx, b, update.Message.Chat.ID, strings.Join(lines, "\n"), logger, "/events sent")
	}
}

func eventWizardCallbackHandler(logger *slog.Logger, celebrations CelebrationProvider, wizard *eventWizardState) bot.HandlerFunc {
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

		if celebrations == nil {
			if chatID != 0 {
				sendText(ctx, b, chatID, "Celebration feature is not configured yet.", logger, "event wizard unavailable")
			}
			answerCallbackQuery(ctx, b, update, logger, "")
			return
		}

		payload := strings.TrimPrefix(update.CallbackQuery.Data, eventWizardCallbackPrefix)
		action := payload
		value := ""
		if idx := strings.Index(payload, ":"); idx >= 0 {
			action = payload[:idx]
			value = payload[idx+1:]
		}

		session, ok := wizard.Get(user.ID)
		if !ok {
			if chatID != 0 {
				sendText(ctx, b, chatID, "Event flow expired. Send /event to start again.", logger, "event wizard missing session")
			}
			answerCallbackQuery(ctx, b, update, logger, "")
			return
		}

		switch action {
		case "cancel":
			wizard.Delete(user.ID)
			if chatID != 0 {
				sendText(ctx, b, chatID, "Event flow canceled.", logger, "event wizard cancel")
			}
		case "date":
			eventWizardHandleDateChoice(ctx, b, logger, chatID, user.ID, value, wizard, session)
		case "days":
			eventWizardHandleDaysChoice(ctx, b, logger, chatID, user, value, celebrations, wizard, session)
		default:
			if chatID != 0 {
				sendText(ctx, b, chatID, "I did not understand that choice. Please try again.", logger, "event wizard unknown action")
			}
		}

		answerCallbackQuery(ctx, b, update, logger, "")
	}
}

func eventWizardMessageHandler(logger *slog.Logger, celebrations CelebrationProvider, wizard *eventWizardState) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil || update.Message.From == nil {
			return
		}

		if celebrations == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Celebration feature is not configured yet.", logger, "event wizard unavailable")
			return
		}

		session, ok := wizard.Get(update.Message.From.ID)
		if !ok {
			return
		}

		text := strings.TrimSpace(update.Message.Text)
		if isEventWizardCancel(text) {
			wizard.Delete(update.Message.From.ID)
			sendText(ctx, b, update.Message.Chat.ID, "Event flow canceled.", logger, "event wizard cancel")
			return
		}

		switch session.Step {
		case eventWizardStepAwaitDate:
			eventWizardHandleCustomDate(ctx, b, logger, update.Message.Chat.ID, update.Message.From.ID, text, wizard, session)
		case eventWizardStepAwaitTitle:
			eventWizardHandleTitle(ctx, b, logger, update.Message.Chat.ID, update.Message.From.ID, text, wizard, session)
		case eventWizardStepAwaitDays:
			eventWizardHandleManualDays(ctx, b, logger, update, celebrations, wizard, session)
		}
	}
}

func eventWizardMatch(wizard *eventWizardState) bot.MatchFunc {
	return func(update *models.Update) bool {
		if wizard == nil || update.Message == nil || update.Message.From == nil {
			return false
		}

		session, ok := wizard.Get(update.Message.From.ID)
		if !ok {
			return false
		}

		if session.Step != eventWizardStepAwaitDate && session.Step != eventWizardStepAwaitTitle && session.Step != eventWizardStepAwaitDays {
			return false
		}

		text := strings.TrimSpace(update.Message.Text)
		if text == "" {
			return true
		}

		if strings.HasPrefix(text, "/") && !isEventWizardCancel(text) {
			return false
		}

		return true
	}
}

func eventWizardHandleDateChoice(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64, userID int64, value string, wizard *eventWizardState, session eventWizardSession) {
	switch value {
	case "today":
		session.EventDate = time.Now().Local().Format("2006-01-02")
		session.Step = eventWizardStepAwaitTitle
		wizard.Set(userID, session)
		sendText(ctx, b, chatID, "Step 2: send the event title (e.g., 'Anniversary dinner').", logger, "event wizard today")
	case "tomorrow":
		session.EventDate = time.Now().Local().Add(24 * time.Hour).Format("2006-01-02")
		session.Step = eventWizardStepAwaitTitle
		wizard.Set(userID, session)
		sendText(ctx, b, chatID, "Step 2: send the event title (e.g., 'Anniversary dinner').", logger, "event wizard tomorrow")
	case "custom":
		session.Step = eventWizardStepAwaitDate
		wizard.Set(userID, session)
		sendText(ctx, b, chatID, "Send the date as YYYY-MM-DD. Example: 2026-09-12", logger, "event wizard custom date")
	default:
		sendText(ctx, b, chatID, "Please choose Today, Tomorrow, or Custom Date.", logger, "event wizard date invalid")
	}
}

func eventWizardHandleCustomDate(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64, userID int64, text string, wizard *eventWizardState, session eventWizardSession) {
	eventDate, err := parseEventWizardDate(text)
	if err != nil {
		if errors.Is(err, service.ErrEventDateInPast) {
			sendText(ctx, b, chatID, "That date is in the past. Send today or a future date.", logger, "event wizard past date")
			return
		}

		sendText(ctx, b, chatID, "Send the date as YYYY-MM-DD. Example: 2026-09-12", logger, "event wizard invalid date")
		return
	}

	session.EventDate = eventDate
	session.Step = eventWizardStepAwaitTitle
	wizard.Set(userID, session)
	sendText(ctx, b, chatID, "Step 2: send the event title (e.g., 'Anniversary dinner').", logger, "event wizard date set")
}

func eventWizardHandleTitle(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64, userID int64, text string, wizard *eventWizardState, session eventWizardSession) {
	title := strings.TrimSpace(text)
	if title == "" {
		sendText(ctx, b, chatID, "Event title cannot be empty. Send a short title.", logger, "event wizard empty title")
		return
	}

	session.Title = title
	session.Step = eventWizardStepSelectDays
	wizard.Set(userID, session)
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        "Step 3: how many days before should I remind you?",
		ReplyMarkup: eventWizardDaysKeyboard(),
	})
	if err != nil {
		logger.Error("failed to send event days keyboard", "error", err)
	}
}

func eventWizardHandleDaysChoice(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64, user *models.User, value string, celebrations CelebrationProvider, wizard *eventWizardState, session eventWizardSession) {
	if value == "manual" {
		session.Step = eventWizardStepAwaitDays
		wizard.Set(user.ID, session)
		sendText(ctx, b, chatID, "Send a positive number of days. Example: 3", logger, "event wizard manual days")
		return
	}

	days, err := parseEventWizardDays(value)
	if err != nil {
		sendText(ctx, b, chatID, "Send a positive number of days. Example: 3", logger, "event wizard invalid preset days")
		return
	}

	session.RemindDaysBefore = days
	eventWizardSave(ctx, b, logger, chatID, user, celebrations, wizard, session)
}

func eventWizardHandleManualDays(ctx context.Context, b *bot.Bot, logger *slog.Logger, update *models.Update, celebrations CelebrationProvider, wizard *eventWizardState, session eventWizardSession) {
	days, err := parseEventWizardDays(update.Message.Text)
	if err != nil {
		sendText(ctx, b, update.Message.Chat.ID, "Send a positive number of days. Example: 3", logger, "event wizard invalid days")
		return
	}

	session.RemindDaysBefore = days
	eventWizardSave(ctx, b, logger, update.Message.Chat.ID, update.Message.From, celebrations, wizard, session)
}

func eventWizardSave(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64, user *models.User, celebrations CelebrationProvider, wizard *eventWizardState, session eventWizardSession) {
	if user == nil {
		sendText(ctx, b, chatID, "I could not read your profile info for this event. Please try again.", logger, "event wizard missing sender")
		return
	}

	command := buildEventWizardCommand(session)
	if command == "" {
		sendText(ctx, b, chatID, "I could not understand that event. Start again with /event.", logger, "event wizard command empty")
		wizard.Delete(user.ID)
		return
	}

	event, err := celebrations.AddEvent(ctx, user.ID, user.FirstName, command)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEventDateInPast):
			session.Step = eventWizardStepAwaitDate
			wizard.Set(user.ID, session)
			sendText(ctx, b, chatID, "That date is in the past. Send today or a future date.", logger, "event wizard save date past")
			return
		case errors.Is(err, service.ErrEventDateFormat):
			session.Step = eventWizardStepAwaitDate
			wizard.Set(user.ID, session)
			sendText(ctx, b, chatID, "Send the date as YYYY-MM-DD. Example: 2026-09-12", logger, "event wizard save date invalid")
			return
		case errors.Is(err, service.ErrEventTitleEmpty):
			session.Step = eventWizardStepAwaitTitle
			wizard.Set(user.ID, session)
			sendText(ctx, b, chatID, "Event title cannot be empty. Send a short title.", logger, "event wizard save title empty")
			return
		case errors.Is(err, service.ErrEventDaysBefore):
			session.Step = eventWizardStepAwaitDays
			wizard.Set(user.ID, session)
			sendText(ctx, b, chatID, "Send a positive number of days. Example: 3", logger, "event wizard save days invalid")
			return
		}

		logger.Error("failed to save wizard event", "error", err)
		sendText(ctx, b, chatID, "I could not save your event right now. Please try again in a moment.", logger, "event wizard save failed")
		wizard.Delete(user.ID)
		return
	}

	wizard.Delete(user.ID)
	message := fmt.Sprintf("Event saved: %s on %s (remind %d day(s) before).", event.Title, event.EventDate.Format("2006-01-02"), event.RemindDaysBefore)
	sendText(ctx, b, chatID, message, logger, "event wizard saved")
}

func buildEventWizardCommand(session eventWizardSession) string {
	if strings.TrimSpace(session.EventDate) == "" || strings.TrimSpace(session.Title) == "" || session.RemindDaysBefore <= 0 {
		return ""
	}

	return fmt.Sprintf("add %s | %s | %d", strings.TrimSpace(session.EventDate), strings.TrimSpace(session.Title), session.RemindDaysBefore)
}

func eventWizardDateKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "Today", CallbackData: eventWizardCallbackPrefix + "date:today"},
			{Text: "Tomorrow", CallbackData: eventWizardCallbackPrefix + "date:tomorrow"},
		},
		{
			{Text: "Custom Date", CallbackData: eventWizardCallbackPrefix + "date:custom"},
		},
		{
			{Text: "Cancel", CallbackData: eventWizardCallbackPrefix + "cancel"},
		},
	}}
}

func eventWizardDaysKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "1 day", CallbackData: eventWizardCallbackPrefix + "days:1"},
			{Text: "3 days", CallbackData: eventWizardCallbackPrefix + "days:3"},
		},
		{
			{Text: "7 days", CallbackData: eventWizardCallbackPrefix + "days:7"},
			{Text: "Type Number", CallbackData: eventWizardCallbackPrefix + "days:manual"},
		},
		{
			{Text: "Cancel", CallbackData: eventWizardCallbackPrefix + "cancel"},
		},
	}}
}

func parseEventWizardDate(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", service.ErrEventDateFormat
	}

	now := time.Now()
	parsedDate, err := time.ParseInLocation("2006-01-02", trimmed, now.Location())
	if err != nil {
		return "", service.ErrEventDateFormat
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if parsedDate.Before(today) {
		return "", service.ErrEventDateInPast
	}

	return parsedDate.Format("2006-01-02"), nil
}

func parseEventWizardDays(value string) (int, error) {
	days, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || days <= 0 {
		return 0, service.ErrEventDaysBefore
	}

	return days, nil
}

func isEventWizardCancel(text string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(text))
	return trimmed == "cancel" || trimmed == "/cancel"
}
