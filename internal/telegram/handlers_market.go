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

const marketCallbackPrefix = "mk:"

func productAddHandler(logger *slog.Logger, products ProductProvider, adminTelegramUserID int64) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if !isAdminMessage(update.Message.From, adminTelegramUserID) {
			sendText(ctx, b, update.Message.Chat.ID, "Only admin can use market admin commands.", logger, "/stuff_add forbidden")
			return
		}

		if products == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Market feature is not configured yet.", logger, "/stuff_add unavailable")
			return
		}

		product, err := products.AddProduct(ctx, extractCommandPayload(update.Message.Text, "/stuff_add"))
		if err != nil {
			if errors.Is(err, service.ErrProductNameEmpty) || errors.Is(err, service.ErrProductCostInvalid) {
				sendText(ctx, b, update.Message.Chat.ID, service.ProductAddUsage(), logger, "/stuff_add usage")
				return
			}

			logger.Error("failed to add product", "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not add that stuff right now. Please try again in a moment.", logger, "/stuff_add failed")
			return
		}

		sendText(ctx, b, update.Message.Chat.ID, fmt.Sprintf("Stuff added: #%d %s - %s", product.ID, product.Name, service.FormatPanCoins(product.Cost)), logger, "/stuff_add added")
	}
}

func productRemoveHandler(logger *slog.Logger, products ProductProvider, adminTelegramUserID int64) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if !isAdminMessage(update.Message.From, adminTelegramUserID) {
			sendText(ctx, b, update.Message.Chat.ID, "Only admin can use market admin commands.", logger, "/stuff_remove forbidden")
			return
		}

		if products == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Market feature is not configured yet.", logger, "/stuff_remove unavailable")
			return
		}

		productID, err := products.RemoveProduct(ctx, extractCommandPayload(update.Message.Text, "/stuff_remove"))
		if err != nil {
			if errors.Is(err, service.ErrProductIDInvalid) {
				sendText(ctx, b, update.Message.Chat.ID, service.ProductRemoveUsage(), logger, "/stuff_remove usage")
				return
			}
			if errors.Is(err, service.ErrProductNotFound) {
				sendText(ctx, b, update.Message.Chat.ID, "That stuff does not exist.", logger, "/stuff_remove missing")
				return
			}

			logger.Error("failed to remove product", "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not remove that stuff right now. Please try again in a moment.", logger, "/stuff_remove failed")
			return
		}

		sendText(ctx, b, update.Message.Chat.ID, fmt.Sprintf("Stuff #%d removed.", productID), logger, "/stuff_remove removed")
	}
}

func moneySetHandler(logger *slog.Logger, users UserProvider, adminTelegramUserID int64) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil {
			return
		}

		if !isAdminMessage(update.Message.From, adminTelegramUserID) {
			sendText(ctx, b, update.Message.Chat.ID, "Only admin can use market admin commands.", logger, "/money_set forbidden")
			return
		}

		if users == nil {
			sendText(ctx, b, update.Message.Chat.ID, "User management is not configured yet.", logger, "/money_set users nil")
			return
		}

		telegramUserID, money, err := service.ParseMoneySetPayload(extractCommandPayload(update.Message.Text, "/money_set"))
		if err != nil {
			sendText(ctx, b, update.Message.Chat.ID, service.MoneySetUsage(), logger, "/money_set usage")
			return
		}

		user, err := users.SetMoney(ctx, telegramUserID, money)
		if err != nil {
			if errors.Is(err, service.ErrUserNotFound) {
				sendText(ctx, b, update.Message.Chat.ID, "That user does not exist in the database.", logger, "/money_set missing")
				return
			}

			logger.Error("failed to set user money", "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not update that balance right now. Please try again in a moment.", logger, "/money_set failed")
			return
		}

		sendText(ctx, b, update.Message.Chat.ID, fmt.Sprintf("%s now has %s.", user.FirstName, service.FormatPanCoins(user.Money)), logger, "/money_set updated")
	}
}

