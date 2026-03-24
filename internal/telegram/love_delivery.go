package telegram

import (
	"context"
	"log/slog"
	"strings"

	"fambow/internal/service"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type LoveNoteSender interface {
	SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)
	SendPhoto(ctx context.Context, params *bot.SendPhotoParams) (*models.Message, error)
}

const emptyLoveNotesMessage = "No love notes yet. Add one with /add_love."

func SendLoveNotesEmptyState(ctx context.Context, sender LoveNoteSender, chatID int64, replyMarkup any, logger *slog.Logger) bool {
	if sender == nil {
		return false
	}

	_, err := sender.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        emptyLoveNotesMessage,
		ReplyMarkup: replyMarkup,
	})
	if err != nil {
		logger.Error("failed to send empty love notes message", "chat_id", chatID, "error", err)
		return false
	}

	return true
}

func SendLoveNote(ctx context.Context, sender LoveNoteSender, chatID int64, note service.LoveNote, replyMarkup any, logger *slog.Logger) bool {
	if sender == nil {
		return false
	}

	if strings.TrimSpace(note.TelegramFileID) != "" {
		return sendLovePhoto(ctx, sender, chatID, note, replyMarkup, logger)
	}

	_, err := sender.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        note.Text,
		ReplyMarkup: replyMarkup,
	})
	if err != nil {
		logger.Error("failed to send love note", "chat_id", chatID, "error", err)
		return false
	}

	return true
}

func sendLovePhoto(ctx context.Context, sender LoveNoteSender, chatID int64, note service.LoveNote, replyMarkup any, logger *slog.Logger) bool {
	caption := strings.TrimSpace(note.Text)
	if caption == "" {
		if _, err := sender.SendPhoto(ctx, &bot.SendPhotoParams{
			ChatID:      chatID,
			Photo:       &models.InputFileString{Data: note.TelegramFileID},
			ReplyMarkup: replyMarkup,
		}); err != nil {
			logger.Error("failed to send love note photo", "chat_id", chatID, "error", err)
			return false
		}
		return true
	}

	if isLovePhotoCaptionTooLong(caption) {
		return sendLovePhotoWithTextFallback(ctx, sender, chatID, note, replyMarkup, logger)
	}

	if _, err := sender.SendPhoto(ctx, &bot.SendPhotoParams{
		ChatID:      chatID,
		Photo:       &models.InputFileString{Data: note.TelegramFileID},
		Caption:     caption,
		ReplyMarkup: replyMarkup,
	}); err != nil {
		logger.Error("failed to send love note photo", "chat_id", chatID, "error", err)
		return sendLovePhotoWithTextFallback(ctx, sender, chatID, note, replyMarkup, logger)
	}

	return true
}

func sendLovePhotoWithTextFallback(ctx context.Context, sender LoveNoteSender, chatID int64, note service.LoveNote, replyMarkup any, logger *slog.Logger) bool {
	if _, err := sender.SendPhoto(ctx, &bot.SendPhotoParams{
		ChatID:      chatID,
		Photo:       &models.InputFileString{Data: note.TelegramFileID},
		ReplyMarkup: replyMarkup,
	}); err != nil {
		logger.Error("failed to send love note fallback photo", "chat_id", chatID, "error", err)
		return false
	}

	_, err := sender.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        strings.TrimSpace(note.Text),
		ReplyMarkup: replyMarkup,
	})
	if err != nil {
		logger.Error("failed to send love note fallback text", "chat_id", chatID, "error", err)
		return false
	}

	return true
}
