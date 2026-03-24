package telegram

import (
	"errors"
	"strings"
	"testing"
	"time"

	"fambow/internal/service"

	"github.com/go-telegram/bot/models"
)

func TestStartAndHelpHandlersIncludeShortcuts(t *testing.T) {
	harness := newTestBotHarness(t, testBotDeps{adminTelegramUserID: 1})

	processUpdate(t, harness, newTextUpdate(1, 100, "/start"))
	processUpdate(t, harness, newTextUpdate(1, 100, "/help"))

	requests := harness.client.requestsFor("sendMessage")
	if len(requests) != 2 {
		t.Fatalf("expected 2 sendMessage requests, got %d", len(requests))
	}

	start := requests[0]
	if !strings.Contains(start.Fields["text"], "Reminder, My Reminders") {
		t.Fatalf("expected /start text to mention My Reminders, got %q", start.Fields["text"])
	}
	if !strings.Contains(start.Fields["text"], "Event, or Events") {
		t.Fatalf("expected /start text to mention Event and Events, got %q", start.Fields["text"])
	}
	if !strings.Contains(start.Fields["text"], "photo caption `/add_love`") {
		t.Fatalf("expected /start text to mention photo love notes, got %q", start.Fields["text"])
	}

	startKeyboard := parseReplyKeyboardMarkup(t, start.Fields["reply_markup"])
	if !replyKeyboardContains(startKeyboard, "My Reminders") {
		t.Fatal("expected /start keyboard to include My Reminders")
	}
	if !replyKeyboardContains(startKeyboard, "Event") || !replyKeyboardContains(startKeyboard, "Events") {
		t.Fatal("expected /start keyboard to include Event and Events")
	}

	help := requests[1]
	if !strings.Contains(help.Fields["text"], "Reminder, My Reminders") {
		t.Fatalf("expected /help text to mention My Reminders, got %q", help.Fields["text"])
	}
	if !strings.Contains(help.Fields["text"], "/event - guided celebration creator") {
		t.Fatalf("expected /help text to mention guided /event, got %q", help.Fields["text"])
	}
	if !strings.Contains(help.Fields["text"], "/reminders - list active reminders") {
		t.Fatalf("expected /help text to mention /reminders, got %q", help.Fields["text"])
	}
	if !strings.Contains(help.Fields["text"], "Photo shortcut: send a photo with caption /add_love <optional note>") {
		t.Fatalf("expected /help text to mention photo /add_love shortcut, got %q", help.Fields["text"])
	}
	if !strings.Contains(help.Fields["text"], "/list_love_notes - admin list saved love notes") {
		t.Fatalf("expected /help text to mention /list_love_notes, got %q", help.Fields["text"])
	}
	if !strings.Contains(help.Fields["text"], "/delete_love_notes <id> <id> ... - admin delete saved love notes") {
		t.Fatalf("expected /help text to mention /delete_love_notes, got %q", help.Fields["text"])
	}

	helpKeyboard := parseReplyKeyboardMarkup(t, help.Fields["reply_markup"])
	if !replyKeyboardContains(helpKeyboard, "My Reminders") {
		t.Fatal("expected /help keyboard to include My Reminders")
	}
	if !replyKeyboardContains(helpKeyboard, "Event") || !replyKeyboardContains(helpKeyboard, "Events") {
		t.Fatal("expected /help keyboard to include Event and Events")
	}
}

