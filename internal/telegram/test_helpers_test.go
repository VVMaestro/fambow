package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"fambow/internal/service"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type telegramRequest struct {
	Method string
	Fields map[string]string
}

type fakeTelegramClient struct {
	t        *testing.T
	mu       sync.Mutex
	requests []telegramRequest
}

func newFakeTelegramClient(t *testing.T) *fakeTelegramClient {
	t.Helper()
	return &fakeTelegramClient{t: t}
}

func (c *fakeTelegramClient) Do(req *http.Request) (*http.Response, error) {
	c.t.Helper()

	fields, err := parseTelegramRequest(req)
	if err != nil {
		return nil, err
	}

	method := path.Base(req.URL.Path)

	c.mu.Lock()
	c.requests = append(c.requests, telegramRequest{
		Method: method,
		Fields: fields,
	})
	c.mu.Unlock()

	body := `{"ok":true,"result":true}`
	switch method {
	case "sendMessage", "sendPhoto":
		body = `{"ok":true,"result":{"message_id":1}}`
	case "getMe":
		body = `{"ok":true,"result":{"id":123,"is_bot":true,"first_name":"test-bot"}}`
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

func parseTelegramRequest(req *http.Request) (map[string]string, error) {
	fields := make(map[string]string)
	if req.Body == nil {
		return fields, nil
	}

	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}

	mediaType, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("parse content type: %w", err)
	}

	if !strings.HasPrefix(mediaType, "multipart/") {
		fields["_raw"] = string(body)
		return fields, nil
	}

	reader := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read multipart: %w", err)
		}

		value, err := io.ReadAll(part)
		if err != nil {
			return nil, fmt.Errorf("read part %q: %w", part.FormName(), err)
		}
		fields[part.FormName()] = string(value)
	}

	return fields, nil
}

func (c *fakeTelegramClient) requestsFor(method string) []telegramRequest {
	c.mu.Lock()
	defer c.mu.Unlock()

	filtered := make([]telegramRequest, 0, len(c.requests))
	for _, request := range c.requests {
		if request.Method == method {
			filtered = append(filtered, request)
		}
	}

	return filtered
}

func (c *fakeTelegramClient) allRequests() []telegramRequest {
	c.mu.Lock()
	defer c.mu.Unlock()

	copied := make([]telegramRequest, len(c.requests))
	copy(copied, c.requests)
	return copied
}

func (c *fakeTelegramClient) lastRequest(method string) telegramRequest {
	c.t.Helper()

	requests := c.requestsFor(method)
	if len(requests) == 0 {
		c.t.Fatalf("expected %s request to be sent", method)
	}

	return requests[len(requests)-1]
}

type testBotHarness struct {
	bot    *bot.Bot
	client *fakeTelegramClient
}

type testBotDeps struct {
	loveNotes            LoveNoteProvider
	memories             MemoryProvider
	reminders            ReminderProvider
	loveSchedules        LoveNoteScheduleProvider
	celebrations         CelebrationProvider
	products             ProductProvider
	users                UserProvider
	adminTelegramUserID  int64
	registerMenuCommands bool
}

