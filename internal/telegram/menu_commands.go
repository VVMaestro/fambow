package telegram

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func registerMenuCommands(ctx context.Context, b *bot.Bot, logger *slog.Logger) {
	commands := []models.BotCommand{
		{Command: "start", Description: "Show welcome message"},
		{Command: "help", Description: "Show available commands"},
		{Command: "love", Description: "Send a love note"},
		{Command: "memory", Description: "Save memory (text/photo/date)"},
		{Command: "memories", Description: "Show recent memories"},
		{Command: "surprise_memory", Description: "Send random memory"},
		{Command: "reminder", Description: "Guided reminder creator"},
		{Command: "remind", Description: "Create a reminder"},
		{Command: "reminders", Description: "List active reminders"},
		{Command: "event", Description: "Add a celebration date"},
		{Command: "events", Description: "List celebration dates"},
		{Command: "create_user", Description: "Admin: create bot user"},
	}

	_, err := b.SetMyCommands(ctx, &bot.SetMyCommandsParams{
		Commands: commands,
		Scope:    &models.BotCommandScopeAllPrivateChats{},
	})
	if err != nil {
		logger.Warn("failed to set bot menu commands", "error", err)
		return
	}

	logger.Info("bot menu commands registered", "count", len(commands))
}
