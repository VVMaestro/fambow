# Fambow Telegram Bot

Gift-focused Telegram bot written in Go.

## Prerequisites

- Go 1.23+
- Telegram bot token from @BotFather

## Quick Start

1. Copy env file:

   ```bash
   cp .env.example .env
   ```

2. Set your token in `.env` (or export env vars in your shell).
3. Run the bot:

   ```bash
   go run ./cmd/bot
   ```

## Current Commands

- `/start` - warm welcome
- `/help` - command list
- `/love` - instant love note
- `/memory <text>` - save a happy moment
- `/memories` - list recent memories
- `/remind at 19:30 <text>` - one-time reminder (today or tomorrow)
- `/remind daily 08:00 <text>` - recurring daily reminder
- `/reminders` - list active reminders
- `/event add YYYY-MM-DD | Title | 3` - add a celebration date
- `/events` - list celebration dates

## Data Storage

- Uses SQLite file database (default `fambow.db`)
- Applies schema from `migrations/001_init.sql` on startup
- Seeds default love notes into `love_notes` table when empty
- Scheduler checks every minute and dispatches due reminders/events

## Project Structure

- `cmd/bot/main.go` - entrypoint and graceful shutdown
- `internal/app` - config, app wiring, logger
- `internal/telegram` - bot client and command handlers
- `internal/service` - business logic placeholders
- `internal/repository` - data model placeholders
- `internal/scheduler` - scheduler placeholder
- `migrations/001_init.sql` - initial database schema
