package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

const (
	UserTypeHusband = "husband"
	UserTypeWife    = "wife"
)

var ErrUserTypeNotFound = errors.New("user type not found")
var ErrUserNotFound = errors.New("user not found")
var ErrUserAlreadyExists = errors.New("user already exists")

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) ExistsByTelegramUserID(ctx context.Context, telegramUserID int64) (bool, error) {
	_, err := userIDByTelegramUserID(ctx, r.db, telegramUserID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (r *UserRepository) CreateUser(ctx context.Context, telegramUserID int64, firstName string, userType string) (User, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO users (telegram_user_id, first_name, type)
		VALUES (?, ?, ?)
	`, telegramUserID, strings.TrimSpace(firstName), strings.TrimSpace(strings.ToLower(userType)))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return User{}, ErrUserAlreadyExists
		}
		return User{}, fmt.Errorf("insert user: %w", err)
	}

	userID, err := result.LastInsertId()
	if err != nil {
		return User{}, fmt.Errorf("get inserted user id: %w", err)
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT id, telegram_user_id, first_name, type, created_at
		FROM users
		WHERE id = ?
	`, userID)

	var user User
	if err := row.Scan(&user.ID, &user.TelegramUserID, &user.FirstName, &user.Type, &user.CreatedAt); err != nil {
		return User{}, fmt.Errorf("scan inserted user: %w", err)
	}

	return user, nil
}

func userIDByTelegramUserID(ctx context.Context, db *sql.DB, telegramUserID int64) (int64, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id
		FROM users
		WHERE telegram_user_id = ?
		LIMIT 1
	`, telegramUserID)

	var userID int64
	if err := row.Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrUserNotFound
		}
		return 0, fmt.Errorf("fetch user id by telegram id: %w", err)
	}

	return userID, nil
}

func userIDByType(ctx context.Context, db *sql.DB, userType string) (int64, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id
		FROM users
		WHERE lower(type) = lower(?)
		LIMIT 1
	`, userType)

	var userID int64
	if err := row.Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrUserTypeNotFound
		}
		return 0, fmt.Errorf("fetch user id by type: %w", err)
	}

	return userID, nil
}
