package telegram

import "github.com/go-telegram/bot/models"

func loveCommandKeyboard() *models.ReplyKeyboardMarkup {
	return &models.ReplyKeyboardMarkup{
		Keyboard: [][]models.KeyboardButton{
			{
				{Text: "Love Note"},
				{Text: "Memory"},
			},
		},
		ResizeKeyboard: true,
	}
}
