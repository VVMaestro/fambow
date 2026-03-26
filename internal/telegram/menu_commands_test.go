package telegram

import (
	"strings"
	"testing"
)

func TestRegisterMenuCommandsIncludesLoveNoteAdminCommands(t *testing.T) {
	harness := newTestBotHarness(t, testBotDeps{
		adminTelegramUserID:  1,
		registerMenuCommands: true,
	})

	request := harness.client.lastRequest("setMyCommands")
	raw := request.Fields["_raw"]
	if raw == "" {
		raw = request.Fields["commands"]
	}
	if !strings.Contains(raw, `"command":"list_love_notes"`) {
		t.Fatalf("expected menu commands to include list_love_notes, got %q", raw)
	}
	if !strings.Contains(raw, `"command":"delete_love_notes"`) {
		t.Fatalf("expected menu commands to include delete_love_notes, got %q", raw)
	}
	if !strings.Contains(raw, `"command":"list_reminders"`) {
		t.Fatalf("expected menu commands to include list_reminders, got %q", raw)
	}
	if !strings.Contains(raw, `"command":"remove_reminder"`) {
		t.Fatalf("expected menu commands to include remove_reminder, got %q", raw)
	}
}
