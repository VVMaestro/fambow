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

func eventHandler(logger *slog.Logger, celebrations CelebrationProvider) bot.HandlerFunc {
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

		message := fmt.Sprintf("Event saved: %s on %s (remind %d day(s) before).", event.Title, event.EventDate.Format("2006-01-02"), event.RemindDaysBefore)
		sendText(ctx, b, update.Message.Chat.ID, message, logger, "/event saved")
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
