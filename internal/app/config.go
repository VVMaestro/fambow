package app

import (
	"bufio"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	BotToken      string
	LogLevel      slog.Level
	DatabasePath  string
	MigrationFile string
}

func LoadConfig() (Config, error) {
	_ = loadDotEnv(".env")

	botToken := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	if botToken == "" {
		return Config{}, errors.New("TELEGRAM_BOT_TOKEN is required")
	}

	return Config{
		BotToken:      botToken,
		LogLevel:      parseLogLevel(os.Getenv("LOG_LEVEL")),
		DatabasePath:  getEnvOrDefault("DATABASE_PATH", "fambow.db"),
		MigrationFile: getEnvOrDefault("MIGRATION_FILE", "migrations/001_init.sql"),
	}, nil
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
