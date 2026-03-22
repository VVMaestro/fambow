package telegram

import (
	"errors"
	"strings"
	"testing"

	"fambow/internal/service"
)

func TestLoveSchedulerCommands(t *testing.T) {
	t.Run("forbids non admin setup and list and remove", func(t *testing.T) {
		schedules := &loveScheduleProviderSpy{}
		users := &userProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			loveSchedules:       schedules,
			users:               users,
			adminTelegramUserID: 99,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/love_scheduler"))
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "Only admin can use love note scheduler commands." {
			t.Fatalf("unexpected setup forbidden message: %q", got)
		}

		processUpdate(t, harness, newTextUpdate(1, 100, "/love_schedulers"))
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "Only admin can use love note scheduler commands." {
			t.Fatalf("unexpected list forbidden message: %q", got)
		}

		processUpdate(t, harness, newTextUpdate(1, 100, "/love_scheduler_remove 3"))
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "Only admin can use love note scheduler commands." {
			t.Fatalf("unexpected remove forbidden message: %q", got)
		}
	})

	t.Run("wizard shows empty state when no registered users exist", func(t *testing.T) {
		schedules := &loveScheduleProviderSpy{}
		users := &userProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			loveSchedules:       schedules,
			users:               users,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/love_scheduler"))

		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "No registered users yet. Create one first with /create_user." {
			t.Fatalf("unexpected no-users message: %q", got)
		}
	})

	t.Run("wizard creates schedule with preset time", func(t *testing.T) {
		schedules := &loveScheduleProviderSpy{
			addResult: service.LoveNoteSchedule{
				ID:              3,
				TelegramUserID:  5,
				FirstName:       "Mia",
				UserType:        "wife",
				ScheduleTime:    "08:30",
				ScheduleDisplay: "at 08:30",
			},
		}
		users := &userProviderSpy{
			listResult: []service.User{{TelegramUserID: 5, FirstName: "Mia", Type: "wife"}},
			getResult:  service.User{TelegramUserID: 5, FirstName: "Mia", Type: "wife"},
		}
		harness := newTestBotHarness(t, testBotDeps{
			loveSchedules:       schedules,
			users:               users,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/love_scheduler"))
		request := harness.client.lastRequest("sendMessage")
		if !strings.Contains(request.Fields["text"], "Step 1: choose who should receive it.") {
			t.Fatalf("unexpected wizard start text: %q", request.Fields["text"])
		}
		markup := parseInlineKeyboardMarkup(t, request.Fields["reply_markup"])
		if !inlineKeyboardContains(markup, "Mia (wife)") || !inlineKeyboardContains(markup, "Cancel") {
			t.Fatalf("unexpected user keyboard: %#v", markup)
		}

		processUpdate(t, harness, newCallbackUpdate(1, 100, loveScheduleWizardCallbackPrefix+"user:5"))
		request = harness.client.lastRequest("sendMessage")
		if !strings.Contains(request.Fields["text"], "Mia (wife) will receive a daily love note.") {
			t.Fatalf("unexpected user selection text: %q", request.Fields["text"])
		}

		processUpdate(t, harness, newCallbackUpdate(1, 100, loveScheduleWizardCallbackPrefix+"time:08:30"))
		if schedules.addTelegramUserID != 5 || schedules.addScheduleTime != "08:30" {
			t.Fatalf("unexpected add schedule args: user=%d time=%q", schedules.addTelegramUserID, schedules.addScheduleTime)
		}

		request = harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Love note scheduler saved: #3 Mia (wife) at 08:30." {
			t.Fatalf("unexpected save confirmation: %q", request.Fields["text"])
		}
	})

	t.Run("wizard accepts manual time and cancel", func(t *testing.T) {
		schedules := &loveScheduleProviderSpy{
			addResult: service.LoveNoteSchedule{
				ID:              4,
				TelegramUserID:  6,
				FirstName:       "Ivan",
				UserType:        "husband",
				ScheduleDisplay: "at 21:00",
			},
		}
		users := &userProviderSpy{
			listResult: []service.User{{TelegramUserID: 6, FirstName: "Ivan", Type: "husband"}},
			getResult:  service.User{TelegramUserID: 6, FirstName: "Ivan", Type: "husband"},
		}
		harness := newTestBotHarness(t, testBotDeps{
			loveSchedules:       schedules,
			users:               users,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/love_scheduler"))
		processUpdate(t, harness, newCallbackUpdate(1, 100, loveScheduleWizardCallbackPrefix+"user:6"))
		processUpdate(t, harness, newCallbackUpdate(1, 100, loveScheduleWizardCallbackPrefix+"time:manual"))

		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "Send the time in HH:MM (24h). Example: 08:30" {
			t.Fatalf("unexpected manual prompt: %q", got)
		}

		processUpdate(t, harness, newTextUpdate(1, 100, "21:00"))
		if schedules.addScheduleTime != "21:00" {
			t.Fatalf("expected manual time to be saved, got %q", schedules.addScheduleTime)
		}

		processUpdate(t, harness, newTextUpdate(1, 100, "/love_scheduler"))
		processUpdate(t, harness, newTextUpdate(1, 100, "cancel"))
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "Love note scheduler flow canceled." {
			t.Fatalf("unexpected cancel text: %q", got)
		}
	})

	t.Run("wizard expired callback asks to restart", func(t *testing.T) {
		harness := newTestBotHarness(t, testBotDeps{
			loveSchedules:       &loveScheduleProviderSpy{},
			users:               &userProviderSpy{},
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newCallbackUpdate(1, 100, loveScheduleWizardCallbackPrefix+"user:5"))

		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "Love note scheduler flow expired. Send /love_scheduler to start again." {
			t.Fatalf("unexpected expired message: %q", got)
		}
	})

	t.Run("list includes ids and remove handles usage missing and success", func(t *testing.T) {
		schedules := &loveScheduleProviderSpy{
			listResult: []service.LoveNoteSchedule{{
				ID:              3,
				FirstName:       "Mia",
				UserType:        "wife",
				ScheduleDisplay: "at 08:30",
			}},
		}
		harness := newTestBotHarness(t, testBotDeps{
			loveSchedules:       schedules,
			users:               &userProviderSpy{},
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/love_schedulers"))
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "Active love note schedulers:\n- #3 Mia (wife) at 08:30" {
			t.Fatalf("unexpected list output: %q", got)
		}

		processUpdate(t, harness, newTextUpdate(1, 100, "/love_scheduler_remove nope"))
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != loveScheduleRemoveUsage() {
			t.Fatalf("unexpected remove usage output: %q", got)
		}

		processUpdate(t, harness, newTextUpdate(1, 100, "/love_scheduler_remove 3"))
		if schedules.removeScheduleID != 3 {
			t.Fatalf("expected remove schedule id 3, got %d", schedules.removeScheduleID)
		}
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "Love note scheduler #3 removed." {
			t.Fatalf("unexpected remove success output: %q", got)
		}
	})

	t.Run("remove returns not found", func(t *testing.T) {
		schedules := &loveScheduleProviderSpy{removeErr: service.ErrLoveNoteScheduleNotFound}
		harness := newTestBotHarness(t, testBotDeps{
			loveSchedules:       schedules,
			users:               &userProviderSpy{},
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/love_scheduler_remove 9"))
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "That love note scheduler does not exist or is already inactive." {
			t.Fatalf("unexpected remove missing output: %q", got)
		}
	})

	t.Run("list returns load failure", func(t *testing.T) {
		schedules := &loveScheduleProviderSpy{listErr: errors.New("boom")}
		harness := newTestBotHarness(t, testBotDeps{
			loveSchedules:       schedules,
			users:               &userProviderSpy{},
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/love_schedulers"))
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "I could not load love note schedulers right now. Please try again in a moment." {
			t.Fatalf("unexpected list failure output: %q", got)
		}
	})
}