func newTestBotHarness(t *testing.T, deps testBotDeps) *testBotHarness {
	t.Helper()

	client := newFakeTelegramClient(t)
	b, err := bot.New(
		"123:token",
		bot.WithHTTPClient(time.Second, client),
		bot.WithSkipGetMe(),
		bot.WithNotAsyncHandlers(),
		bot.WithDefaultHandler(func(context.Context, *bot.Bot, *models.Update) {}),
	)
	if err != nil {
		t.Fatalf("bot.New() unexpected error: %v", err)
	}

	registerCoreHandlers(
		b,
		testLogger(),
		deps.loveNotes,
		deps.memories,
		deps.reminders,
		deps.loveSchedules,
		deps.celebrations,
		deps.products,
		deps.users,
		deps.adminTelegramUserID,
		newMemoryWizardState(),
		newReminderWizardState(),
		newEventWizardState(),
		newLoveScheduleWizardState(),
	)
	if deps.registerMenuCommands {
		registerMenuCommands(context.Background(), b, testLogger())
	}

	return &testBotHarness{
		bot:    b,
		client: client,
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func processUpdate(t *testing.T, h *testBotHarness, update *models.Update) {
	t.Helper()
	h.bot.ProcessUpdate(context.Background(), update)
}

func newTextUpdate(userID, chatID int64, text string) *models.Update {
	return &models.Update{
		Message: &models.Message{
			Text: text,
			Chat: models.Chat{ID: chatID},
			From: &models.User{
				ID:        userID,
				FirstName: "Anna",
			},
		},
	}
}

func newPhotoUpdate(userID, chatID int64, caption string, photos []models.PhotoSize) *models.Update {
	return &models.Update{
		Message: &models.Message{
			Caption: caption,
			Photo:   photos,
			Chat:    models.Chat{ID: chatID},
			From: &models.User{
				ID:        userID,
				FirstName: "Anna",
			},
		},
	}
}

func newCallbackUpdate(userID, chatID int64, data string) *models.Update {
	return &models.Update{
		CallbackQuery: &models.CallbackQuery{
			ID:   "callback-1",
			From: models.User{ID: userID, FirstName: "Anna"},
			Data: data,
			Message: models.MaybeInaccessibleMessage{
				Message: &models.Message{
					Chat: models.Chat{ID: chatID},
				},
			},
		},
	}
}

func parseReplyKeyboardMarkup(t *testing.T, raw string) models.ReplyKeyboardMarkup {
	t.Helper()

	var markup models.ReplyKeyboardMarkup
	if err := json.Unmarshal([]byte(raw), &markup); err != nil {
		t.Fatalf("unmarshal reply keyboard: %v", err)
	}

	return markup
}

func parseInlineKeyboardMarkup(t *testing.T, raw string) models.InlineKeyboardMarkup {
	t.Helper()

	var markup models.InlineKeyboardMarkup
	if err := json.Unmarshal([]byte(raw), &markup); err != nil {
		t.Fatalf("unmarshal inline keyboard: %v", err)
	}

	return markup
}

func replyKeyboardContains(markup models.ReplyKeyboardMarkup, label string) bool {
	for _, row := range markup.Keyboard {
		for _, button := range row {
			if strings.Contains(button.Text, label) {
				return true
			}
		}
	}

	return false
}

func inlineKeyboardContains(markup models.InlineKeyboardMarkup, label string) bool {
	for _, row := range markup.InlineKeyboard {
		for _, button := range row {
			if button.Text == label {
				return true
			}
		}
	}

	return false
}

type loveNoteProviderSpy struct {
	randomFirstName string
	randomResult    service.LoveNote
	randomErr       error
	addedNote       service.LoveNoteInput
	addErr          error
}

func (s *loveNoteProviderSpy) RandomNote(_ context.Context, firstName string) (service.LoveNote, error) {
	s.randomFirstName = firstName
	return s.randomResult, s.randomErr
}

func (s *loveNoteProviderSpy) AddLoveNote(_ context.Context, note service.LoveNoteInput) error {
	s.addedNote = note
	return s.addErr
}

type memoryProviderSpy struct {
	addUserID    int64
	addFirstName string
	addInput     service.MemoryInput
	addResult    service.Memory
	addErr       error

	recentUserID int64
	recentLimit  int
	recentResult []service.Memory
	recentErr    error

	randomResult service.Memory
	randomErr    error
}

func (s *memoryProviderSpy) AddMemory(_ context.Context, telegramUserID int64, firstName string, input service.MemoryInput) (service.Memory, error) {
	s.addUserID = telegramUserID
	s.addFirstName = firstName
	s.addInput = input
	return s.addResult, s.addErr
}

func (s *memoryProviderSpy) RecentMemories(_ context.Context, telegramUserID int64, limit int) ([]service.Memory, error) {
	s.recentUserID = telegramUserID
	s.recentLimit = limit
	return s.recentResult, s.recentErr
}

func (s *memoryProviderSpy) RandomMemory(_ context.Context) (service.Memory, error) {
	return s.randomResult, s.randomErr
}

type reminderProviderSpy struct {
	addUserID      int64
	addFirstName   string
	addCommand     string
	addResult      service.Reminder
	addErr         error
	targetUserType string
	targetCommand  string
	targetResult   service.Reminder
	targetErr      error
	listUserID     int64
	listResult     []service.Reminder
	listErr        error
}

func (s *reminderProviderSpy) AddReminder(_ context.Context, telegramUserID int64, firstName string, command string) (service.Reminder, error) {
	s.addUserID = telegramUserID
	s.addFirstName = firstName
	s.addCommand = command
	return s.addResult, s.addErr
}

func (s *reminderProviderSpy) AddReminderForUserType(_ context.Context, userType string, command string) (service.Reminder, error) {
	s.targetUserType = userType
	s.targetCommand = command
	return s.targetResult, s.targetErr
}

func (s *reminderProviderSpy) ListReminders(_ context.Context, telegramUserID int64) ([]service.Reminder, error) {
	s.listUserID = telegramUserID
	return s.listResult, s.listErr
}

type celebrationProviderSpy struct {
	addUserID    int64
	addFirstName string
	addCommand   string
	addResult    service.CelebrationEvent
	addErr       error
	listUserID   int64
	listResult   []service.CelebrationEvent
	listErr      error
}

func (s *celebrationProviderSpy) AddEvent(_ context.Context, telegramUserID int64, firstName string, command string) (service.CelebrationEvent, error) {
	s.addUserID = telegramUserID
	s.addFirstName = firstName
	s.addCommand = command
	return s.addResult, s.addErr
}

func (s *celebrationProviderSpy) ListEvents(_ context.Context, telegramUserID int64) ([]service.CelebrationEvent, error) {
	s.listUserID = telegramUserID
	return s.listResult, s.listErr
}

type loveScheduleProviderSpy struct {
	addTelegramUserID int64
	addScheduleTime   string
	addResult         service.LoveNoteSchedule
	addErr            error

	listResult []service.LoveNoteSchedule
	listErr    error

	removeScheduleID int64
	removeErr        error
}

func (s *loveScheduleProviderSpy) AddLoveNoteSchedule(_ context.Context, telegramUserID int64, scheduleTime string) (service.LoveNoteSchedule, error) {
	s.addTelegramUserID = telegramUserID
	s.addScheduleTime = scheduleTime
	return s.addResult, s.addErr
}

func (s *loveScheduleProviderSpy) ListLoveNoteSchedules(context.Context) ([]service.LoveNoteSchedule, error) {
	return s.listResult, s.listErr
}

func (s *loveScheduleProviderSpy) RemoveLoveNoteSchedule(_ context.Context, scheduleID int64) error {
	s.removeScheduleID = scheduleID
	return s.removeErr
}

type userProviderSpy struct {
	isRegisteredResult bool
	isRegisteredErr    error

	createTelegramUserID int64
	createFirstName      string
	createType           string
	createResult         service.User
	createErr            error

	getUserID int64
	getResult service.User
	getErr    error

	listResult []service.User
	listErr    error

	setMoneyUserID int64
	setMoneyValue  int64
	setMoneyResult service.User
	setMoneyErr    error
}

func (s *userProviderSpy) IsRegistered(_ context.Context, telegramUserID int64) (bool, error) {
	if telegramUserID == 0 {
		return false, s.isRegisteredErr
	}
	return s.isRegisteredResult, s.isRegisteredErr
}

func (s *userProviderSpy) CreateUser(_ context.Context, telegramUserID int64, firstName string, userType string) (service.User, error) {
	s.createTelegramUserID = telegramUserID
	s.createFirstName = firstName
	s.createType = userType
	return s.createResult, s.createErr
}

func (s *userProviderSpy) GetUser(_ context.Context, telegramUserID int64) (service.User, error) {
	s.getUserID = telegramUserID
	return s.getResult, s.getErr
}

func (s *userProviderSpy) ListUsers(context.Context) ([]service.User, error) {
	return s.listResult, s.listErr
}

func (s *userProviderSpy) SetMoney(_ context.Context, telegramUserID int64, money int64) (service.User, error) {
	s.setMoneyUserID = telegramUserID
	s.setMoneyValue = money
	return s.setMoneyResult, s.setMoneyErr
}

type productProviderSpy struct {
	addCommand    string
	addResult     service.Product
	addErr        error
	removeCommand string
	removeResult  int64
	removeErr     error
	listResult    []service.Product
	listErr       error
	buyBuyerID    int64
	buyProductID  int64
	buyResult     service.PurchaseResult
	buyErr        error
}

func (s *productProviderSpy) AddProduct(_ context.Context, command string) (service.Product, error) {
	s.addCommand = command
	return s.addResult, s.addErr
}

func (s *productProviderSpy) RemoveProduct(_ context.Context, command string) (int64, error) {
	s.removeCommand = command
	if s.removeResult != 0 {
		return s.removeResult, s.removeErr
	}
	return 0, s.removeErr
}

func (s *productProviderSpy) ListProducts(context.Context) ([]service.Product, error) {
	return s.listResult, s.listErr
}

func (s *productProviderSpy) BuyProduct(_ context.Context, buyerTelegramUserID int64, productID int64) (service.PurchaseResult, error) {
	s.buyBuyerID = buyerTelegramUserID
	s.buyProductID = productID
	return s.buyResult, s.buyErr
}
