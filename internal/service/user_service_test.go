package service

import (
	"context"
	"errors"
	"testing"

	"fambow/internal/repository"
)

type userStoreSpy struct {
	createTelegramUserID int64
	createFirstName      string
	createType           string
	createResult         repository.User
	createErr            error
	listResult           []repository.User
	listErr              error
	findResult           repository.User
	findErr              error
	setMoneyUserID       int64
	setMoneyValue        int64
	setMoneyResult       repository.User
	setMoneyErr          error
}

func (s *userStoreSpy) ExistsByTelegramUserID(context.Context, int64) (bool, error) {
	return false, nil
}

func (s *userStoreSpy) CreateUser(_ context.Context, telegramUserID int64, firstName string, userType string) (repository.User, error) {
	s.createTelegramUserID = telegramUserID
	s.createFirstName = firstName
	s.createType = userType
	return s.createResult, s.createErr
}

func (s *userStoreSpy) FindByTelegramUserID(context.Context, int64) (repository.User, error) {
	return s.findResult, s.findErr
}

func (s *userStoreSpy) ListUsers(context.Context) ([]repository.User, error) {
	return s.listResult, s.listErr
}

func (s *userStoreSpy) SetMoneyByTelegramUserID(_ context.Context, telegramUserID int64, money int64) (repository.User, error) {
	s.setMoneyUserID = telegramUserID
	s.setMoneyValue = money
	return s.setMoneyResult, s.setMoneyErr
}

func TestUserServiceCreateUserValidation(t *testing.T) {
	store := &userStoreSpy{}
	svc := NewUserService(store)

	tests := []struct {
		name    string
		id      int64
		nameArg string
		typeArg string
		wantErr error
	}{
		{name: "invalid telegram id", id: 0, nameArg: "Anna", typeArg: "wife", wantErr: ErrUserTelegramIDInvalid},
		{name: "empty first name", id: 1, nameArg: "   ", typeArg: "wife", wantErr: ErrUserFirstNameEmpty},
		{name: "invalid user type", id: 1, nameArg: "Anna", typeArg: "friend", wantErr: ErrUserTypeInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateUser(context.Background(), tt.id, tt.nameArg, tt.typeArg)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestUserServiceCreateUserMapsRepositoryErrors(t *testing.T) {
	store := &userStoreSpy{createErr: repository.ErrUserAlreadyExists}
	svc := NewUserService(store)

	_, err := svc.CreateUser(context.Background(), 5, "Mia", "wife")
	if !errors.Is(err, ErrUserAlreadyExists) {
		t.Fatalf("expected ErrUserAlreadyExists, got %v", err)
	}
}

func TestUserServiceCreateUserNormalizesInputs(t *testing.T) {
	store := &userStoreSpy{
		createResult: repository.User{
			TelegramUserID: 5,
			FirstName:      "Mia",
			Type:           repository.UserTypeWife,
		},
	}
	svc := NewUserService(store)

	user, err := svc.CreateUser(context.Background(), 5, " Mia ", "WIFE")
	if err != nil {
		t.Fatalf("CreateUser() unexpected error: %v", err)
	}

	if store.createTelegramUserID != 5 || store.createFirstName != "Mia" || store.createType != "wife" {
		t.Fatalf("unexpected normalized inputs: id=%d name=%q type=%q", store.createTelegramUserID, store.createFirstName, store.createType)
	}
	if user.FirstName != "Mia" || user.Type != "wife" {
		t.Fatalf("unexpected returned user: %#v", user)
	}
}

func TestUserServiceListUsers(t *testing.T) {
	store := &userStoreSpy{
		listResult: []repository.User{{
			TelegramUserID: 5,
			FirstName:      "Mia",
			Type:           repository.UserTypeWife,
		}},
	}
	svc := NewUserService(store)

	users, err := svc.ListUsers(context.Background())
	if err != nil {
		t.Fatalf("ListUsers() unexpected error: %v", err)
	}
	if len(users) != 1 || users[0].TelegramUserID != 5 || users[0].FirstName != "Mia" || users[0].Type != "wife" {
		t.Fatalf("unexpected listed users: %#v", users)
	}
}

func TestUserServiceSetMoney(t *testing.T) {
	store := &userStoreSpy{
		setMoneyResult: repository.User{
			TelegramUserID: 5,
			FirstName:      "Mia",
			Type:           repository.UserTypeWife,
			Money:          80,
		},
	}
	svc := NewUserService(store)

	user, err := svc.SetMoney(context.Background(), 5, 80)
	if err != nil {
		t.Fatalf("SetMoney() unexpected error: %v", err)
	}
	if store.setMoneyUserID != 5 || store.setMoneyValue != 80 {
		t.Fatalf("unexpected set money inputs: id=%d money=%d", store.setMoneyUserID, store.setMoneyValue)
	}
	if user.Money != 80 {
		t.Fatalf("unexpected set money result: %#v", user)
	}
}
