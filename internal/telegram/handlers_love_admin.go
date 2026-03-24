package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"fambow/internal/service"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	loveNoteAdminMessageLimit = 3500
	loveNotePreviewLimit      = 80
)

func listLoveNotesHandler(logger *slog.Logger, loveNotes LoveNoteProvider, adminTelegramUserID int64) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if !isAdminMessage(update.Message.From, adminTelegramUserID) {
			sendText(ctx, b, update.Message.Chat.ID, "Only admin can use love note admin commands.", logger, "/list_love_notes forbidden")
			return
		}

		if loveNotes == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Love note feature is not configured yet.", logger, "/list_love_notes unavailable")
			return
		}

		notes, err := loveNotes.ListLoveNotes(ctx)
		if err != nil {
			logger.Error("failed listing love notes", "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not load love notes right now. Please try again in a moment.", logger, "/list_love_notes failed")
			return
		}

		if len(notes) == 0 {
			sendText(ctx, b, update.Message.Chat.ID, emptyLoveNotesMessage, logger, "/list_love_notes empty")
			return
		}

		for _, chunk := range buildLoveNoteListMessages(notes, loveNoteAdminMessageLimit) {
			sendText(ctx, b, update.Message.Chat.ID, chunk, logger, "/list_love_notes sent")
		}
	}
}

func deleteLoveNotesHandler(logger *slog.Logger, loveNotes LoveNoteProvider, adminTelegramUserID int64) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if !isAdminMessage(update.Message.From, adminTelegramUserID) {
			sendText(ctx, b, update.Message.Chat.ID, "Only admin can use love note admin commands.", logger, "/delete_love_notes forbidden")
			return
		}

		if loveNotes == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Love note feature is not configured yet.", logger, "/delete_love_notes unavailable")
			return
		}

		noteIDs, err := parseDeleteLoveNotesPayload(extractCommandPayload(update.Message.Text, "/delete_love_notes"))
		if err != nil {
			sendText(ctx, b, update.Message.Chat.ID, deleteLoveNotesUsage(), logger, "/delete_love_notes usage")
			return
		}

		result, err := loveNotes.DeleteLoveNotes(ctx, noteIDs)
		if err != nil {
			if errors.Is(err, service.ErrLoveNoteIDsInvalid) {
				sendText(ctx, b, update.Message.Chat.ID, deleteLoveNotesUsage(), logger, "/delete_love_notes usage")
				return
			}

			logger.Error("failed deleting love notes", "note_ids", formatLoveNoteIDList(noteIDs), "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not delete those love notes right now. Please try again in a moment.", logger, "/delete_love_notes failed")
			return
		}

		sendText(ctx, b, update.Message.Chat.ID, formatDeleteLoveNotesResult(result), logger, "/delete_love_notes removed")
	}
}

func parseDeleteLoveNotesPayload(payload string) ([]int64, error) {
	parts := strings.Fields(strings.TrimSpace(payload))
	if len(parts) == 0 {
		return nil, service.ErrLoveNoteIDsInvalid
	}

	noteIDs := make([]int64, 0, len(parts))
	for _, part := range parts {
		noteID, err := strconv.ParseInt(part, 10, 64)
		if err != nil || noteID <= 0 {
			return nil, service.ErrLoveNoteIDsInvalid
		}
		noteIDs = append(noteIDs, noteID)
	}

	return noteIDs, nil
}

func deleteLoveNotesUsage() string {
	return "Use this format:\n/delete_love_notes <id> <id> ..."
}

func buildLoveNoteListMessages(notes []service.AdminLoveNote, limit int) []string {
	if len(notes) == 0 {
		return nil
	}

	chunks := make([]string, 0, 1)
	header := "Saved love notes:"
	current := header
	continuationHeader := "Saved love notes (cont.):"

	for _, note := range notes {
		line := formatAdminLoveNoteLine(note)
		candidate := current + "\n" + line
		if len(candidate) <= limit {
			current = candidate
			continue
		}

		chunks = append(chunks, current)
		current = continuationHeader + "\n" + line
	}

	chunks = append(chunks, current)
	return chunks
}

func formatAdminLoveNoteLine(note service.AdminLoveNote) string {
	timestamp := note.CreatedAt.Local().Format("2006-01-02 15:04")
	preview := buildLoveNotePreview(note)
	if note.HasPhoto && preview != "[photo only]" {
		return fmt.Sprintf("- #%d %s [photo] %s", note.ID, timestamp, preview)
	}

	return fmt.Sprintf("- #%d %s %s", note.ID, timestamp, preview)
}

func buildLoveNotePreview(note service.AdminLoveNote) string {
	text := strings.Join(strings.Fields(strings.TrimSpace(note.Text)), " ")
	switch {
	case text == "" && note.HasPhoto:
		return "[photo only]"
	case text == "":
		return "[empty]"
	default:
		return truncateLoveNotePreview(text, loveNotePreviewLimit)
	}
}

func truncateLoveNotePreview(text string, limit int) string {
	if len([]rune(text)) <= limit {
		return text
	}

	runes := []rune(text)
	if limit <= 3 {
		return string(runes[:limit])
	}

	return string(runes[:limit-3]) + "..."
}

func formatDeleteLoveNotesResult(result service.DeleteLoveNotesResult) string {
	lines := make([]string, 0, 3)
	if len(result.DeletedIDs) == 0 {
		lines = append(lines, "No love notes deleted.")
	} else {
		lines = append(lines, "Deleted love notes: "+formatLoveNoteIDList(result.DeletedIDs))
	}

	if len(result.MissingIDs) > 0 {
		lines = append(lines, "Missing IDs: "+formatLoveNoteIDList(result.MissingIDs))
	}

	return strings.Join(lines, "\n")
}

func formatLoveNoteIDList(noteIDs []int64) string {
	parts := make([]string, 0, len(noteIDs))
	for _, id := range noteIDs {
		parts = append(parts, "#"+strconv.FormatInt(id, 10))
	}

	return strings.Join(parts, ", ")
}