func TestLoveCommands(t *testing.T) {
	t.Run("love uses provider result", func(t *testing.T) {
		loveNotes := &loveNoteProviderSpy{randomResult: service.LoveNote{Text: "Hey Anna, you are magic."}}
		harness := newTestBotHarness(t, testBotDeps{
			loveNotes:           loveNotes,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/love"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Hey Anna, you are magic." {
			t.Fatalf("unexpected /love response: %q", request.Fields["text"])
		}
		if loveNotes.randomFirstName != "Anna" {
			t.Fatalf("expected first name Anna, got %q", loveNotes.randomFirstName)
		}

		keyboard := parseReplyKeyboardMarkup(t, request.Fields["reply_markup"])
		if !replyKeyboardContains(keyboard, "My Reminders") {
			t.Fatal("expected /love keyboard to include My Reminders")
		}
		if !replyKeyboardContains(keyboard, "Event") || !replyKeyboardContains(keyboard, "Events") {
			t.Fatal("expected /love keyboard to include Event and Events")
		}
	})

	t.Run("love returns explicit empty state when provider has no notes", func(t *testing.T) {
		loveNotes := &loveNoteProviderSpy{randomErr: service.ErrLoveNotesEmpty}
		harness := newTestBotHarness(t, testBotDeps{
			loveNotes:           loveNotes,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/love"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "No love notes yet. Add one with /add_love." {
			t.Fatalf("unexpected empty-state /love response: %q", request.Fields["text"])
		}
	})

	t.Run("love returns configuration message when provider is nil", func(t *testing.T) {
		harness := newTestBotHarness(t, testBotDeps{adminTelegramUserID: 1})

		processUpdate(t, harness, newTextUpdate(1, 100, "/love"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Love note feature is not configured yet." {
			t.Fatalf("unexpected nil-provider /love response: %q", request.Fields["text"])
		}
	})

	t.Run("add_love stores note and acknowledges success", func(t *testing.T) {
		loveNotes := &loveNoteProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			loveNotes:           loveNotes,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/add_love You are sunshine"))

		if loveNotes.addedNote.Text != "You are sunshine" {
			t.Fatalf("expected stored love note, got %#v", loveNotes.addedNote)
		}

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Love note added successfully!" {
			t.Fatalf("unexpected /add_love response: %q", request.Fields["text"])
		}
	})

	t.Run("add_love photo stores largest photo and optional text", func(t *testing.T) {
		loveNotes := &loveNoteProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			loveNotes:           loveNotes,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newPhotoUpdate(1, 100, "/add_love You are sunshine", []models.PhotoSize{
			{FileID: "small", FileUniqueID: "small-uniq", FileSize: 10},
			{FileID: "large", FileUniqueID: "large-uniq", FileSize: 20},
		}))

		if loveNotes.addedNote.Text != "You are sunshine" {
			t.Fatalf("expected stored photo note text, got %#v", loveNotes.addedNote)
		}
		if loveNotes.addedNote.TelegramFileID != "large" {
			t.Fatalf("expected largest photo id, got %#v", loveNotes.addedNote)
		}
		if loveNotes.addedNote.TelegramFileUnique != "large-uniq" {
			t.Fatalf("expected largest photo unique id, got %#v", loveNotes.addedNote)
		}

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Love note added successfully!" {
			t.Fatalf("unexpected /add_love photo response: %q", request.Fields["text"])
		}
	})

	t.Run("add_love photo returns retry when photo cannot be read", func(t *testing.T) {
		loveNotes := &loveNoteProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			loveNotes:           loveNotes,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newPhotoUpdate(1, 100, "/add_love My favorite", []models.PhotoSize{
			{FileID: "", FileUniqueID: "", FileSize: 10},
		}))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "I could not read the attached photo. Please try sending it again." {
			t.Fatalf("unexpected unreadable photo response: %q", request.Fields["text"])
		}
	})

	t.Run("love sends photo with caption when note has image", func(t *testing.T) {
		loveNotes := &loveNoteProviderSpy{randomResult: service.LoveNote{
			Text:           "Photo day",
			TelegramFileID: "photo-123",
		}}
		harness := newTestBotHarness(t, testBotDeps{
			loveNotes:           loveNotes,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/love"))

		request := harness.client.lastRequest("sendPhoto")
		if request.Fields["caption"] != "Photo day" {
			t.Fatalf("unexpected /love photo caption: %#v", request)
		}
		keyboard := parseReplyKeyboardMarkup(t, request.Fields["reply_markup"])
		if !replyKeyboardContains(keyboard, "My Reminders") {
			t.Fatal("expected /love photo keyboard to include My Reminders")
		}
	})

	t.Run("love sends photo without caption for photo-only note", func(t *testing.T) {
		loveNotes := &loveNoteProviderSpy{randomResult: service.LoveNote{
			TelegramFileID: "photo-456",
		}}
		harness := newTestBotHarness(t, testBotDeps{
			loveNotes:           loveNotes,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/love"))

		request := harness.client.lastRequest("sendPhoto")
		if request.Fields["caption"] != "" {
			t.Fatalf("expected empty photo caption, got %#v", request)
		}
	})

	t.Run("love falls back to photo then text when caption is too long", func(t *testing.T) {
		loveNotes := &loveNoteProviderSpy{randomResult: service.LoveNote{
			Text:           strings.Repeat("a", telegramPhotoCaptionLimit+1),
			TelegramFileID: "photo-long",
		}}
		harness := newTestBotHarness(t, testBotDeps{
			loveNotes:           loveNotes,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/love"))

		requests := harness.client.allRequests()
		if len(requests) != 2 {
			t.Fatalf("expected photo and text fallback, got %#v", requests)
		}
		if requests[0].Method != "sendPhoto" || requests[0].Fields["caption"] != "" {
			t.Fatalf("unexpected fallback photo request: %#v", requests[0])
		}
		if requests[1].Method != "sendMessage" || requests[1].Fields["text"] != strings.Repeat("a", telegramPhotoCaptionLimit+1) {
			t.Fatalf("unexpected fallback text request: %#v", requests[1])
		}
	})
}

func TestLoveNoteAdminCommands(t *testing.T) {
	t.Run("forbids non admin list and delete", func(t *testing.T) {
		loveNotes := &loveNoteProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			loveNotes:           loveNotes,
			adminTelegramUserID: 99,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/list_love_notes"))
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "Only admin can use love note admin commands." {
			t.Fatalf("unexpected non-admin list response: %q", got)
		}

		processUpdate(t, harness, newTextUpdate(1, 100, "/delete_love_notes 1 2"))
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "Only admin can use love note admin commands." {
			t.Fatalf("unexpected non-admin delete response: %q", got)
		}
	})

	t.Run("list returns empty state", func(t *testing.T) {
		loveNotes := &loveNoteProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			loveNotes:           loveNotes,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/list_love_notes"))

		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "No love notes yet. Add one with /add_love." {
			t.Fatalf("unexpected empty list response: %q", got)
		}
	})

	t.Run("list renders ids preview and photo marker", func(t *testing.T) {
		loveNotes := &loveNoteProviderSpy{
			listResult: []service.AdminLoveNote{
				{
					ID:        7,
					Text:      "  Sunset walk with hot tea  ",
					HasPhoto:  true,
					CreatedAt: time.Date(2026, time.March, 20, 18, 15, 0, 0, time.Local),
				},
				{
					ID:        6,
					HasPhoto:  true,
					CreatedAt: time.Date(2026, time.March, 19, 9, 5, 0, 0, time.Local),
				},
			},
		}
		harness := newTestBotHarness(t, testBotDeps{
			loveNotes:           loveNotes,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/list_love_notes"))

		got := harness.client.lastRequest("sendMessage").Fields["text"]
		if !strings.Contains(got, "Saved love notes:") {
			t.Fatalf("expected header, got %q", got)
		}
		if !strings.Contains(got, "#7 2026-03-20 18:15 [photo] Sunset walk with hot tea") {
			t.Fatalf("expected photo preview line, got %q", got)
		}
		if !strings.Contains(got, "#6 2026-03-19 09:05 [photo only]") {
			t.Fatalf("expected photo-only line, got %q", got)
		}
	})

	t.Run("list splits large output into multiple messages", func(t *testing.T) {
		items := make([]service.AdminLoveNote, 0, 60)
		for i := 0; i < 60; i++ {
			items = append(items, service.AdminLoveNote{
				ID:        int64(100 - i),
				Text:      strings.Repeat("long preview text ", 8),
				CreatedAt: time.Date(2026, time.March, 20, 18, 15, 0, 0, time.Local),
			})
		}
		loveNotes := &loveNoteProviderSpy{listResult: items}
		harness := newTestBotHarness(t, testBotDeps{
			loveNotes:           loveNotes,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/list_love_notes"))

		requests := harness.client.requestsFor("sendMessage")
		if len(requests) < 2 {
			t.Fatalf("expected list output to split into multiple messages, got %d", len(requests))
		}
	})

	t.Run("delete returns usage for malformed payload", func(t *testing.T) {
		loveNotes := &loveNoteProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			loveNotes:           loveNotes,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/delete_love_notes nope"))

		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != deleteLoveNotesUsage() {
			t.Fatalf("unexpected delete usage response: %q", got)
		}
	})

	t.Run("delete reports partial success", func(t *testing.T) {
		loveNotes := &loveNoteProviderSpy{
			deleteResult: service.DeleteLoveNotesResult{
				DeletedIDs: []int64{2, 7},
				MissingIDs: []int64{10},
			},
		}
		harness := newTestBotHarness(t, testBotDeps{
			loveNotes:           loveNotes,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/delete_love_notes 2 7 2 10"))

		if len(loveNotes.deleteIDs) != 4 || loveNotes.deleteIDs[0] != 2 || loveNotes.deleteIDs[1] != 7 || loveNotes.deleteIDs[2] != 2 || loveNotes.deleteIDs[3] != 10 {
			t.Fatalf("unexpected delete inputs: %#v", loveNotes.deleteIDs)
		}
		got := harness.client.lastRequest("sendMessage").Fields["text"]
		expected := "Deleted love notes: #2, #7\nMissing IDs: #10"
		if got != expected {
			t.Fatalf("unexpected partial delete response: %q", got)
		}
	})
}

func TestMemoryCommands(t *testing.T) {
	t.Run("memory command starts wizard when empty", func(t *testing.T) {
		memories := &memoryProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			memories:            memories,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/memory"))

		request := harness.client.lastRequest("sendMessage")
		if !strings.Contains(request.Fields["text"], "Step 1: send one text message or one photo") {
			t.Fatalf("unexpected /memory wizard response: %q", request.Fields["text"])
		}
		if memories.addInput.Text != "" || memories.addInput.TelegramFileID != "" {
			t.Fatal("expected empty /memory not to save immediately")
		}
	})

	t.Run("memory button starts same wizard", func(t *testing.T) {
		memories := &memoryProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			memories:            memories,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "Memory"))

		request := harness.client.lastRequest("sendMessage")
		if !strings.Contains(request.Fields["text"], "Step 1: send one text message or one photo") {
			t.Fatalf("unexpected Memory button response: %q", request.Fields["text"])
		}
	})

	t.Run("memory saves text payload", func(t *testing.T) {
		memories := &memoryProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			memories:            memories,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/memory Our first hike"))

		if memories.addInput.Text != "Our first hike" {
			t.Fatalf("expected saved text, got %q", memories.addInput.Text)
		}

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Memory saved. I will keep this moment safe for you." {
			t.Fatalf("unexpected /memory response: %q", request.Fields["text"])
		}
	})

	t.Run("memory returns usage for invalid date payload", func(t *testing.T) {
		memories := &memoryProviderSpy{addErr: service.ErrMemoryDateFormat}
		harness := newTestBotHarness(t, testBotDeps{
			memories:            memories,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/memory 2026-99-99 | Broken"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != service.MemoryUsage() {
			t.Fatalf("expected memory usage response, got %q", request.Fields["text"])
		}
	})

	t.Run("memory photo saves largest photo id", func(t *testing.T) {
		memories := &memoryProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			memories:            memories,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newPhotoUpdate(1, 100, "/memory 2020-06-12 | Lake day", []models.PhotoSize{
			{FileID: "small", FileUniqueID: "small-uniq", FileSize: 10},
			{FileID: "large", FileUniqueID: "large-uniq", FileSize: 20},
		}))

		if memories.addInput.TelegramFileID != "large" {
			t.Fatalf("expected largest photo id to be saved, got %q", memories.addInput.TelegramFileID)
		}
		if memories.addInput.TelegramFileUnique != "large-uniq" {
			t.Fatalf("expected largest photo unique id, got %q", memories.addInput.TelegramFileUnique)
		}

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Memory with photo saved. I will keep this moment safe for you." {
			t.Fatalf("unexpected /memory photo response: %q", request.Fields["text"])
		}
	})

	t.Run("empty memory photo starts wizard with preloaded photo", func(t *testing.T) {
		memories := &memoryProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			memories:            memories,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newPhotoUpdate(1, 100, "/memory", []models.PhotoSize{
			{FileID: "small", FileUniqueID: "small-uniq", FileSize: 10},
			{FileID: "large", FileUniqueID: "large-uniq", FileSize: 20},
		}))

		request := harness.client.lastRequest("sendMessage")
		if !strings.Contains(request.Fields["text"], "Step 2: save this memory for today or pick a custom date?") {
			t.Fatalf("unexpected preloaded photo wizard response: %q", request.Fields["text"])
		}
		markup := parseInlineKeyboardMarkup(t, request.Fields["reply_markup"])
		if !inlineKeyboardContains(markup, "Today") || !inlineKeyboardContains(markup, "Custom Date") || !inlineKeyboardContains(markup, "Cancel") {
			t.Fatalf("unexpected memory wizard keyboard: %#v", markup)
		}
		if memories.addInput.TelegramFileID != "" {
			t.Fatal("expected empty photo-caption /memory not to save immediately")
		}
	})

	t.Run("memory wizard saves text for today", func(t *testing.T) {
		memories := &memoryProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			memories:            memories,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/memory"))
		processUpdate(t, harness, newTextUpdate(1, 100, "Road trip to the lake"))
		processUpdate(t, harness, newCallbackUpdate(1, 100, memoryWizardCallbackPrefix+"date:today"))

		if memories.addInput.Text != "Road trip to the lake" {
			t.Fatalf("expected wizard text to be saved, got %q", memories.addInput.Text)
		}
		if memories.addInput.CreatedAt != nil {
			t.Fatal("expected Today path to keep createdAt nil for service default")
		}

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Memory saved. I will keep this moment safe for you." {
			t.Fatalf("unexpected memory wizard save response: %q", request.Fields["text"])
		}
	})

	t.Run("memory wizard keeps session on invalid and future custom date", func(t *testing.T) {
		memories := &memoryProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			memories:            memories,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/memory"))
		processUpdate(t, harness, newTextUpdate(1, 100, "Concert night"))
		processUpdate(t, harness, newCallbackUpdate(1, 100, memoryWizardCallbackPrefix+"date:custom"))
		processUpdate(t, harness, newTextUpdate(1, 100, "broken"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Send the date as YYYY-MM-DD. Example: 2020-06-12" {
			t.Fatalf("unexpected invalid custom date response: %q", request.Fields["text"])
		}

		futureDate := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
		processUpdate(t, harness, newTextUpdate(1, 100, futureDate))

		request = harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "That date is in the future. Send a past date or today." {
			t.Fatalf("unexpected future custom date response: %q", request.Fields["text"])
		}

		processUpdate(t, harness, newTextUpdate(1, 100, "2020-06-12"))

		if memories.addInput.CreatedAt == nil {
			t.Fatal("expected valid custom date to be passed to memory save")
		}
		if got := memories.addInput.CreatedAt.Format("2006-01-02"); got != "2020-06-12" {
			t.Fatalf("unexpected wizard custom date: %q", got)
		}

		request = harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Memory saved. I will keep this moment safe for you." {
			t.Fatalf("unexpected custom date save response: %q", request.Fields["text"])
		}
	})

	t.Run("memory wizard can save preloaded photo for today", func(t *testing.T) {
		memories := &memoryProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			memories:            memories,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newPhotoUpdate(1, 100, "/memory", []models.PhotoSize{
			{FileID: "small", FileUniqueID: "small-uniq", FileSize: 10},
			{FileID: "large", FileUniqueID: "large-uniq", FileSize: 20},
		}))
		processUpdate(t, harness, newCallbackUpdate(1, 100, memoryWizardCallbackPrefix+"date:today"))

		if memories.addInput.TelegramFileID != "large" {
			t.Fatalf("expected preloaded photo id to be saved, got %q", memories.addInput.TelegramFileID)
		}

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Memory with photo saved. I will keep this moment safe for you." {
			t.Fatalf("unexpected wizard photo save response: %q", request.Fields["text"])
		}
	})

	t.Run("memory wizard ignores unrelated commands and allows cancel", func(t *testing.T) {
		memories := &memoryProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			memories:            memories,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/memory"))
		processUpdate(t, harness, newTextUpdate(1, 100, "/help"))

		request := harness.client.lastRequest("sendMessage")
		if !strings.Contains(request.Fields["text"], "Available commands:") {
			t.Fatalf("expected /help to handle command during wizard, got %q", request.Fields["text"])
		}

		processUpdate(t, harness, newTextUpdate(1, 100, "cancel"))

		request = harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Memory flow canceled." {
			t.Fatalf("unexpected memory cancel response: %q", request.Fields["text"])
		}
		if memories.addInput.Text != "" || memories.addInput.TelegramFileID != "" {
			t.Fatal("expected canceled wizard not to save memory")
		}
	})

	t.Run("memories sends empty state when nothing exists", func(t *testing.T) {
		memories := &memoryProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			memories:            memories,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/memories"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "No saved memories yet. Add one with /memory <text> or photo caption /memory." {
			t.Fatalf("unexpected /memories empty response: %q", request.Fields["text"])
		}
		if memories.recentLimit != 3 {
			t.Fatalf("expected /memories limit 3, got %d", memories.recentLimit)
		}
	})

	t.Run("memories sends header plus text and photo entries", func(t *testing.T) {
		memories := &memoryProviderSpy{
			recentResult: []service.Memory{
				{Text: "Road trip", CreatedAt: time.Date(2026, time.March, 1, 0, 0, 0, 0, time.Local)},
				{CreatedAt: time.Date(2026, time.March, 2, 0, 0, 0, 0, time.Local), TelegramFileID: "photo-1"},
			},
		}
		harness := newTestBotHarness(t, testBotDeps{
			memories:            memories,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/memories"))

		requests := harness.client.allRequests()
		if len(requests) != 3 {
			t.Fatalf("expected 3 requests for /memories, got %d", len(requests))
		}
		if requests[0].Method != "sendMessage" || requests[0].Fields["text"] != "Your recent memories:" {
			t.Fatalf("unexpected memories header request: %#v", requests[0])
		}
		if requests[1].Method != "sendMessage" || requests[1].Fields["text"] != "Road trip (2026-03-01)" {
			t.Fatalf("unexpected memories text entry: %#v", requests[1])
		}
		if requests[2].Method != "sendPhoto" || requests[2].Fields["caption"] != "Photo memory (2026-03-02)" {
			t.Fatalf("unexpected memories photo entry: %#v", requests[2])
		}
	})

	t.Run("surprise memory sends empty state when nothing exists", func(t *testing.T) {
		memories := &memoryProviderSpy{randomErr: service.ErrMemoryNotFound}
		harness := newTestBotHarness(t, testBotDeps{
			memories:            memories,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/surprise_memory"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "No saved memories yet. Add one with /memory <text> or photo caption /memory." {
			t.Fatalf("unexpected /surprise_memory empty response: %q", request.Fields["text"])
		}
	})

	t.Run("surprise memory sends header and photo", func(t *testing.T) {
		memories := &memoryProviderSpy{
			randomResult: service.Memory{
				CreatedAt:      time.Date(2026, time.March, 3, 0, 0, 0, 0, time.Local),
				TelegramFileID: "photo-surprise",
			},
		}
		harness := newTestBotHarness(t, testBotDeps{
			memories:            memories,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/surprise_memory"))

		requests := harness.client.allRequests()
		if len(requests) != 2 {
			t.Fatalf("expected 2 requests for surprise memory, got %d", len(requests))
		}
		if requests[0].Method != "sendMessage" || requests[0].Fields["text"] != "Your surprise memory:" {
			t.Fatalf("unexpected surprise memory header: %#v", requests[0])
		}
		if requests[1].Method != "sendPhoto" || requests[1].Fields["caption"] != "Photo memory (2026-03-03)" {
			t.Fatalf("unexpected surprise memory photo response: %#v", requests[1])
		}
	})
}

func TestReminderCommands(t *testing.T) {
	t.Run("reminder command starts wizard with target keyboard", func(t *testing.T) {
		users := &userProviderSpy{getResult: service.User{Type: "wife"}}
		harness := newTestBotHarness(t, testBotDeps{
			reminders:           &reminderProviderSpy{},
			users:               users,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/reminder"))

		request := harness.client.lastRequest("sendMessage")
		if !strings.Contains(request.Fields["text"], "Step 1: who is this reminder for?") {
			t.Fatalf("unexpected /reminder response: %q", request.Fields["text"])
		}

		markup := parseInlineKeyboardMarkup(t, request.Fields["reply_markup"])
		if !inlineKeyboardContains(markup, "For Me") || !inlineKeyboardContains(markup, "For Him") || !inlineKeyboardContains(markup, "Cancel") {
			t.Fatalf("unexpected reminder wizard keyboard: %#v", markup)
		}
	})

	t.Run("remind saves self reminder", func(t *testing.T) {
		reminders := &reminderProviderSpy{
			addResult: service.Reminder{
				Text:            "vitamins",
				ScheduleDisplay: "Daily at 08:00",
			},
		}
		harness := newTestBotHarness(t, testBotDeps{
			reminders:           reminders,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/remind daily 08:00 vitamins"))

		if reminders.addCommand != "daily 08:00 vitamins" {
			t.Fatalf("expected reminder payload without command prefix, got %q", reminders.addCommand)
		}

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Reminder saved: Daily at 08:00\n- vitamins" {
			t.Fatalf("unexpected /remind success response: %q", request.Fields["text"])
		}
	})

	t.Run("remind saves targeted reminder", func(t *testing.T) {
		reminders := &reminderProviderSpy{
			targetResult: service.Reminder{
				Text:            "to drink tea",
				ScheduleDisplay: "Once at 2026-03-21 19:30",
			},
		}
		harness := newTestBotHarness(t, testBotDeps{
			reminders:           reminders,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/remind her at 19:30 to drink tea"))

		if reminders.targetUserType != "wife" {
			t.Fatalf("expected targeted user type wife, got %q", reminders.targetUserType)
		}
		if reminders.targetCommand != "at 19:30 to drink tea" {
			t.Fatalf("expected targeted reminder payload, got %q", reminders.targetCommand)
		}

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Reminder saved for her: Once at 2026-03-21 19:30\n- to drink tea" {
			t.Fatalf("unexpected targeted /remind response: %q", request.Fields["text"])
		}
	})

	t.Run("remind returns usage for invalid payload", func(t *testing.T) {
		reminders := &reminderProviderSpy{addErr: service.ErrReminderInvalidFormat}
		harness := newTestBotHarness(t, testBotDeps{
			reminders:           reminders,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/remind nope"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != service.ReminderUsage() {
			t.Fatalf("expected reminder usage response, got %q", request.Fields["text"])
		}
	})

	t.Run("remind returns partner missing error", func(t *testing.T) {
		reminders := &reminderProviderSpy{targetErr: service.ErrReminderTargetNotFound}
		harness := newTestBotHarness(t, testBotDeps{
			reminders:           reminders,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/remind her at 19:30 to drink tea"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "I could not find that partner in the database yet. Ask both users to use the bot once, then try again." {
			t.Fatalf("unexpected reminder target missing response: %q", request.Fields["text"])
		}
	})

	t.Run("remind returns generic save failure", func(t *testing.T) {
		reminders := &reminderProviderSpy{addErr: errors.New("boom")}
		harness := newTestBotHarness(t, testBotDeps{
			reminders:           reminders,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/remind daily 08:00 vitamins"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "I could not save your reminder right now. Please try again in a moment." {
			t.Fatalf("unexpected reminder save failure response: %q", request.Fields["text"])
		}
	})

	t.Run("reminders list sends empty state", func(t *testing.T) {
		reminders := &reminderProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			reminders:           reminders,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/reminders"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "No active reminders yet. Add one with /remind." {
			t.Fatalf("unexpected /reminders empty response: %q", request.Fields["text"])
		}
	})

	t.Run("reminders list sends formatted reminder items", func(t *testing.T) {
		reminders := &reminderProviderSpy{
			listResult: []service.Reminder{
				{ScheduleDisplay: "Daily at 08:00", Text: "vitamins"},
				{ScheduleDisplay: "Once at 2026-03-21 19:30", Text: "call mom"},
			},
		}
		harness := newTestBotHarness(t, testBotDeps{
			reminders:           reminders,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/reminders"))

		request := harness.client.lastRequest("sendMessage")
		expected := "Your active reminders:\n- Daily at 08:00: vitamins\n- Once at 2026-03-21 19:30: call mom"
		if request.Fields["text"] != expected {
			t.Fatalf("unexpected /reminders success response: %q", request.Fields["text"])
		}
	})

	t.Run("reminders list returns generic load failure", func(t *testing.T) {
		reminders := &reminderProviderSpy{listErr: errors.New("boom")}
		harness := newTestBotHarness(t, testBotDeps{
			reminders:           reminders,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/reminders"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "I could not load reminders right now. Please try again in a moment." {
			t.Fatalf("unexpected /reminders load failure response: %q", request.Fields["text"])
		}
	})

	t.Run("my reminders button routes to reminders handler", func(t *testing.T) {
		reminders := &reminderProviderSpy{
			listResult: []service.Reminder{
				{ScheduleDisplay: "Daily at 08:00", Text: "vitamins"},
			},
		}
		harness := newTestBotHarness(t, testBotDeps{
			reminders:           reminders,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "My Reminders"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Your active reminders:\n- Daily at 08:00: vitamins" {
			t.Fatalf("unexpected My Reminders response: %q", request.Fields["text"])
		}
	})
}

func TestEventCommands(t *testing.T) {
	t.Run("event command starts wizard when empty", func(t *testing.T) {
		celebrations := &celebrationProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			celebrations:        celebrations,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/event"))

		request := harness.client.lastRequest("sendMessage")
		if !strings.Contains(request.Fields["text"], "Step 1: when is the event?") {
			t.Fatalf("unexpected /event wizard response: %q", request.Fields["text"])
		}

		markup := parseInlineKeyboardMarkup(t, request.Fields["reply_markup"])
		if !inlineKeyboardContains(markup, "Today") || !inlineKeyboardContains(markup, "Tomorrow") || !inlineKeyboardContains(markup, "Custom Date") {
			t.Fatalf("unexpected event wizard date keyboard: %#v", markup)
		}
		if celebrations.addCommand != "" {
			t.Fatal("expected empty /event not to save immediately")
		}
	})

	t.Run("event button starts same wizard", func(t *testing.T) {
		celebrations := &celebrationProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			celebrations:        celebrations,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "Event"))

		request := harness.client.lastRequest("sendMessage")
		if !strings.Contains(request.Fields["text"], "Step 1: when is the event?") {
			t.Fatalf("unexpected Event button response: %q", request.Fields["text"])
		}
	})

	t.Run("events button lists celebration dates", func(t *testing.T) {
		celebrations := &celebrationProviderSpy{
			listResult: []service.CelebrationEvent{
				{
					Title:            "Anniversary dinner",
					EventDate:        time.Date(2026, time.September, 12, 0, 0, 0, 0, time.Local),
					RemindDaysBefore: 3,
				},
			},
		}
		harness := newTestBotHarness(t, testBotDeps{
			celebrations:        celebrations,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "Events"))

		request := harness.client.lastRequest("sendMessage")
		expected := "Your celebration dates:\n- Anniversary dinner on 2026-09-12 (remind 3 day(s) before)"
		if request.Fields["text"] != expected {
			t.Fatalf("unexpected Events button response: %q", request.Fields["text"])
		}
	})

	t.Run("event saves celebration", func(t *testing.T) {
		celebrations := &celebrationProviderSpy{
			addResult: service.CelebrationEvent{
				Title:            "Anniversary dinner",
				EventDate:        time.Date(2026, time.September, 12, 0, 0, 0, 0, time.Local),
				RemindDaysBefore: 3,
			},
		}
		harness := newTestBotHarness(t, testBotDeps{
			celebrations:        celebrations,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/event add 2026-09-12 | Anniversary dinner | 3"))

		if celebrations.addCommand != "add 2026-09-12 | Anniversary dinner | 3" {
			t.Fatalf("expected event payload without command prefix, got %q", celebrations.addCommand)
		}

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Event saved: Anniversary dinner on 2026-09-12 (remind 3 day(s) before)." {
			t.Fatalf("unexpected /event response: %q", request.Fields["text"])
		}
	})

	t.Run("event wizard saves using today shortcut", func(t *testing.T) {
		today := time.Now().Local().Format("2006-01-02")
		celebrations := &celebrationProviderSpy{
			addResult: service.CelebrationEvent{
				Title:            "Anniversary dinner",
				EventDate:        time.Now().Local(),
				RemindDaysBefore: 3,
			},
		}
		harness := newTestBotHarness(t, testBotDeps{
			celebrations:        celebrations,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/event"))
		processUpdate(t, harness, newCallbackUpdate(1, 100, eventWizardCallbackPrefix+"date:today"))
		processUpdate(t, harness, newTextUpdate(1, 100, "Anniversary dinner"))
		processUpdate(t, harness, newCallbackUpdate(1, 100, eventWizardCallbackPrefix+"days:3"))

		expectedCommand := "add " + today + " | Anniversary dinner | 3"
		if celebrations.addCommand != expectedCommand {
			t.Fatalf("expected wizard command %q, got %q", expectedCommand, celebrations.addCommand)
		}

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Event saved: Anniversary dinner on "+time.Now().Local().Format("2006-01-02")+" (remind 3 day(s) before)." {
			t.Fatalf("unexpected wizard save response: %q", request.Fields["text"])
		}
	})

	t.Run("event wizard saves using tomorrow shortcut", func(t *testing.T) {
		tomorrow := time.Now().Local().Add(24 * time.Hour)
		celebrations := &celebrationProviderSpy{
			addResult: service.CelebrationEvent{
				Title:            "Flowers day",
				EventDate:        tomorrow,
				RemindDaysBefore: 1,
			},
		}
		harness := newTestBotHarness(t, testBotDeps{
			celebrations:        celebrations,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/event"))
		processUpdate(t, harness, newCallbackUpdate(1, 100, eventWizardCallbackPrefix+"date:tomorrow"))
		processUpdate(t, harness, newTextUpdate(1, 100, "Flowers day"))
		processUpdate(t, harness, newCallbackUpdate(1, 100, eventWizardCallbackPrefix+"days:1"))

		expectedCommand := "add " + tomorrow.Format("2006-01-02") + " | Flowers day | 1"
		if celebrations.addCommand != expectedCommand {
			t.Fatalf("expected tomorrow command %q, got %q", expectedCommand, celebrations.addCommand)
		}
	})

	t.Run("event wizard retries invalid and past custom date", func(t *testing.T) {
		futureDate := time.Now().Local().Add(72 * time.Hour)
		celebrations := &celebrationProviderSpy{
			addResult: service.CelebrationEvent{
				Title:            "Trip anniversary",
				EventDate:        futureDate,
				RemindDaysBefore: 7,
			},
		}
		harness := newTestBotHarness(t, testBotDeps{
			celebrations:        celebrations,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/event"))
		processUpdate(t, harness, newCallbackUpdate(1, 100, eventWizardCallbackPrefix+"date:custom"))
		processUpdate(t, harness, newTextUpdate(1, 100, "2026-99-99"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Send the date as YYYY-MM-DD. Example: 2026-09-12" {
			t.Fatalf("unexpected invalid custom date response: %q", request.Fields["text"])
		}

		pastDate := time.Now().Local().Add(-24 * time.Hour).Format("2006-01-02")
		processUpdate(t, harness, newTextUpdate(1, 100, pastDate))

		request = harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "That date is in the past. Send today or a future date." {
			t.Fatalf("unexpected past custom date response: %q", request.Fields["text"])
		}

		processUpdate(t, harness, newTextUpdate(1, 100, futureDate.Format("2006-01-02")))
		processUpdate(t, harness, newTextUpdate(1, 100, "Trip anniversary"))
		processUpdate(t, harness, newCallbackUpdate(1, 100, eventWizardCallbackPrefix+"days:7"))

		expectedCommand := "add " + futureDate.Format("2006-01-02") + " | Trip anniversary | 7"
		if celebrations.addCommand != expectedCommand {
			t.Fatalf("expected custom date command %q, got %q", expectedCommand, celebrations.addCommand)
		}
	})

	t.Run("event wizard rejects empty title and allows retry", func(t *testing.T) {
		celebrations := &celebrationProviderSpy{
			addResult: service.CelebrationEvent{
				Title:            "Date night",
				EventDate:        time.Now().Local(),
				RemindDaysBefore: 1,
			},
		}
		harness := newTestBotHarness(t, testBotDeps{
			celebrations:        celebrations,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/event"))
		processUpdate(t, harness, newCallbackUpdate(1, 100, eventWizardCallbackPrefix+"date:today"))
		processUpdate(t, harness, newTextUpdate(1, 100, "   "))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Event title cannot be empty. Send a short title." {
			t.Fatalf("unexpected empty title response: %q", request.Fields["text"])
		}

		processUpdate(t, harness, newTextUpdate(1, 100, "Date night"))

		request = harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Step 3: how many days before should I remind you?" {
			t.Fatalf("unexpected title retry response: %q", request.Fields["text"])
		}

		markup := parseInlineKeyboardMarkup(t, request.Fields["reply_markup"])
		if !inlineKeyboardContains(markup, "Type Number") {
			t.Fatalf("unexpected event days keyboard: %#v", markup)
		}
	})

	t.Run("event wizard accepts manual reminder days after retry", func(t *testing.T) {
		celebrations := &celebrationProviderSpy{
			addResult: service.CelebrationEvent{
				Title:            "Concert",
				EventDate:        time.Now().Local(),
				RemindDaysBefore: 5,
			},
		}
		harness := newTestBotHarness(t, testBotDeps{
			celebrations:        celebrations,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/event"))
		processUpdate(t, harness, newCallbackUpdate(1, 100, eventWizardCallbackPrefix+"date:today"))
		processUpdate(t, harness, newTextUpdate(1, 100, "Concert"))
		processUpdate(t, harness, newCallbackUpdate(1, 100, eventWizardCallbackPrefix+"days:manual"))
		processUpdate(t, harness, newTextUpdate(1, 100, "0"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Send a positive number of days. Example: 3" {
			t.Fatalf("unexpected invalid manual days response: %q", request.Fields["text"])
		}

		processUpdate(t, harness, newTextUpdate(1, 100, "5"))

		if !strings.HasSuffix(celebrations.addCommand, "| 5") {
			t.Fatalf("expected manual days to be saved, got %q", celebrations.addCommand)
		}

		request = harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Event saved: Concert on "+time.Now().Local().Format("2006-01-02")+" (remind 5 day(s) before)." {
			t.Fatalf("unexpected manual days save response: %q", request.Fields["text"])
		}
	})

	t.Run("event returns usage for invalid payload", func(t *testing.T) {
		celebrations := &celebrationProviderSpy{addErr: service.ErrEventInvalidFormat}
		harness := newTestBotHarness(t, testBotDeps{
			celebrations:        celebrations,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/event nope"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != service.EventUsage() {
			t.Fatalf("expected event usage response, got %q", request.Fields["text"])
		}
	})

	t.Run("event wizard callback cancel clears flow", func(t *testing.T) {
		celebrations := &celebrationProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			celebrations:        celebrations,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/event"))
		processUpdate(t, harness, newCallbackUpdate(1, 100, eventWizardCallbackPrefix+"cancel"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Event flow canceled." {
			t.Fatalf("unexpected callback cancel response: %q", request.Fields["text"])
		}
	})

	t.Run("event wizard text cancel clears flow", func(t *testing.T) {
		celebrations := &celebrationProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			celebrations:        celebrations,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/event"))
		processUpdate(t, harness, newCallbackUpdate(1, 100, eventWizardCallbackPrefix+"date:custom"))
		processUpdate(t, harness, newTextUpdate(1, 100, "/cancel"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Event flow canceled." {
			t.Fatalf("unexpected text cancel response: %q", request.Fields["text"])
		}
	})

	t.Run("event wizard ignores unrelated commands", func(t *testing.T) {
		celebrations := &celebrationProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			celebrations:        celebrations,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/event"))
		processUpdate(t, harness, newTextUpdate(1, 100, "/help"))

		request := harness.client.lastRequest("sendMessage")
		if !strings.Contains(request.Fields["text"], "Available commands:") {
			t.Fatalf("expected /help to handle command during event wizard, got %q", request.Fields["text"])
		}
		if celebrations.addCommand != "" {
			t.Fatal("expected unrelated command not to save event")
		}
	})

	t.Run("event wizard expired callback asks user to restart", func(t *testing.T) {
		celebrations := &celebrationProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			celebrations:        celebrations,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newCallbackUpdate(1, 100, eventWizardCallbackPrefix+"date:today"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Event flow expired. Send /event to start again." {
			t.Fatalf("unexpected expired callback response: %q", request.Fields["text"])
		}
	})

	t.Run("event returns generic save failure", func(t *testing.T) {
		celebrations := &celebrationProviderSpy{addErr: errors.New("boom")}
		harness := newTestBotHarness(t, testBotDeps{
			celebrations:        celebrations,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/event add 2026-09-12 | Anniversary dinner | 3"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "I could not save your event right now. Please try again in a moment." {
			t.Fatalf("unexpected /event generic failure response: %q", request.Fields["text"])
		}
	})

	t.Run("event wizard returns generic save failure", func(t *testing.T) {
		celebrations := &celebrationProviderSpy{addErr: errors.New("boom")}
		harness := newTestBotHarness(t, testBotDeps{
			celebrations:        celebrations,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/event"))
		processUpdate(t, harness, newCallbackUpdate(1, 100, eventWizardCallbackPrefix+"date:today"))
		processUpdate(t, harness, newTextUpdate(1, 100, "Anniversary dinner"))
		processUpdate(t, harness, newCallbackUpdate(1, 100, eventWizardCallbackPrefix+"days:3"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "I could not save your event right now. Please try again in a moment." {
			t.Fatalf("unexpected wizard generic failure response: %q", request.Fields["text"])
		}
	})

	t.Run("events sends empty state and success list", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			celebrations := &celebrationProviderSpy{}
			harness := newTestBotHarness(t, testBotDeps{
				celebrations:        celebrations,
				adminTelegramUserID: 1,
			})

			processUpdate(t, harness, newTextUpdate(1, 100, "/events"))

			request := harness.client.lastRequest("sendMessage")
			if request.Fields["text"] != "No celebration dates yet. Add one with /event add." {
				t.Fatalf("unexpected /events empty response: %q", request.Fields["text"])
			}
		})

		t.Run("success", func(t *testing.T) {
			celebrations := &celebrationProviderSpy{
				listResult: []service.CelebrationEvent{
					{
						Title:            "Anniversary dinner",
						EventDate:        time.Date(2026, time.September, 12, 0, 0, 0, 0, time.Local),
						RemindDaysBefore: 3,
					},
				},
			}
			harness := newTestBotHarness(t, testBotDeps{
				celebrations:        celebrations,
				adminTelegramUserID: 1,
			})

			processUpdate(t, harness, newTextUpdate(1, 100, "/events"))

			request := harness.client.lastRequest("sendMessage")
			expected := "Your celebration dates:\n- Anniversary dinner on 2026-09-12 (remind 3 day(s) before)"
			if request.Fields["text"] != expected {
				t.Fatalf("unexpected /events success response: %q", request.Fields["text"])
			}
		})

		t.Run("generic load failure", func(t *testing.T) {
			celebrations := &celebrationProviderSpy{listErr: errors.New("boom")}
			harness := newTestBotHarness(t, testBotDeps{
				celebrations:        celebrations,
				adminTelegramUserID: 1,
			})

			processUpdate(t, harness, newTextUpdate(1, 100, "/events"))

			request := harness.client.lastRequest("sendMessage")
			if request.Fields["text"] != "I could not load events right now. Please try again in a moment." {
				t.Fatalf("unexpected /events generic failure response: %q", request.Fields["text"])
			}
		})
	})
}

func TestCreateUserCommand(t *testing.T) {
	t.Run("forbids non admin user", func(t *testing.T) {
		users := &userProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			users:               users,
			adminTelegramUserID: 99,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/create_user 5 Mia wife"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Only admin can use /create_user." {
			t.Fatalf("unexpected non-admin response: %q", request.Fields["text"])
		}
	})

	t.Run("returns usage for invalid payload", func(t *testing.T) {
		users := &userProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			users:               users,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/create_user nope"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != createUserUsage() {
			t.Fatalf("expected create_user usage, got %q", request.Fields["text"])
		}
	})

	t.Run("returns duplicate user message", func(t *testing.T) {
		users := &userProviderSpy{createErr: service.ErrUserAlreadyExists}
		harness := newTestBotHarness(t, testBotDeps{
			users:               users,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/create_user 5 Mia wife"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "This Telegram user already exists in the database." {
			t.Fatalf("unexpected duplicate user response: %q", request.Fields["text"])
		}
	})

	t.Run("creates user successfully", func(t *testing.T) {
		users := &userProviderSpy{
			createResult: service.User{
				TelegramUserID: 5,
				FirstName:      "Mia",
				Type:           "wife",
			},
		}
		harness := newTestBotHarness(t, testBotDeps{
			users:               users,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/create_user 5 Mia wife"))

		if users.createTelegramUserID != 5 || users.createFirstName != "Mia" || users.createType != "wife" {
			t.Fatalf("unexpected create user inputs: id=%d name=%q type=%q", users.createTelegramUserID, users.createFirstName, users.createType)
		}

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "User created: Mia (wife)" {
			t.Fatalf("unexpected create user success response: %q", request.Fields["text"])
		}
	})
}
