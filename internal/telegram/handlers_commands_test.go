package telegram

import (
	"errors"
	"strings"
	"testing"
	"time"

	"fambow/internal/service"

	"github.com/go-telegram/bot/models"
)

func TestStartAndHelpHandlersIncludeMyRemindersShortcut(t *testing.T) {
	harness := newTestBotHarness(t, testBotDeps{adminTelegramUserID: 1})

	processUpdate(t, harness, newTextUpdate(1, 100, "/start"))
	processUpdate(t, harness, newTextUpdate(1, 100, "/help"))

	requests := harness.client.requestsFor("sendMessage")
	if len(requests) != 2 {
		t.Fatalf("expected 2 sendMessage requests, got %d", len(requests))
	}

	start := requests[0]
	if !strings.Contains(start.Fields["text"], "Reminder, or My Reminders") {
		t.Fatalf("expected /start text to mention My Reminders, got %q", start.Fields["text"])
	}

	startKeyboard := parseReplyKeyboardMarkup(t, start.Fields["reply_markup"])
	if !replyKeyboardContains(startKeyboard, "My Reminders") {
		t.Fatal("expected /start keyboard to include My Reminders")
	}

	help := requests[1]
	if !strings.Contains(help.Fields["text"], "Reminder, or My Reminders") {
		t.Fatalf("expected /help text to mention My Reminders, got %q", help.Fields["text"])
	}
	if !strings.Contains(help.Fields["text"], "/reminders - list active reminders") {
		t.Fatalf("expected /help text to mention /reminders, got %q", help.Fields["text"])
	}

	helpKeyboard := parseReplyKeyboardMarkup(t, help.Fields["reply_markup"])
	if !replyKeyboardContains(helpKeyboard, "My Reminders") {
		t.Fatal("expected /help keyboard to include My Reminders")
	}
}

func TestLoveCommands(t *testing.T) {
	t.Run("love uses provider result", func(t *testing.T) {
		loveNotes := &loveNoteProviderSpy{randomResult: "Hey Anna, you are magic."}
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
	})

	t.Run("love falls back when provider is nil", func(t *testing.T) {
		harness := newTestBotHarness(t, testBotDeps{adminTelegramUserID: 1})

		processUpdate(t, harness, newTextUpdate(1, 100, "/love"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "You are loved so much, my love." {
			t.Fatalf("unexpected fallback /love response: %q", request.Fields["text"])
		}
	})

	t.Run("add_love stores note and acknowledges success", func(t *testing.T) {
		loveNotes := &loveNoteProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			loveNotes:           loveNotes,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/add_love You are sunshine"))

		if loveNotes.addedNote != "You are sunshine" {
			t.Fatalf("expected stored love note, got %q", loveNotes.addedNote)
		}

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Love note added successfully!" {
			t.Fatalf("unexpected /add_love response: %q", request.Fields["text"])
		}
	})
}

func TestMemoryCommands(t *testing.T) {
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
