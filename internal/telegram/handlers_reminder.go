package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"fambow/internal/service"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

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
