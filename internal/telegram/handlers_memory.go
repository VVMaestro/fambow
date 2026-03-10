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

type memorySaveMessages struct {
	emptyText     string
	successText   string
	usageText     string
	saveFailed    string
	saveFailedLog string
}

func memoryHandler(logger *slog.Logger, memories MemoryProvider, memoryIntake *memoryIntakeState) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if update.Message.From == nil {
			sendText(ctx, b, update.Message.Chat.ID, "I could not read your profile info for this message. Please try again.", logger, "/memory missing sender")
			return
		}

		text := extractCommandPayload(update.Message.Text, "/memory")
		saved, _ := saveMemoryInput(ctx, b, update.Message.Chat.ID, update.Message.From, memories, service.MemoryInput{Text: text}, logger, memorySaveMessages{
			emptyText:     "Your memory looks empty. Try /memory followed by a sentence or attach a photo with /memory caption.",
			successText:   "Memory saved. I will keep this moment safe for you.",
			saveFailed:    "I could not save that memory right now. Please try again in a moment.",
			saveFailedLog: "failed to save memory",
		})
		if saved {
			memoryIntake.Disarm(update.Message.From.ID)
		}
	}
}

func memoryPhotoHandler(logger *slog.Logger, memories MemoryProvider, memoryIntake *memoryIntakeState) bot.HandlerFunc {
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

		telegramFileID, telegramFileUnique := pickLargestPhoto(update.Message.Photo)
		if telegramFileID == "" {
			sendText(ctx, b, update.Message.Chat.ID, "I could not read the attached photo. Please try sending it again.", logger, "/memory photo missing")
			return
		}

		text := extractCommandPayload(update.Message.Caption, "/memory")
		saved, _ := saveMemoryInput(ctx, b, update.Message.Chat.ID, update.Message.From, memories, service.MemoryInput{
			Text:               text,
			TelegramFileID:     telegramFileID,
			TelegramFileUnique: telegramFileUnique,
		}, logger, memorySaveMessages{
			emptyText:     "Your memory looks empty. Add text in caption or attach a photo with /memory.",
			successText:   "Memory with photo saved. I will keep this moment safe for you.",
			saveFailed:    "I could not save that memory right now. Please try again in a moment.",
			saveFailedLog: "failed to save memory photo",
		})
		if saved {
			memoryIntake.Disarm(update.Message.From.ID)
		}
	}
}

func memoryIntakeStartHandler(logger *slog.Logger, memories MemoryProvider, memoryIntake *memoryIntakeState) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if update.Message.From == nil {
			sendText(ctx, b, update.Message.Chat.ID, "I could not read your profile info for this message. Please try again.", logger, "memory button missing sender")
			return
		}

		if memories == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Memory feature is not configured yet.", logger, "/memory unavailable")
			return
		}

		memoryIntake.Arm(update.Message.From.ID)
		sendText(ctx, b, update.Message.Chat.ID, "Memory mode is on. Send one text message or one photo with optional caption, and I will save it.\n\nCustom date syntax: YYYY-MM-DD | your memory text", logger, "memory button armed")
	}
}

func memoryIntakeMatch(memoryIntake *memoryIntakeState) bot.MatchFunc {
	return func(update *models.Update) bool {
		if memoryIntake == nil || update.Message == nil || update.Message.From == nil {
			return false
		}

		if !memoryIntake.IsArmed(update.Message.From.ID) {
			return false
		}

		if len(update.Message.Photo) > 0 {
			return true
		}

		text := strings.TrimSpace(update.Message.Text)
		if text == "" {
			return false
		}

		return !strings.HasPrefix(text, "/")
	}
}

