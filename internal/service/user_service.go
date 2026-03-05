package service

import (
	"context"
	"errors"
	"strings"

	"fambow/internal/repository"
)

var ErrUserTelegramIDInvalid = errors.New("invalid telegram user id")
var ErrUserFirstNameEmpty = errors.New("user first name cannot be empty")
var ErrUserTypeInvalid = errors.New("invalid user type")
var ErrUserAlreadyExists = errors.New("user already exists")

type User struct {
	TelegramUserID int64
	FirstName      string
	Type           string
}

type UserStore interface {
	ExistsByTelegramUserID(ctx context.Context, telegramUserID int64) (bool, error)
	CreateUser(ctx context.Context, telegramUserID int64, firstName string, userType string) (repository.User, error)
}

type UserService struct {
	store UserStore
}

func NewUserService(store UserStore) *UserService {
	return &UserService{store: store}
}

func (s *UserService) IsRegistered(ctx context.Context, telegramUserID int64) (bool, error) {
	if telegramUserID <= 0 {
		return false, ErrUserTelegramIDInvalid
	}

	return s.store.ExistsByTelegramUserID(ctx, telegramUserID)
}

func (s *UserService) CreateUser(ctx context.Context, telegramUserID int64, firstName string, userType string) (User, error) {
	if telegramUserID <= 0 {
		return User{}, ErrUserTelegramIDInvalid
	}

	normalizedName := strings.TrimSpace(firstName)
	if normalizedName == "" {
		return User{}, ErrUserFirstNameEmpty
	}

	normalizedType := strings.TrimSpace(strings.ToLower(userType))
	if normalizedType != repository.UserTypeHusband && normalizedType != repository.UserTypeWife {
		return User{}, ErrUserTypeInvalid
	}

	record, err := s.store.CreateUser(ctx, telegramUserID, normalizedName, normalizedType)
	if err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			return User{}, ErrUserAlreadyExists
		}
		return User{}, err
	}

	return User{
		TelegramUserID: record.TelegramUserID,
		FirstName:      record.FirstName,
		Type:           record.Type,
	}, nil
}
