package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"

	"fambow/internal/repository"
	"fambow/internal/scheduler"
	"fambow/internal/service"
	"fambow/internal/telegram"
)

type App struct {
	cfg       Config
	logger    *slog.Logger
	bot       telegram.BotRunner
	scheduler *scheduler.CronScheduler
	db        *sql.DB
}

func New(cfg Config, logger *slog.Logger) (*App, error) {
	db, err := repository.OpenSQLite(cfg.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := repository.RunMigrations(context.Background(), db, cfg.MigrationsDir); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	loveNoteRepo := repository.NewLoveNoteRepository(db)
	loveNoteService := service.NewLoveNoteService(loveNoteRepo)
	if err := loveNoteService.SeedDefaults(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("seed default love notes: %w", err)
	}

	memoryRepo := repository.NewMemoryRepository(db)
	memoryService := service.NewMemoryService(memoryRepo)
	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo)
	reminderRepo := repository.NewReminderRepository(db)
	reminderService := service.NewReminderService(reminderRepo)
	loveNoteScheduleRepo := repository.NewLoveNoteScheduleRepository(db)
	loveNoteScheduleService := service.NewLoveNoteScheduleService(loveNoteScheduleRepo)
	celebrationRepo := repository.NewCelebrationRepository(db)
	celebrationService := service.NewCelebrationService(celebrationRepo)
	productRepo := repository.NewProductRepository(db)
	productService := service.NewProductService(productRepo)

	b, err := telegram.NewBot(cfg.BotToken, logger, loveNoteService, memoryService, reminderService, loveNoteScheduleService, celebrationService, productService, userService, cfg.AdminTelegramUserID)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}

	cronScheduler, err := scheduler.NewCronScheduler(logger, b, loveNoteService, loveNoteScheduleService, reminderService, celebrationService)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create scheduler: %w", err)
	}

	return &App{
		cfg:       cfg,
		logger:    logger,
		bot:       b,
		scheduler: cronScheduler,
		db:        db,
	}, nil
}

func NewLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func (a *App) Run(ctx context.Context) error {
	a.logger.Info("starting telegram bot")
	if a.scheduler != nil {
		go a.scheduler.Start(ctx)
	}
	a.bot.Start(ctx)

	if err := a.db.Close(); err != nil {
		a.logger.Warn("failed to close database", "error", err)
	}

	return nil
}
