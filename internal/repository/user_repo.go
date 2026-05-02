package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

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
		INSERT INTO users (telegram_user_id, first_name, type, money)
		VALUES (?, ?, ?, 0)
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
		SELECT id, telegram_user_id, first_name, type, money, created_at
		FROM users
		WHERE id = ?
	`, userID)

	var user User
	if err := row.Scan(&user.ID, &user.TelegramUserID, &user.FirstName, &user.Type, &user.Money, &user.CreatedAt); err != nil {
		return User{}, fmt.Errorf("scan inserted user: %w", err)
	}

	return user, nil
}

func (r *UserRepository) FindByTelegramUserID(ctx context.Context, telegramUserID int64) (User, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, telegram_user_id, first_name, type, money, created_at
		FROM users
		WHERE telegram_user_id = ?
		LIMIT 1
	`, telegramUserID)

	var user User
	if err := row.Scan(&user.ID, &user.TelegramUserID, &user.FirstName, &user.Type, &user.Money, &user.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("find user by telegram id: %w", err)
	}

	return user, nil
}

func (r *UserRepository) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, telegram_user_id, first_name, type, money, created_at
		FROM users
		ORDER BY lower(first_name) ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	users := make([]User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.TelegramUserID, &user.FirstName, &user.Type, &user.Money, &user.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan listed user: %w", err)
		}
		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate listed users: %w", err)
	}

	return users, nil
}

func (r *UserRepository) SetMoneyByTelegramUserID(ctx context.Context, telegramUserID int64, money int64) (User, error) {
	result, err := r.db.ExecContext(ctx, `
		UPDATE users
		SET money = ?
		WHERE telegram_user_id = ?
	`, money, telegramUserID)
	if err != nil {
		return User{}, fmt.Errorf("set user money: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return User{}, fmt.Errorf("read updated user rows: %w", err)
	}
	if rowsAffected == 0 {
		return User{}, ErrUserNotFound
	}

	return r.FindByTelegramUserID(ctx, telegramUserID)
}

func userIDByTelegramUserID(ctx context.Context, db queryRower, telegramUserID int64) (int64, error) {
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
