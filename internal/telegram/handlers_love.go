package telegram

import (
	"context"
	"log/slog"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func addLoveNoteHandler(logger *slog.Logger, loveNotes LoveNoteProvider) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		note := extractAddLoveNote(update.Message.Text)

		err := loveNotes.AddLoveNote(ctx, note)
		if err != nil {
			logger.Error("failed to add love note", "error", err)
		}

		if _, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: update.Message.Chat.ID,
			Text:   "Love note added successfully!",
		}); err != nil {
			logger.Error("failed to send /add_love response", "error", err)
		}
	}
}

func loveHandler(logger *slog.Logger, loveNotes LoveNoteProvider) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		firstName := ""
		if update.Message.From != nil {
			firstName = update.Message.From.FirstName
		}

		text := "You are loved so much, my love."
		if loveNotes != nil {
			note, err := loveNotes.RandomNote(ctx, firstName)
			if err != nil {
				logger.Error("failed to fetch love note", "error", err)
			} else {
				text = note
			}
		}

		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      update.Message.Chat.ID,
			Text:        text,
			ReplyMarkup: commandKeyboard(),
		})
		if err != nil {
			logger.Error("failed to send /love response", "error", err)
		}
	}
}

func extractAddLoveNote(message string) string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return ""
	}

	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return ""
	}

	if strings.HasPrefix(parts[0], "/add_love") {
		prefix := parts[0]
		return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
	}

	return ""
}
