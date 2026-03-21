package telegram

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"fambow/internal/service"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const telegramPhotoCaptionLimit = 1024

func addLoveNoteHandler(logger *slog.Logger, loveNotes LoveNoteProvider) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if loveNotes == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Love note feature is not configured yet.", logger, "/add_love unavailable")
			return
		}

		note := extractCommandPayload(update.Message.Text, "/add_love")
		if note == "" {
			sendText(ctx, b, update.Message.Chat.ID, "Please provide a love note to add.", logger, "/add_love empty")
			return
		}

		if !saveLoveNote(ctx, b, update.Message.Chat.ID, loveNotes, service.LoveNoteInput{Text: note}, logger) {
			return
		}

		sendText(ctx, b, update.Message.Chat.ID, "Love note added successfully!", logger, "/add_love saved")
	}
}

func addLoveNotePhotoHandler(logger *slog.Logger, loveNotes LoveNoteProvider) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if !strings.HasPrefix(strings.TrimSpace(update.Message.Caption), "/add_love") || len(update.Message.Photo) == 0 {
			return
		}

		if loveNotes == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Love note feature is not configured yet.", logger, "/add_love photo unavailable")
			return
		}

		telegramFileID, telegramFileUnique := pickLargestPhoto(update.Message.Photo)
		if telegramFileID == "" {
			sendText(ctx, b, update.Message.Chat.ID, "I could not read the attached photo. Please try sending it again.", logger, "/add_love photo missing")
			return
		}

		if !saveLoveNote(ctx, b, update.Message.Chat.ID, loveNotes, service.LoveNoteInput{
			Text:               extractCommandPayload(update.Message.Caption, "/add_love"),
			TelegramFileID:     telegramFileID,
			TelegramFileUnique: telegramFileUnique,
		}, logger) {
			return
		}

		sendText(ctx, b, update.Message.Chat.ID, "Love note added successfully!", logger, "/add_love photo saved")
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

		note := service.LoveNote{Text: "You are loved so much, my love."}
		if loveNotes != nil {
			result, err := loveNotes.RandomNote(ctx, firstName)
			if err != nil {
				logger.Error("failed to fetch love note", "error", err)
			} else {
				note = result
			}
		}

		if strings.TrimSpace(note.TelegramFileID) != "" {
			sendLovePhoto(ctx, b, update.Message.Chat.ID, note, logger)
			return
		}

		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      update.Message.Chat.ID,
			Text:        note.Text,
			ReplyMarkup: commandKeyboard(),
		})
		if err != nil {
			logger.Error("failed to send /love response", "error", err)
		}
	}
}

func saveLoveNote(ctx context.Context, b *bot.Bot, chatID int64, loveNotes LoveNoteProvider, input service.LoveNoteInput, logger *slog.Logger) bool {
	err := loveNotes.AddLoveNote(ctx, input)
	if err != nil {
		if errors.Is(err, service.ErrLoveNoteContentEmpty) {
			sendText(ctx, b, chatID, "Please provide a love note to add.", logger, "/add_love empty")
			return false
		}

		logger.Error("failed to add love note", "error", err)
		sendText(ctx, b, chatID, "I could not save that love note right now. Please try again in a moment.", logger, "/add_love save failed")
		return false
	}

	return true
}

func sendLovePhoto(ctx context.Context, b *bot.Bot, chatID int64, note service.LoveNote, logger *slog.Logger) {
	caption := strings.TrimSpace(note.Text)
	if caption == "" {
		if _, err := b.SendPhoto(ctx, &bot.SendPhotoParams{
			ChatID:      chatID,
			Photo:       &models.InputFileString{Data: note.TelegramFileID},
			ReplyMarkup: commandKeyboard(),
		}); err != nil {
			logger.Error("failed to send /love photo response", "error", err)
		}
		return
	}

	if isLovePhotoCaptionTooLong(caption) {
		sendLovePhotoWithTextFallback(ctx, b, chatID, note, logger)
		return
	}

	if _, err := b.SendPhoto(ctx, &bot.SendPhotoParams{
		ChatID:      chatID,
		Photo:       &models.InputFileString{Data: note.TelegramFileID},
		Caption:     caption,
		ReplyMarkup: commandKeyboard(),
	}); err != nil {
		logger.Error("failed to send /love photo response", "error", err)
		sendLovePhotoWithTextFallback(ctx, b, chatID, note, logger)
	}
}

func sendLovePhotoWithTextFallback(ctx context.Context, b *bot.Bot, chatID int64, note service.LoveNote, logger *slog.Logger) {
	if _, err := b.SendPhoto(ctx, &bot.SendPhotoParams{
		ChatID:      chatID,
		Photo:       &models.InputFileString{Data: note.TelegramFileID},
		ReplyMarkup: commandKeyboard(),
	}); err != nil {
		logger.Error("failed to send /love fallback photo response", "error", err)
		return
	}

	sendText(ctx, b, chatID, strings.TrimSpace(note.Text), logger, "/love photo fallback text")
}

func isLovePhotoCaptionTooLong(caption string) bool {
	return len([]rune(caption)) > telegramPhotoCaptionLimit
}
