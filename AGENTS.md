# AGENTS.md

## Project Overview

`fambow` is a Go Telegram bot for couples. It currently focuses on lightweight relationship-oriented flows: love notes, memory capture, reminders, celebration dates, and admin-managed access control.

The app starts in [`cmd/bot/main.go`](cmd/bot/main.go), wires dependencies in [`internal/app/app.go`](internal/app/app.go), stores state in SQLite, applies SQL migrations on startup, and runs a minute-based scheduler for due reminders and celebration notifications.

## Current Features

- Access control backed by `users` table
- Admin-only `/create_user <telegram_id> <first_name> <husband|wife>`
- `/start` and `/help` onboarding messages
- `/love` and `Love Note` quick button for random love notes
- `/add_love ...` for inserting new love notes
- `/memory ...` text memories
- `/memory` photo-caption memories with Telegram file IDs persisted
- Memory intake button flow via `Memory`
- `/memories` recent memory listing
- `/surprise_memory` random memory retrieval
- `/remind ...` text reminder creation
- Guided reminder wizard via `/reminder` and `Reminder` button
- Partner-targeted reminders with `him` / `her`
- `/reminders` active reminder listing
- `/event add ...` celebration date creation
- `/events` celebration listing
- Scheduler dispatch for due one-time reminders, daily reminders, and celebration alerts

## Technology Stack

- Language: Go `1.24`
- Telegram SDK: `github.com/go-telegram/bot`
- Scheduler: `github.com/robfig/cron/v3`
- Database: SQLite via `modernc.org/sqlite`
- Logging: standard library `log/slog` with JSON handler
- Configuration: environment variables plus lightweight `.env` loader in [`internal/app/config.go`](internal/app/config.go)
- Testing: Go `testing` package with focused unit tests in `internal/service` and `internal/telegram`

## Repository Map

- `cmd/bot`: process entrypoint and shutdown handling
- `internal/app`: config loading, logger construction, dependency wiring
- `internal/telegram`: Telegram handlers, keyboards, access guard, conversational state
- `internal/service`: parsing rules and business logic
- `internal/repository`: SQLite access and persistence models
- `internal/scheduler`: periodic dispatch of due reminders and events
- `migrations`: schema bootstrap SQL

## Data Model

The current schema in [`migrations/001_init.sql`](migrations/001_init.sql) defines:

- `users`
- `love_notes`
- `reminders`
- `reminder_dispatches`
- `memories`
- `events`
- `event_dispatches`

Important implementation detail: reminder and event dispatch idempotency is enforced through dispatch tables rather than job queue infrastructure.

## Run And Verify

- Start bot: `go run ./cmd/bot`
- Run tests: `go test ./...`

Required environment variables:

- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_ADMIN_USER_ID`

Optional environment variables:

- `LOG_LEVEL`
- `DATABASE_PATH`
- `MIGRATION_FILE`

## Agent Working Notes

- Keep changes consistent with the existing architecture: handler -> service -> repository.
- Put Telegram-specific parsing and UX flow in `internal/telegram` only when it is truly transport/UI logic.
- Put business rules, validation, and payload parsing in `internal/service`.
- Put SQL and persistence concerns in `internal/repository`.
- Preserve SQLite compatibility; do not introduce database-specific features that break `modernc.org/sqlite`.
- Migrations are startup-applied from SQL files. Schema changes should be additive and explicit.
- Access control is part of the product, not a temporary stub. Do not bypass it accidentally in new handlers.
- The scheduler runs every minute. Time-sensitive features should be designed around minute granularity unless the design is intentionally changed across service, repository, and scheduler layers.
- Existing tests are narrow and unit-focused. Add or update tests when changing parsing, state machines, or scheduling behavior.
- The worktree may already contain unrelated user changes. Do not revert them.

## Known Product Constraints

- User roles are currently limited to `husband` and `wife`.
- Memories can include optional custom dates, but future dates are rejected.
- Random memory retrieval is global across saved memories, not scoped by requesting user.
- Reminder wizard state is in-memory, so it resets on process restart.
- Scheduler behavior depends on local process time via `time.Now()` and `time.Local`.
