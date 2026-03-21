package telegram

import (
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestEventWizardStateStartDelete(t *testing.T) {
	state := newEventWizardState()
	session := state.Start(10)
	if session.Step != eventWizardStepSelectDate {
		t.Fatalf("expected select date step, got %q", session.Step)
	}

	stored, ok := state.Get(10)
	if !ok {
		t.Fatal("expected stored wizard session")
	}
	if stored.Step != eventWizardStepSelectDate {
		t.Fatalf("expected stored select date step, got %q", stored.Step)
	}

	state.Delete(10)
	if _, ok := state.Get(10); ok {
		t.Fatal("expected state to be deleted")
	}
}

func TestEventWizardMatch(t *testing.T) {
	state := newEventWizardState()
	state.Start(55)
	match := eventWizardMatch(state)

	if match(&models.Update{Message: &models.Message{From: &models.User{ID: 55}, Text: "Dinner anniversary"}}) {
		t.Fatal("expected select date step not to match free text")
	}

	state.Set(55, eventWizardSession{Step: eventWizardStepAwaitDate})
	if !match(&models.Update{Message: &models.Message{From: &models.User{ID: 55}, Text: "2026-09-12"}}) {
		t.Fatal("expected custom date text to match when awaiting date")
	}

	state.Set(55, eventWizardSession{Step: eventWizardStepAwaitTitle})
	if !match(&models.Update{Message: &models.Message{From: &models.User{ID: 55}, Text: "Anniversary dinner"}}) {
		t.Fatal("expected title text to match when awaiting title")
	}
	if match(&models.Update{Message: &models.Message{From: &models.User{ID: 55}, Text: "/help"}}) {
		t.Fatal("expected unrelated command not to match")
	}

	state.Set(55, eventWizardSession{Step: eventWizardStepAwaitDays})
	if !match(&models.Update{Message: &models.Message{From: &models.User{ID: 55}, Text: "3"}}) {
		t.Fatal("expected days text to match when awaiting days")
	}
	if !match(&models.Update{Message: &models.Message{From: &models.User{ID: 55}, Text: "cancel"}}) {
		t.Fatal("expected cancel to match")
	}

	if match(&models.Update{Message: &models.Message{From: &models.User{ID: 99}, Text: "3"}}) {
		t.Fatal("expected unarmed user not to match")
	}
}
