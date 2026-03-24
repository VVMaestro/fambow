package telegram

import (
	"errors"
	"strings"
	"testing"

	"fambow/internal/service"
)

func TestHelpMentionsMarketCommands(t *testing.T) {
	harness := newTestBotHarness(t, testBotDeps{adminTelegramUserID: 1})

	processUpdate(t, harness, newTextUpdate(1, 100, "/help"))

	request := harness.client.lastRequest("sendMessage")
	if !strings.Contains(request.Fields["text"], "/stuffs - show available market stuff") {
		t.Fatalf("expected /help to mention /stuffs, got %q", request.Fields["text"])
	}
	if !strings.Contains(request.Fields["text"], "/money_set <telegram_id> <amount in pan-coins> - admin only") {
		t.Fatalf("expected /help to mention /money_set, got %q", request.Fields["text"])
	}
	if !strings.Contains(request.Fields["text"], "/stuff_add <name> | <cost in pan-coins> - admin only") {
		t.Fatalf("expected /help to mention /stuff_add, got %q", request.Fields["text"])
	}
}

func TestMarketAdminCommands(t *testing.T) {
	t.Run("product add forbids non admin", func(t *testing.T) {
		products := &productProviderSpy{}
		harness := newTestBotHarness(t, testBotDeps{
			products:            products,
			adminTelegramUserID: 99,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/stuff_add Flowers | 20"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "Only admin can use market admin commands." {
			t.Fatalf("unexpected non-admin product_add response: %q", request.Fields["text"])
		}
	})

	t.Run("product add returns usage and success", func(t *testing.T) {
		products := &productProviderSpy{
			addErr: service.ErrProductCostInvalid,
		}
		harness := newTestBotHarness(t, testBotDeps{
			products:            products,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/stuff_add nope"))
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != service.ProductAddUsage() {
			t.Fatalf("expected product_add usage, got %q", got)
		}

		products.addErr = nil
		products.addResult = service.Product{ID: 4, Name: "Flowers", Cost: 20}
		processUpdate(t, harness, newTextUpdate(1, 100, "/stuff_add Flowers | 20"))

		if products.addCommand != "Flowers | 20" {
			t.Fatalf("unexpected product add command: %q", products.addCommand)
		}
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "Stuff added: #4 Flowers - 20 pan-coins" {
			t.Fatalf("unexpected product_add success: %q", got)
		}
	})

	t.Run("product remove returns usage, missing, and success", func(t *testing.T) {
		products := &productProviderSpy{removeErr: service.ErrProductIDInvalid}
		harness := newTestBotHarness(t, testBotDeps{
			products:            products,
			adminTelegramUserID: 1,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/stuff_remove nope"))
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != service.ProductRemoveUsage() {
			t.Fatalf("expected product_remove usage, got %q", got)
		}

		products.removeErr = service.ErrProductNotFound
		processUpdate(t, harness, newTextUpdate(1, 100, "/stuff_remove 7"))
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "That stuff does not exist." {
			t.Fatalf("unexpected product_remove missing response: %q", got)
		}

		products.removeErr = nil
		products.removeResult = 7
		processUpdate(t, harness, newTextUpdate(1, 100, "/stuff_remove 7"))
		if products.removeCommand != "7" {
			t.Fatalf("unexpected product remove command: %q", products.removeCommand)
		}
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "Stuff #7 removed." {
			t.Fatalf("unexpected product_remove success: %q", got)
		}
	})

	t.Run("money set forbids non admin, validates payload, and updates", func(t *testing.T) {
		users := &userProviderSpy{
			setMoneyResult: service.User{TelegramUserID: 5, FirstName: "Mia", Type: "wife", Money: 80},
		}
		harness := newTestBotHarness(t, testBotDeps{
			users:               users,
			adminTelegramUserID: 1,
		})

		nonAdminHarness := newTestBotHarness(t, testBotDeps{
			users:               &userProviderSpy{},
			adminTelegramUserID: 99,
		})
		processUpdate(t, nonAdminHarness, newTextUpdate(1, 100, "/money_set 5 80"))
		if got := nonAdminHarness.client.lastRequest("sendMessage").Fields["text"]; got != "Only admin can use market admin commands." {
			t.Fatalf("unexpected non-admin money_set response: %q", got)
		}

		processUpdate(t, harness, newTextUpdate(1, 100, "/money_set nope"))
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != service.MoneySetUsage() {
			t.Fatalf("expected money_set usage, got %q", got)
		}

		processUpdate(t, harness, newTextUpdate(1, 100, "/money_set 5 80"))
		if users.setMoneyUserID != 5 || users.setMoneyValue != 80 {
			t.Fatalf("unexpected money_set inputs: id=%d money=%d", users.setMoneyUserID, users.setMoneyValue)
		}
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "Mia now has 80 pan-coins." {
			t.Fatalf("unexpected money_set success: %q", got)
		}
	})
}

func TestProductsCommandAndCallback(t *testing.T) {
	t.Run("products returns empty state", func(t *testing.T) {
		products := &productProviderSpy{}
		users := &userProviderSpy{
			isRegisteredResult: true,
			getResult:          service.User{TelegramUserID: 1, FirstName: "Anna", Type: "wife", Money: 50},
		}
		harness := newTestBotHarness(t, testBotDeps{
			products:            products,
			users:               users,
			adminTelegramUserID: 99,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/stuffs"))

		request := harness.client.lastRequest("sendMessage")
		if request.Fields["text"] != "No stuff available yet.\nYour balance: 50 pan-coins" {
			t.Fatalf("unexpected products empty state: %q", request.Fields["text"])
		}
	})

	t.Run("products renders keyboard", func(t *testing.T) {
		products := &productProviderSpy{
			listResult: []service.Product{
				{ID: 4, Name: "Flowers", Cost: 20},
				{ID: 5, Name: "Cake", Cost: 35},
			},
		}
		users := &userProviderSpy{
			isRegisteredResult: true,
			getResult:          service.User{TelegramUserID: 1, FirstName: "Anna", Type: "wife", Money: 50},
		}
		harness := newTestBotHarness(t, testBotDeps{
			products:            products,
			users:               users,
			adminTelegramUserID: 99,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/stuffs"))

		request := harness.client.lastRequest("sendMessage")
		if !strings.Contains(request.Fields["text"], "Your balance: 50 pan-coins") {
			t.Fatalf("expected products text to include balance, got %q", request.Fields["text"])
		}
		if !strings.Contains(request.Fields["text"], "#4 Flowers - 20 pan-coins") {
			t.Fatalf("expected products text to list Flowers, got %q", request.Fields["text"])
		}
		markup := parseInlineKeyboardMarkup(t, request.Fields["reply_markup"])
		if !inlineKeyboardContains(markup, "Flowers - 20 pan-coins") || !inlineKeyboardContains(markup, "Cake - 35 pan-coins") {
			t.Fatalf("unexpected products keyboard: %#v", markup)
		}
	})

	t.Run("successful purchase sends buyer and partner messages", func(t *testing.T) {
		products := &productProviderSpy{
			buyResult: service.PurchaseResult{
				Product:                service.Product{ID: 4, Name: "Flowers", Cost: 20},
				BuyerTelegramUserID:    1,
				BuyerFirstName:         "Anna",
				BuyerType:              "wife",
				BuyerMoneyAfter:        30,
				OppositeTelegramUserID: 2,
				OppositeFirstName:      "Ivan",
				OppositeType:           "husband",
			},
		}
		users := &userProviderSpy{isRegisteredResult: true}
		harness := newTestBotHarness(t, testBotDeps{
			products:            products,
			users:               users,
			adminTelegramUserID: 99,
		})

		processUpdate(t, harness, newCallbackUpdate(1, 100, marketCallbackPrefix+"buy:4"))

		requests := harness.client.requestsFor("sendMessage")
		if len(requests) != 2 {
			t.Fatalf("expected 2 sendMessage requests, got %d", len(requests))
		}
		if requests[0].Fields["text"] != "You bought Flowers for 20 pan-coins. Remaining balance: 30 pan-coins." {
			t.Fatalf("unexpected buyer message: %q", requests[0].Fields["text"])
		}
		if requests[1].Fields["chat_id"] != "2" || requests[1].Fields["text"] != "Anna bought Flowers for 20 pan-coins." {
			t.Fatalf("unexpected partner message: %#v", requests[1])
		}
		if products.buyBuyerID != 1 || products.buyProductID != 4 {
			t.Fatalf("unexpected purchase inputs: buyer=%d product=%d", products.buyBuyerID, products.buyProductID)
		}
	})

	t.Run("insufficient funds sends failure only", func(t *testing.T) {
		products := &productProviderSpy{buyErr: service.ErrProductInsufficientFunds}
		users := &userProviderSpy{isRegisteredResult: true}
		harness := newTestBotHarness(t, testBotDeps{
			products:            products,
			users:               users,
			adminTelegramUserID: 99,
		})

		processUpdate(t, harness, newCallbackUpdate(1, 100, marketCallbackPrefix+"buy:4"))

		requests := harness.client.requestsFor("sendMessage")
		if len(requests) != 1 {
			t.Fatalf("expected 1 sendMessage request, got %d", len(requests))
		}
		if requests[0].Fields["text"] != "You do not have enough pan-coins for this stuff." {
			t.Fatalf("unexpected insufficient funds response: %q", requests[0].Fields["text"])
		}
	})

	t.Run("stale or invalid product callback fails cleanly", func(t *testing.T) {
		products := &productProviderSpy{buyErr: service.ErrProductNotFound}
		users := &userProviderSpy{isRegisteredResult: true}
		harness := newTestBotHarness(t, testBotDeps{
			products:            products,
			users:               users,
			adminTelegramUserID: 99,
		})

		processUpdate(t, harness, newCallbackUpdate(1, 100, marketCallbackPrefix+"buy:4"))
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "That stuff is no longer available." {
			t.Fatalf("unexpected stale product response: %q", got)
		}

		processUpdate(t, harness, newCallbackUpdate(1, 100, marketCallbackPrefix+"oops"))
		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "I did not understand that stuff choice." {
			t.Fatalf("unexpected invalid callback response: %q", got)
		}
	})

	t.Run("products load failure returns retry", func(t *testing.T) {
		products := &productProviderSpy{listErr: errors.New("boom")}
		users := &userProviderSpy{
			isRegisteredResult: true,
			getResult:          service.User{TelegramUserID: 1, FirstName: "Anna", Type: "wife", Money: 50},
		}
		harness := newTestBotHarness(t, testBotDeps{
			products:            products,
			users:               users,
			adminTelegramUserID: 99,
		})

		processUpdate(t, harness, newTextUpdate(1, 100, "/stuffs"))

		if got := harness.client.lastRequest("sendMessage").Fields["text"]; got != "I could not load stuff right now. Please try again in a moment." {
			t.Fatalf("unexpected products load failure response: %q", got)
		}
	})
}
