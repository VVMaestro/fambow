package telegram

import "github.com/go-telegram/bot/models"

func commandKeyboard() *models.ReplyKeyboardMarkup {
	return &models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: "Love Note"},
				{Text: "Memory"},
			},
			{
				{Text: "Memories"},
				{Text: "Surprise Memory"},
			},
			{
				{Text: "Reminder"},
				{Text: "My Reminders"},
			},
		},
		ResizeKeyboard: true,
	}
}
