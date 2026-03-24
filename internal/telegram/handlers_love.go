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

		if loveNotes == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Love note feature is not configured yet.", logger, "/love unavailable")
			return
		}

		firstName := ""
		if update.Message.From != nil {
			firstName = update.Message.From.FirstName
		}

		note, err := loveNotes.RandomNote(ctx, firstName)
		if err != nil {
			if errors.Is(err, service.ErrLoveNotesEmpty) {
				SendLoveNotesEmptyState(ctx, b, update.Message.Chat.ID, commandKeyboard(), logger)
				return
			}

			logger.Error("failed to fetch love note", "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not load a love note right now. Please try again in a moment.", logger, "/love failed")
			return
		}

		SendLoveNote(ctx, b, update.Message.Chat.ID, note, commandKeyboard(), logger)
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

func isLovePhotoCaptionTooLong(caption string) bool {
	return len([]rune(caption)) > telegramPhotoCaptionLimit
}
