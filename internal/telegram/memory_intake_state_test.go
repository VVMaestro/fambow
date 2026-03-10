package telegram

import (
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestMemoryIntakeStateArmDisarm(t *testing.T) {
	state := newMemoryIntakeState()
	if state.IsArmed(10) {
		t.Fatal("state should start disarmed")
	}

	state.Arm(10)
	if !state.IsArmed(10) {
		t.Fatal("expected state to be armed")
	}

	state.Disarm(10)
	if state.IsArmed(10) {
		t.Fatal("expected state to be disarmed")
	}
}

func TestMemoryIntakeMatch(t *testing.T) {
	state := newMemoryIntakeState()
	state.Arm(55)
	match := memoryIntakeMatch(state)

	if !match(&models.Update{Message: &models.Message{From: &models.User{ID: 55}, Text: "Small happy memory"}}) {
		t.Fatal("expected plain text to match when armed")
	}

	if match(&models.Update{Message: &models.Message{From: &models.User{ID: 55}, Text: "/help"}}) {
		t.Fatal("expected command text not to match")
	}

	if !match(&models.Update{Message: &models.Message{From: &models.User{ID: 55}, Photo: []models.PhotoSize{{FileID: "abc"}}}}) {
		t.Fatal("expected photo message to match when armed")
	}

	if match(&models.Update{Message: &models.Message{From: &models.User{ID: 99}, Text: "Other user text"}}) {
		t.Fatal("expected unarmed user not to match")
	}
}
