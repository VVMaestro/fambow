package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"fambow/internal/service"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const memoryWizardCallbackPrefix = "mw:"

type memorySaveMessages struct {
	emptyText     string
	successText   string
	usageText     string
	saveFailed    string
	saveFailedLog string
}

func memoryHandler(logger *slog.Logger, memories MemoryProvider, wizard *memoryWizardState) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if update.Message.From == nil {
			sendText(ctx, b, update.Message.Chat.ID, "I could not read your profile info for this message. Please try again.", logger, "/memory missing sender")
			return
		}

		text := extractCommandPayload(update.Message.Text, "/memory")
		if text == "" {
			startMemoryWizard(ctx, b, logger, update.Message.Chat.ID, update.Message.From.ID, memories, wizard, service.MemoryInput{})
			return
		}

		saved, _ := saveMemoryInput(ctx, b, update.Message.Chat.ID, update.Message.From, memories, service.MemoryInput{Text: text}, logger, memorySaveMessages{
			emptyText:     "Your memory looks empty. Try /memory followed by a sentence or attach a photo with /memory caption.",
			successText:   "Memory saved. I will keep this moment safe for you.",
			saveFailed:    "I could not save that memory right now. Please try again in a moment.",
			saveFailedLog: "failed to save memory",
		})
		if saved && wizard != nil {
			wizard.Delete(update.Message.From.ID)
		}
	}
}

func memoryPhotoHandler(logger *slog.Logger, memories MemoryProvider, wizard *memoryWizardState) bot.HandlerFunc {
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
		if text == "" {
			startMemoryWizard(ctx, b, logger, update.Message.Chat.ID, update.Message.From.ID, memories, wizard, service.MemoryInput{
				TelegramFileID:     telegramFileID,
				TelegramFileUnique: telegramFileUnique,
			})
			return
		}

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
		if saved && wizard != nil {
			wizard.Delete(update.Message.From.ID)
		}
	}
}

func memoryWizardStartHandler(logger *slog.Logger, memories MemoryProvider, wizard *memoryWizardState) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if update.Message.From == nil {
			sendText(ctx, b, update.Message.Chat.ID, "I could not read your profile info for this message. Please try again.", logger, "memory button missing sender")
			return
		}

		startMemoryWizard(ctx, b, logger, update.Message.Chat.ID, update.Message.From.ID, memories, wizard, service.MemoryInput{})
	}
}

func startMemoryWizard(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64, userID int64, memories MemoryProvider, wizard *memoryWizardState, input service.MemoryInput) {
	if memories == nil {
		sendText(ctx, b, chatID, "Memory feature is not configured yet.", logger, "/memory unavailable")
		return
	}
	if wizard == nil {
		sendText(ctx, b, chatID, "Memory feature is not configured yet.", logger, "/memory wizard unavailable")
		return
	}

	session := wizard.Start(userID)
	if hasMemoryWizardInput(input) {
		session.Input = input
		session.Step = memoryWizardStepSelectDate
		wizard.Set(userID, session)
		memoryWizardSendDatePrompt(ctx, b, logger, chatID)
		return
	}

	wizard.Set(userID, session)
	sendText(ctx, b, chatID, "Let's save a memory together.\n\nStep 1: send one text message or one photo with optional caption.\n(You can type cancel anytime to stop.)", logger, "memory wizard start")
}

func memoryWizardMatch(wizard *memoryWizardState) bot.MatchFunc {
	return func(update *models.Update) bool {
		if wizard == nil || update.Message == nil || update.Message.From == nil {
			return false
		}

		session, ok := wizard.Get(update.Message.From.ID)
		if !ok {
			return false
		}

		if len(update.Message.Photo) > 0 {
			return session.Step == memoryWizardStepCapture
		}

		text := strings.TrimSpace(update.Message.Text)
		if text == "" {
			return false
		}

		if strings.HasPrefix(text, "/") {
			return isMemoryWizardCancel(text)
		}

		return session.Step == memoryWizardStepCapture || session.Step == memoryWizardStepAwaitDate
	}
}

