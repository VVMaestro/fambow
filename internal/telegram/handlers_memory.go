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

		text := extractMemoryText(update.Message.Text)
		if text == "" {
			sendText(ctx, b, update.Message.Chat.ID, "Please share it like this:\n/memory Today we walked by the river together.", logger, "/memory usage")
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

		_, err := memories.AddMemory(ctx, update.Message.From.ID, update.Message.From.FirstName, text)
		if err != nil {
			if errors.Is(err, service.ErrMemoryTextEmpty) {
				sendText(ctx, b, update.Message.Chat.ID, "Your memory looks empty. Try /memory followed by a sentence.", logger, "/memory empty")
				return
			}

			logger.Error("failed to save memory", "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not save that memory right now. Please try again in a moment.", logger, "/memory save failed")
			return
		}

		sendText(ctx, b, update.Message.Chat.ID, "Memory saved. I will keep this moment safe for you.", logger, "/memory saved")
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

		records, err := memories.RecentMemories(ctx, update.Message.From.ID, 5)
		if err != nil {
			logger.Error("failed to list memories", "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not load memories right now. Please try again in a moment.", logger, "/memories load failed")
			return
		}

		if len(records) == 0 {
			sendText(ctx, b, update.Message.Chat.ID, "No saved memories yet. Add one with /memory <text>.", logger, "/memories empty")
			return
		}

		lines := make([]string, 0, len(records)+1)
		lines = append(lines, "Your recent memories:")
		for _, record := range records {
			lines = append(lines, fmt.Sprintf("- %s (%s)", record.Text, record.CreatedAt.Format("2006-01-02")))
		}

		sendText(ctx, b, update.Message.Chat.ID, strings.Join(lines, "\n"), logger, "/memories sent")
	}
}

func extractMemoryText(message string) string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return ""
	}

	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return ""
	}

	if strings.HasPrefix(parts[0], "/memory") {
		prefix := parts[0]
		return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
	}

	return ""
}

func sendText(ctx context.Context, b *bot.Bot, chatID int64, text string, logger *slog.Logger, logKey string) {
	if _, err := b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text}); err != nil {
		logger.Error("failed to send message", "context", logKey, "error", err)
	}
}
