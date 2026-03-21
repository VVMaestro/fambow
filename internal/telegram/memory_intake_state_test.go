package telegram

import (
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestMemoryWizardStateStartDelete(t *testing.T) {
	state := newMemoryWizardState()
	session := state.Start(10)
	if session.Step != memoryWizardStepCapture {
		t.Fatalf("expected capture step, got %q", session.Step)
	}

	stored, ok := state.Get(10)
	if !ok {
		t.Fatal("expected stored wizard session")
	}
	if stored.Step != memoryWizardStepCapture {
		t.Fatalf("expected stored capture step, got %q", stored.Step)
	}

	state.Delete(10)
	if _, ok := state.Get(10); ok {
		t.Fatal("expected state to be deleted")
	}
}

func TestMemoryWizardMatch(t *testing.T) {
	state := newMemoryWizardState()
	state.Start(55)
	match := memoryWizardMatch(state)

	if !match(&models.Update{Message: &models.Message{From: &models.User{ID: 55}, Text: "Small happy memory"}}) {
		t.Fatal("expected plain text to match when armed")
	}

	if match(&models.Update{Message: &models.Message{From: &models.User{ID: 55}, Text: "/help"}}) {
		t.Fatal("expected command text not to match")
	}

	if !match(&models.Update{Message: &models.Message{From: &models.User{ID: 55}, Photo: []models.PhotoSize{{FileID: "abc"}}}}) {
		t.Fatal("expected photo message to match when armed")
	}

	state.Set(55, memoryWizardSession{Step: memoryWizardStepAwaitDate})
	if !match(&models.Update{Message: &models.Message{From: &models.User{ID: 55}, Text: "2020-06-12"}}) {
		t.Fatal("expected custom date text to match when awaiting date")
	}
	if match(&models.Update{Message: &models.Message{From: &models.User{ID: 55}, Photo: []models.PhotoSize{{FileID: "abc"}}}}) {
		t.Fatal("expected photo not to match when awaiting custom date")
	}

	if match(&models.Update{Message: &models.Message{From: &models.User{ID: 99}, Text: "Other user text"}}) {
		t.Fatal("expected unarmed user not to match")
	}
}