func memoryWizardCallbackHandler(logger *slog.Logger, memories MemoryProvider, wizard *memoryWizardState) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.CallbackQuery == nil {
			return
		}

		chatID := chatIDFromUpdate(update)
		user := senderFromUpdate(update)
		if user == nil {
			answerCallbackQuery(ctx, b, update, logger, "")
			return
		}

		if memories == nil {
			if chatID != 0 {
				sendText(ctx, b, chatID, "Memory feature is not configured yet.", logger, "memory wizard unavailable")
			}
			answerCallbackQuery(ctx, b, update, logger, "")
			return
		}

		payload := strings.TrimPrefix(update.CallbackQuery.Data, memoryWizardCallbackPrefix)
		action := payload
		value := ""
		if idx := strings.Index(payload, ":"); idx >= 0 {
			action = payload[:idx]
			value = payload[idx+1:]
		}

		session, ok := wizard.Get(user.ID)
		if !ok {
			if chatID != 0 {
				sendText(ctx, b, chatID, "Memory flow expired. Send /memory to start again.", logger, "memory wizard missing session")
			}
			answerCallbackQuery(ctx, b, update, logger, "")
			return
		}

		switch action {
		case "cancel":
			wizard.Delete(user.ID)
			if chatID != 0 {
				sendText(ctx, b, chatID, "Memory flow canceled.", logger, "memory wizard cancel")
			}
		case "date":
			memoryWizardHandleDateChoice(ctx, b, logger, chatID, user, value, session, memories, wizard)
		default:
			if chatID != 0 {
				sendText(ctx, b, chatID, "I did not understand that choice. Please try again.", logger, "memory wizard unknown action")
			}
		}

		answerCallbackQuery(ctx, b, update, logger, "")
	}
}

func memoryWizardMessageHandler(logger *slog.Logger, memories MemoryProvider, wizard *memoryWizardState) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil || update.Message.From == nil {
			return
		}

		if memories == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Memory feature is not configured yet.", logger, "memory wizard unavailable")
			return
		}

		session, ok := wizard.Get(update.Message.From.ID)
		if !ok {
			return
		}

		text := strings.TrimSpace(update.Message.Text)
		if isMemoryWizardCancel(text) {
			wizard.Delete(update.Message.From.ID)
			sendText(ctx, b, update.Message.Chat.ID, "Memory flow canceled.", logger, "memory wizard cancel")
			return
		}

		switch session.Step {
		case memoryWizardStepCapture:
			memoryWizardHandleCapture(ctx, b, logger, update, wizard, session)
		case memoryWizardStepAwaitDate:
			memoryWizardHandleCustomDate(ctx, b, logger, update, memories, wizard, session)
		}
	}
}

func memoryWizardHandleCapture(ctx context.Context, b *bot.Bot, logger *slog.Logger, update *models.Update, wizard *memoryWizardState, session memoryWizardSession) {
	input := service.MemoryInput{Text: strings.TrimSpace(update.Message.Text)}
	if len(update.Message.Photo) > 0 {
		telegramFileID, telegramFileUnique := pickLargestPhoto(update.Message.Photo)
		if telegramFileID == "" {
			sendText(ctx, b, update.Message.Chat.ID, "I could not read the attached photo. Please try sending it again.", logger, "memory wizard photo missing")
			return
		}

		input.Text = strings.TrimSpace(update.Message.Caption)
		input.TelegramFileID = telegramFileID
		input.TelegramFileUnique = telegramFileUnique
	}

	if !hasMemoryWizardInput(input) {
		sendText(ctx, b, update.Message.Chat.ID, "I still need a memory text or a photo. Send one message and I will save it.", logger, "memory wizard empty")
		return
	}

	session.Input = input
	session.Step = memoryWizardStepSelectDate
	wizard.Set(update.Message.From.ID, session)
	memoryWizardSendDatePrompt(ctx, b, logger, update.Message.Chat.ID)
}

func memoryWizardHandleDateChoice(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64, user *models.User, value string, session memoryWizardSession, memories MemoryProvider, wizard *memoryWizardState) {
	switch value {
	case "today":
		session.Input.CreatedAt = nil
		memoryWizardSave(ctx, b, logger, chatID, user, memories, wizard, session)
	case "custom":
		session.Step = memoryWizardStepAwaitDate
		wizard.Set(user.ID, session)
		if chatID != 0 {
			sendText(ctx, b, chatID, "Send the date as YYYY-MM-DD. Example: 2020-06-12", logger, "memory wizard custom date")
		}
	default:
		if chatID != 0 {
			sendText(ctx, b, chatID, "Please choose Today or Custom Date.", logger, "memory wizard date invalid")
		}
	}
}

