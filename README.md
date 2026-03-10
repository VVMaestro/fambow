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
3. Set `TELEGRAM_ADMIN_USER_ID` in `.env` to your Telegram numeric user ID.
4. Run the bot:

   ```bash
   go run ./cmd/bot
   ```

## Current Commands

- `/start` - warm welcome
- `/help` - command list
- `/love` - instant love note
- `/start` and `/help` show quick keyboard buttons `Love Note` and `Memory`
- `Memory` button flow: tap `Memory`, then send your next text or photo with optional caption to save
- `/memory <text>` - save a text memory
- `/memory YYYY-MM-DD | <text>` - save memory with custom date (date cannot be in the future)
- photo with caption `/memory <text optional>` - save a memory with attached photo
- photo with caption `/memory YYYY-MM-DD | <text optional>` - save photo memory with custom date
- `/memories` - list recent memories (re-sends saved photos)
- `/surprise_memory` - send one random memory from all saved memories
- `/remind at 19:30 <text>` - one-time reminder (today or tomorrow)
- `/remind daily 08:00 <text>` - recurring daily reminder
- `/remind him at 19:30 to <text>` - reminder for husband user
- `/remind her at 19:30 to <text>` - reminder for wife user
- `/reminders` - list active reminders
- `/event add YYYY-MM-DD | Title | 3` - add a celebration date
- `/events` - list celebration dates
- `/create_user <telegram_id> <first_name> <husband|wife>` - admin only, adds user to access list

## Access Control

- Every command requires the sender to exist in `users` table.
- Users not in `users` are blocked from interacting with the bot.
- Admin Telegram ID from `TELEGRAM_ADMIN_USER_ID` bypasses user check and can run `/create_user`.

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
