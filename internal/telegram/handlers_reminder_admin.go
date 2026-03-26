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

const reminderAdminMessageLimit = 3500

func listAllRemindersHandler(logger *slog.Logger, reminders ReminderProvider, adminTelegramUserID int64) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if !isAdminMessage(update.Message.From, adminTelegramUserID) {
			sendText(ctx, b, update.Message.Chat.ID, "Only admin can use reminder admin commands.", logger, "/list_reminders forbidden")
			return
		}

		if reminders == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Reminder feature is not configured yet.", logger, "/list_reminders unavailable")
			return
		}

		isActive, err := parseReminderListFilter(extractCommandPayload(update.Message.Text, "/list_reminders"))
		if err != nil {
			sendText(ctx, b, update.Message.Chat.ID, reminderListUsage(), logger, "/list_reminders usage")
			return
		}

		items, err := reminders.ListRemindersByActiveState(ctx, isActive)
		if err != nil {
			logger.Error("failed listing admin reminders", "is_active", isActive, "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not load reminders right now. Please try again in a moment.", logger, "/list_reminders failed")
			return
		}

		if len(items) == 0 {
			if isActive {
				sendText(ctx, b, update.Message.Chat.ID, "No active reminders yet.", logger, "/list_reminders empty active")
				return
			}
			sendText(ctx, b, update.Message.Chat.ID, "No inactive reminders yet.", logger, "/list_reminders empty inactive")
			return
		}

		for _, chunk := range buildAdminReminderListMessages(items, isActive, reminderAdminMessageLimit) {
			sendText(ctx, b, update.Message.Chat.ID, chunk, logger, "/list_reminders sent")
		}
	}
}

func removeReminderHandler(logger *slog.Logger, reminders ReminderProvider, adminTelegramUserID int64) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if !isAdminMessage(update.Message.From, adminTelegramUserID) {
			sendText(ctx, b, update.Message.Chat.ID, "Only admin can use reminder admin commands.", logger, "/remove_reminder forbidden")
			return
		}

		if reminders == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Reminder feature is not configured yet.", logger, "/remove_reminder unavailable")
			return
		}

		reminderID, err := parseReminderRemovePayload(extractCommandPayload(update.Message.Text, "/remove_reminder"))
		if err != nil {
			sendText(ctx, b, update.Message.Chat.ID, removeReminderUsage(), logger, "/remove_reminder usage")
			return
		}

		if err := reminders.RemoveReminder(ctx, reminderID); err != nil {
			if errors.Is(err, service.ErrReminderIDInvalid) {
				sendText(ctx, b, update.Message.Chat.ID, removeReminderUsage(), logger, "/remove_reminder invalid id")
				return
			}
			if errors.Is(err, service.ErrReminderNotFound) {
				sendText(ctx, b, update.Message.Chat.ID, "That reminder does not exist or is already inactive.", logger, "/remove_reminder missing")
				return
			}

			logger.Error("failed removing reminder", "reminder_id", reminderID, "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not remove that reminder right now. Please try again in a moment.", logger, "/remove_reminder failed")
			return
		}

		sendText(ctx, b, update.Message.Chat.ID, fmt.Sprintf("Reminder #%d removed.", reminderID), logger, "/remove_reminder removed")
	}
}

func parseReminderListFilter(payload string) (bool, error) {
	trimmed := strings.TrimSpace(strings.ToLower(payload))
	switch trimmed {
	case "":
		return true, nil
	case "inactive":
		return false, nil
	default:
		return false, service.ErrReminderListFilterInvalid
	}
}

func parseReminderRemovePayload(payload string) (int64, error) {
	parts := strings.Fields(strings.TrimSpace(payload))
	if len(parts) != 1 {
		return 0, service.ErrReminderIDInvalid
	}

	reminderID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || reminderID <= 0 {
		return 0, service.ErrReminderIDInvalid
	}

	return reminderID, nil
}

func reminderListUsage() string {
	return "Use this format:\n/list_reminders\n/list_reminders inactive"
}

func removeReminderUsage() string {
	return "Use this format:\n/remove_reminder <id>"
}

func buildAdminReminderListMessages(reminders []service.AdminReminder, isActive bool, limit int) []string {
	if len(reminders) == 0 {
		return nil
	}

	header := "Active reminders:"
	continuationHeader := "Active reminders (cont.):"
	if !isActive {
		header = "Inactive reminders:"
		continuationHeader = "Inactive reminders (cont.):"
	}

	chunks := make([]string, 0, 1)
	current := header
	for _, reminder := range reminders {
		line := fmt.Sprintf("- #%d %s (%s) %s: %s", reminder.ID, reminder.FirstName, reminder.UserType, reminder.ScheduleDisplay, reminder.Text)
		candidate := current + "\n" + line
		if len(candidate) <= limit {
			current = candidate
			continue
		}

		chunks = append(chunks, current)
		current = continuationHeader + "\n" + line
	}

	chunks = append(chunks, current)
	return chunks
}