func memoryWizardHandleCustomDate(ctx context.Context, b *bot.Bot, logger *slog.Logger, update *models.Update, memories MemoryProvider, wizard *memoryWizardState, session memoryWizardSession) {
	customDate, err := parseMemoryWizardDate(update.Message.Text)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMemoryDateInFuture):
			sendText(ctx, b, update.Message.Chat.ID, "That date is in the future. Send a past date or today.", logger, "memory wizard future date")
		default:
			sendText(ctx, b, update.Message.Chat.ID, "Send the date as YYYY-MM-DD. Example: 2020-06-12", logger, "memory wizard invalid date")
		}
		return
	}

	session.Input.CreatedAt = customDate
	memoryWizardSave(ctx, b, logger, update.Message.Chat.ID, update.Message.From, memories, wizard, session)
}

func memoryWizardSave(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64, user *models.User, memories MemoryProvider, wizard *memoryWizardState, session memoryWizardSession) {
	if user == nil {
		sendText(ctx, b, chatID, "I could not read your profile info for this message. Please try again.", logger, "memory wizard missing sender")
		return
	}

	_, err := memories.AddMemory(ctx, user.ID, user.FirstName, session.Input)
	if err != nil {
		if errors.Is(err, service.ErrMemoryContentEmpty) || errors.Is(err, service.ErrMemoryTextEmpty) {
			session.Step = memoryWizardStepCapture
			wizard.Set(user.ID, session)
			sendText(ctx, b, chatID, "I still need a memory text or a photo. Send one message and I will save it.", logger, "memory wizard empty")
			return
		}
		if errors.Is(err, service.ErrMemoryDateInFuture) {
			session.Step = memoryWizardStepAwaitDate
			wizard.Set(user.ID, session)
			sendText(ctx, b, chatID, "That date is in the future. Send a past date or today.", logger, "memory wizard future date")
			return
		}

		logger.Error("failed to save wizard memory", "error", err)
		sendText(ctx, b, chatID, "I could not save that memory right now. Please try again with /memory.", logger, "memory wizard save failed")
		wizard.Delete(user.ID)
		return
	}

	wizard.Delete(user.ID)
	if strings.TrimSpace(session.Input.TelegramFileID) != "" {
		sendText(ctx, b, chatID, "Memory with photo saved. I will keep this moment safe for you.", logger, "memory wizard saved")
		return
	}

	sendText(ctx, b, chatID, "Memory saved. I will keep this moment safe for you.", logger, "memory wizard saved")
}

func memoryWizardSendDatePrompt(ctx context.Context, b *bot.Bot, logger *slog.Logger, chatID int64) {
	if chatID == 0 {
		return
	}

	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        "Step 2: save this memory for today or pick a custom date?",
		ReplyMarkup: memoryWizardDateKeyboard(),
	})
	if err != nil {
		logger.Error("failed to send memory date keyboard", "error", err)
	}
}

func memoryWizardDateKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{
		{
			{Text: "Today", CallbackData: memoryWizardCallbackPrefix + "date:today"},
			{Text: "Custom Date", CallbackData: memoryWizardCallbackPrefix + "date:custom"},
		},
		{
			{Text: "Cancel", CallbackData: memoryWizardCallbackPrefix + "cancel"},
		},
	}}
}

func hasMemoryWizardInput(input service.MemoryInput) bool {
	return strings.TrimSpace(input.Text) != "" || strings.TrimSpace(input.TelegramFileID) != ""
}

func parseMemoryWizardDate(value string) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, service.ErrMemoryDateFormat
	}

	now := time.Now()
	parsedDate, err := time.ParseInLocation("2006-01-02", trimmed, now.Location())
	if err != nil {
		return nil, service.ErrMemoryDateFormat
	}

	customDate := time.Date(parsedDate.Year(), parsedDate.Month(), parsedDate.Day(), 0, 0, 0, 0, now.Location())
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if customDate.After(today) {
		return nil, service.ErrMemoryDateInFuture
	}

	return &customDate, nil
}

func isMemoryWizardCancel(text string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(text))
	return trimmed == "cancel" || trimmed == "/cancel"
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

		telegramUserID := int64(0)
		if update.Message.From != nil {
			telegramUserID = update.Message.From.ID
		}

		record, err := memories.RandomMemoryForUser(ctx, telegramUserID)
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