func memoryIntakeMessageHandler(logger *slog.Logger, memories MemoryProvider, memoryIntake *memoryIntakeState) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil || update.Message.From == nil {
			return
		}

		input := service.MemoryInput{Text: strings.TrimSpace(update.Message.Text)}
		messages := memorySaveMessages{
			emptyText:     "I still need a memory text or a photo with caption. Send one more message and I will save it.",
			successText:   "Memory saved. I will keep this moment safe for you.",
			usageText:     "Use custom date syntax like this:\n2020-12-24 | Cozy holiday walk\nPhoto: add the same syntax in caption.",
			saveFailed:    "I could not save that memory right now. Please try again in a moment.",
			saveFailedLog: "failed to save memory from intake",
		}

		if len(update.Message.Photo) > 0 {
			telegramFileID, telegramFileUnique := pickLargestPhoto(update.Message.Photo)
			if telegramFileID == "" {
				sendText(ctx, b, update.Message.Chat.ID, "I could not read the attached photo. Please try sending it again.", logger, "memory intake photo missing")
				return
			}

			input.Text = strings.TrimSpace(update.Message.Caption)
			input.TelegramFileID = telegramFileID
			input.TelegramFileUnique = telegramFileUnique
			messages.successText = "Memory with photo saved. I will keep this moment safe for you."
			messages.saveFailedLog = "failed to save memory photo from intake"
		}

		saved, _ := saveMemoryInput(ctx, b, update.Message.Chat.ID, update.Message.From, memories, input, logger, messages)
		if saved {
			memoryIntake.Disarm(update.Message.From.ID)
		}
	}
}

func saveMemoryInput(ctx context.Context, b *bot.Bot, chatID int64, user *models.User, memories MemoryProvider, input service.MemoryInput, logger *slog.Logger, messages memorySaveMessages) (bool, bool) {
	if user == nil {
		sendText(ctx, b, chatID, "I could not read your profile info for this message. Please try again.", logger, "memory missing sender")
		return false, false
	}

	if memories == nil {
		sendText(ctx, b, chatID, "Memory feature is not configured yet.", logger, "memory unavailable")
		return false, false
	}

	_, err := memories.AddMemory(ctx, user.ID, user.FirstName, input)
	if err != nil {
		if errors.Is(err, service.ErrMemoryContentEmpty) || errors.Is(err, service.ErrMemoryTextEmpty) {
			sendText(ctx, b, chatID, messages.emptyText, logger, "memory empty")
			return false, true
		}

		if errors.Is(err, service.ErrMemoryDateFormat) || errors.Is(err, service.ErrMemoryDateInFuture) {
			usageText := strings.TrimSpace(messages.usageText)
			if usageText == "" {
				usageText = service.MemoryUsage()
			}
			sendText(ctx, b, chatID, usageText, logger, "memory usage")
			return false, true
		}

		logger.Error(messages.saveFailedLog, "error", err)
		sendText(ctx, b, chatID, messages.saveFailed, logger, "memory save failed")
		return false, false
	}

	sendText(ctx, b, chatID, messages.successText, logger, "memory saved")
	return true, false
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

func surpriseMemoryHandler(logger *slog.Logger, memories MemoryProvider) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if memories == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Memory feature is not configured yet.", logger, "/surprise_memory unavailable")
			return
		}

		record, err := memories.RandomMemory(ctx)
		if err != nil {
			if errors.Is(err, service.ErrMemoryNotFound) {
				sendText(ctx, b, update.Message.Chat.ID, "No saved memories yet. Add one with /memory <text> or photo caption /memory.", logger, "/surprise_memory empty")
				return
			}

			logger.Error("failed to load surprise memory", "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not pick a surprise memory right now. Please try again in a moment.", logger, "/surprise_memory load failed")
			return
		}

		sendText(ctx, b, update.Message.Chat.ID, "Your surprise memory:", logger, "/surprise_memory sent")

		entry := formatMemoryLine(record)
		if record.TelegramFileID != "" {
			_, sendErr := b.SendPhoto(ctx, &bot.SendPhotoParams{
				ChatID:  update.Message.Chat.ID,
				Photo:   &models.InputFileString{Data: record.TelegramFileID},
				Caption: entry,
			})
			if sendErr != nil {
				logger.Warn("failed to send surprise memory photo", "error", sendErr)
				sendText(ctx, b, update.Message.Chat.ID, "[photo unavailable]", logger, "/surprise_memory sent")
			}
			return
		}

		sendText(ctx, b, update.Message.Chat.ID, entry, logger, "/surprise_memory sent")
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