func productsHandler(logger *slog.Logger, products ProductProvider, users UserProvider) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.Message == nil || update.Message.From == nil {
			return
		}

		if products == nil || users == nil {
			sendText(ctx, b, update.Message.Chat.ID, "Market feature is not configured yet.", logger, "/stuffs unavailable")
			return
		}

		user, err := users.GetUser(ctx, update.Message.From.ID)
		if err != nil {
			logger.Error("failed to load user for products", "telegram_user_id", update.Message.From.ID, "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not load your pan-coin balance right now. Please try again in a moment.", logger, "/stuffs user failed")
			return
		}

		items, err := products.ListProducts(ctx)
		if err != nil {
			logger.Error("failed listing products", "error", err)
			sendText(ctx, b, update.Message.Chat.ID, "I could not load stuff right now. Please try again in a moment.", logger, "/stuffs load failed")
			return
		}

		if len(items) == 0 {
			sendText(ctx, b, update.Message.Chat.ID, fmt.Sprintf("No stuff available yet.\nYour balance: %s", service.FormatPanCoins(user.Money)), logger, "/stuffs empty")
			return
		}

		lines := make([]string, 0, len(items)+2)
		lines = append(lines, "Family market:")
		lines = append(lines, fmt.Sprintf("Your balance: %s pan-coins 💰", service.FormatPanCoins(user.Money)))
		for _, item := range items {
			lines = append(lines, "- "+service.FormatProduct(item))
		}

		_, err = b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID:      update.Message.Chat.ID,
			Text:        strings.Join(lines, "\n"),
			ReplyMarkup: productKeyboard(items),
		})
		if err != nil {
			logger.Error("failed to send stuff list", "error", err)
		}
	}
}

func productBuyCallbackHandler(logger *slog.Logger, products ProductProvider) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if update.CallbackQuery == nil {
			return
		}

		chatID := chatIDFromUpdate(update)
		user := senderFromUpdate(update)
		if chatID == 0 || user == nil {
			answerCallbackQuery(ctx, b, update, logger, "")
			return
		}

		if products == nil {
			sendText(ctx, b, chatID, "Market feature is not configured yet.", logger, "product buy unavailable")
			answerCallbackQuery(ctx, b, update, logger, "")
			return
		}

		productID, err := parseMarketCallback(update.CallbackQuery.Data)
		if err != nil {
			sendText(ctx, b, chatID, "I did not understand that stuff choice.", logger, "product buy invalid callback")
			answerCallbackQuery(ctx, b, update, logger, "")
			return
		}

		result, err := products.BuyProduct(ctx, user.ID, productID)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrProductNotFound):
				sendText(ctx, b, chatID, "That stuff is no longer available.", logger, "product buy missing")
			case errors.Is(err, service.ErrProductInsufficientFunds):
				sendText(ctx, b, chatID, "You do not have enough pan-coins for this stuff.", logger, "product buy insufficient")
			case errors.Is(err, service.ErrProductBuyerMissingPartner):
				sendText(ctx, b, chatID, "Your opposite partner type is not registered yet, so I could not complete the purchase.", logger, "product buy partner missing")
			case errors.Is(err, service.ErrProductBuyerNotFound) || errors.Is(err, service.ErrUserTelegramIDInvalid):
				sendText(ctx, b, chatID, "I could not find your user in the database.", logger, "product buy buyer missing")
			default:
				logger.Error("failed to buy product", "product_id", productID, "telegram_user_id", user.ID, "error", err)
				sendText(ctx, b, chatID, "I could not complete that purchase right now. Please try again in a moment.", logger, "product buy failed")
			}
			answerCallbackQuery(ctx, b, update, logger, "")
			return
		}

		sendText(ctx, b, chatID, fmt.Sprintf("You bought %s for %s. Remaining balance: %s.", result.Product.Name, service.FormatPanCoins(result.Product.Cost), service.FormatPanCoins(result.BuyerMoneyAfter)), logger, "product buy success")
		sendText(ctx, b, result.OppositeTelegramUserID, fmt.Sprintf("%s bought %s for %s.", result.BuyerFirstName, result.Product.Name, service.FormatPanCoins(result.Product.Cost)), logger, "product partner notify")
		answerCallbackQuery(ctx, b, update, logger, "Purchased")
	}
}

func productKeyboard(products []service.Product) *models.InlineKeyboardMarkup {
	buttons := make([][]models.InlineKeyboardButton, 0, len(products))
	for _, product := range products {
		buttons = append(buttons, []models.InlineKeyboardButton{{
			Text:         fmt.Sprintf("%s - %s", product.Name, service.FormatPanCoins(product.Cost)),
			CallbackData: marketCallbackPrefix + "buy:" + strconv.FormatInt(product.ID, 10),
		}})
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: buttons}
}

func parseMarketCallback(data string) (int64, error) {
	payload := strings.TrimPrefix(data, marketCallbackPrefix)
	if !strings.HasPrefix(payload, "buy:") {
		return 0, fmt.Errorf("invalid callback action")
	}

	productID, err := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(payload, "buy:")), 10, 64)
	if err != nil || productID <= 0 {
		return 0, fmt.Errorf("invalid product id")
	}

	return productID, nil
}
