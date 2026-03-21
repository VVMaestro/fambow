package app

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	BotToken            string
	LogLevel            slog.Level
	DatabasePath        string
	MigrationsDir       string
	AdminTelegramUserID int64
}

func LoadConfig() (Config, error) {
	_ = loadDotEnv(".env")

	botToken := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if botToken == "" {
		return Config{}, errors.New("TELEGRAM_BOT_TOKEN is required")
	}

	adminTelegramUserID, err := parseInt64Env("TELEGRAM_ADMIN_USER_ID")
	if err != nil {
		return Config{}, err
	}

	return Config{
		BotToken:            botToken,
		LogLevel:            parseLogLevel(os.Getenv("LOG_LEVEL")),
		DatabasePath:        getEnvOrDefault("DATABASE_PATH", "fambow.db"),
		MigrationsDir:       getMigrationsDir(),
		AdminTelegramUserID: adminTelegramUserID,
	}, nil
}

func getMigrationsDir() string {
	if value := strings.TrimSpace(os.Getenv("MIGRATIONS_DIR")); value != "" {
		return value
	}

	legacyMigrationFile := getEnvOrDefault("MIGRATION_FILE", filepath.Join("migrations", "001_init.sql"))
	return filepath.Dir(legacyMigrationFile)
}

func parseInt64Env(key string) (int64, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, fmt.Errorf("%s is required", key)
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}

	return value, nil
}

func getEnvOrDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func loadDotEnv(path string) error {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}

		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			key, value, ok := strings.Cut(line, "=")
			if ok {
				key = strings.TrimSpace(key)
				value = strings.TrimSpace(value)
				if key != "" {
					if _, exists := os.LookupEnv(key); !exists {
						_ = os.Setenv(key, strings.Trim(value, "\"'"))
					}
				}
			}
		}

		if errors.Is(err, io.EOF) {
			break
		}
	}

	return nil
}

func parseLogLevel(raw string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
