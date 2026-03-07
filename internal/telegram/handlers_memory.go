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

func memoryHandler(logger *slog.Logger, memories MemoryProvider) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		text := extractCommandPayload(update.Message.Text, "/memory")

		if update.Message.From == nil {
			sendText(ctx, b, update.Message.Chat.ID, "I could not read your profile info for this message. Please try again.", logger, "/memory missing sender")
			return
		}

		if memories == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Memory feature is not configured yet.", logger, "/memory unavailable")
			return
		}

		_, err := memories.AddMemory(ctx, update.Message.From.ID, update.Message.From.FirstName, service.MemoryInput{Text: text})
		if err != nil {
			if errors.Is(err, service.ErrMemoryContentEmpty) || errors.Is(err, service.ErrMemoryTextEmpty) {
				sendText(ctx, b, update.Message.Chat.ID, "Your memory looks empty. Try /memory followed by a sentence or attach a photo with /memory caption.", logger, "/memory empty")
				return
			}

			if errors.Is(err, service.ErrMemoryDateFormat) || errors.Is(err, service.ErrMemoryDateInFuture) {
				sendText(ctx, b, update.Message.Chat.ID, service.MemoryUsage(), logger, "/memory usage")
				return
			}

			logger.Error("failed to save memory", "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not save that memory right now. Please try again in a moment.", logger, "/memory save failed")
			return
		}

		sendText(ctx, b, update.Message.Chat.ID, "Memory saved. I will keep this moment safe for you.", logger, "/memory saved")
	}
}

func memoryPhotoHandler(logger *slog.Logger, memories MemoryProvider) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if !strings.HasPrefix(strings.TrimSpace(update.Message.Caption), "/memory") {
			return
		}

		if len(update.Message.Photo) == 0 {
			return
		}

		if update.Message.From == nil {
			sendText(ctx, b, update.Message.Chat.ID, "I could not read your profile info for this message. Please try again.", logger, "/memory missing sender")
			return
		}

		if memories == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Memory feature is not configured yet.", logger, "/memory unavailable")
			return
		}

		telegramFileID, telegramFileUnique := pickLargestPhoto(update.Message.Photo)
		if telegramFileID == "" {
			sendText(ctx, b, update.Message.Chat.ID, "I could not read the attached photo. Please try sending it again.", logger, "/memory photo missing")
			return
		}

		text := extractCommandPayload(update.Message.Caption, "/memory")
		_, err := memories.AddMemory(ctx, update.Message.From.ID, update.Message.From.FirstName, service.MemoryInput{
			Text:               text,
			TelegramFileID:     telegramFileID,
			TelegramFileUnique: telegramFileUnique,
		})
		if err != nil {
			if errors.Is(err, service.ErrMemoryContentEmpty) || errors.Is(err, service.ErrMemoryTextEmpty) {
				sendText(ctx, b, update.Message.Chat.ID, "Your memory looks empty. Add text in caption or attach a photo with /memory.", logger, "/memory empty")
				return
			}

			if errors.Is(err, service.ErrMemoryDateFormat) || errors.Is(err, service.ErrMemoryDateInFuture) {
				sendText(ctx, b, update.Message.Chat.ID, service.MemoryUsage(), logger, "/memory usage")
				return
			}

			logger.Error("failed to save memory photo", "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not save that memory right now. Please try again in a moment.", logger, "/memory photo save failed")
			return
		}

		sendText(ctx, b, update.Message.Chat.ID, "Memory with photo saved. I will keep this moment safe for you.", logger, "/memory photo saved")
	}
}

func memoriesHandler(logger *slog.Logger, memories MemoryProvider) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil || update.Message.From == nil {
			return
		}

		if memories == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Memory feature is not configured yet.", logger, "/memories unavailable")
			return
		}

		records, err := memories.RecentMemories(ctx, update.Message.From.ID, 3)
		if err != nil {
			logger.Error("failed to list memories", "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not load memories right now. Please try again in a moment.", logger, "/memories load failed")
			return
		}

		if len(records) == 0 {
			sendText(ctx, b, update.Message.Chat.ID, "No saved memories yet. Add one with /memory <text> or photo caption /memory.", logger, "/memories empty")
			return
		}

		sendText(ctx, b, update.Message.Chat.ID, "Your recent memories:", logger, "/memories sent")

		for _, record := range records {
			entry := formatMemoryLine(record)
			if record.TelegramFileID != "" {
				_, sendErr := b.SendPhoto(ctx, &bot.SendPhotoParams{
					ChatID:  update.Message.Chat.ID,
					Photo:   &models.InputFileString{Data: record.TelegramFileID},
					Caption: entry,
				})
				if sendErr != nil {
					logger.Warn("failed to send memory photo", "error", sendErr)
					sendText(ctx, b, update.Message.Chat.ID, "[photo unavailable]", logger, "/memories sent")
				}
				continue
			}

			sendText(ctx, b, update.Message.Chat.ID, entry, logger, "/memories sent")
		}
	}
}

func pickLargestPhoto(items []models.PhotoSize) (string, string) {
	if len(items) == 0 {
		return "", ""
	}

	best := items[0]
	for _, item := range items[1:] {
		if item.FileSize > best.FileSize {
			best = item
		}
	}

	return strings.TrimSpace(best.FileID), strings.TrimSpace(best.FileUniqueID)
}

func formatMemoryLine(memory service.Memory) string {
	text := strings.TrimSpace(memory.Text)
	if text == "" {
		text = "Photo memory"
	}

	return fmt.Sprintf("%s (%s)", text, memory.CreatedAt.Format("2006-01-02"))
}

func sendText(ctx context.Context, b *bot.Bot, chatID int64, text string, logger *slog.Logger, logKey string) {
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text}); err != nil {
		logger.Error("failed to send message", "context", logKey, "error", err)
	}
}
