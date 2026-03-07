# Telegram Bot Gift Plan (Go)

This document outlines several bot ideas for your wife and a practical implementation roadmap using Go.

---

## 1) Product Vision

Build a warm, personal Telegram bot that feels like a thoughtful daily companion rather than a generic utility.

Core goals:
- Personal and loving tone
- Useful in everyday routine
- Low maintenance for you
- Easy to expand over time

---

## 2) Bot Ideas (Feature Catalog)

Use these as a menu. Start with 3-5 features for V1, then add more.

### A. Daily Love Note
- Sends a short heartfelt message every morning
- Supports rotating templates and random variations
- Optional: include a quote or emoji style she likes

### B. Mood Check-In
- Bot asks: "How are you feeling today?"
- She can tap quick buttons (Great / Okay / Tired / Stressed)
- Bot responds with supportive text and simple suggestions
- Optional: weekly mood summary for you (private admin chat)

### C. Smart Reminder Assistant
- Reminders for water, vitamins, meds, gym, calls, etc.
- Natural flow: `/remind me at 19:30 to drink tea`
- Recurring reminders (daily, weekdays, custom)

### D. Shared Memory Jar
- Save happy moments via `/memory` command
- Example: "Today we walked by the river"
- Bot can send "On this day" memories monthly

### E. Date Night Picker
- Suggests date ideas from categories (home / outside / low-budget)
- Randomized with tags (romantic, cozy, active)
- Command: `/datenight`

### F. Compliment Generator (Personalized)
- Generates compliments from your custom list
- Command: `/compliment`
- Optional: style modes (cute, elegant, funny)

### G. Mini Wellness Coach
- Gentle breathing prompt
- 2-minute stretch routine text
- Sleep wind-down reminder

### H. Gratitude Journal Prompt
- Evening prompt: "What made you smile today?"
- Saves entries privately
- Weekly recap with highlights

### I. Couple Goals Tracker
- Track shared goals (travel fund, workouts, books)
- Simple command set: `/goal add`, `/goal done`, `/goal status`

### J. Celebration Calendar
- Birthday, anniversary, milestone countdown
- Auto reminders days before key dates

---

## 3) Recommended V1 Scope (Gift-Ready in 1-2 Weeks)

Pick a focused V1 that feels polished:

1. Daily Love Note
2. Smart Reminder Assistant
3. Date Night Picker
4. Shared Memory Jar
5. Celebration Calendar

Why this set:
- High emotional value
- Immediately useful
- Technically straightforward
- Strong demo effect when you present it

---

## 4) Go Technical Stack

### Language & Runtime
- Go 1.23+ (or latest stable)

### Telegram Library
- `github.com/go-telegram/bot` (recommended modern API)
  - Alternative: `github.com/go-telegram-bot-api/telegram-bot-api/v5`

### Storage
- SQLite for local/simple setup
- Use `modernc.org/sqlite` (pure Go) or CGO sqlite driver

### Config & Secrets
- Environment variables
- `.env` for local development (do not commit)

### Scheduling
- `github.com/robfig/cron/v3` for recurring tasks

### Logging
- Go `slog` structured logging

### Optional Add-ons
- Docker for easy deployment
- `golang-migrate` for DB migrations

---

## 5) High-Level Architecture

```text
Telegram Updates (Webhook or Long Polling)
        |
        v
Command Router (/start, /remind, /memory, ...)
        |
        +--> Feature Services (love notes, reminders, memories, dates)
        |
        +--> Scheduler (cron jobs + one-off reminders)
        |
        +--> Repository Layer (SQLite)
```

Design principles:
- Keep handlers thin
- Business logic in service layer
- Data access in repository layer
- Add features as modules

---

## 6) Project Structure (Go)

```text
fambow/
  cmd/bot/main.go
  internal/
    app/
      app.go
      config.go
    telegram/
      router.go
      handlers_start.go
      handlers_reminder.go
      handlers_memory.go
      handlers_date_night.go
    service/
      love_note_service.go
      reminder_service.go
      memory_service.go
      celebration_service.go
    repository/
      models.go
      reminder_repo.go
      memory_repo.go
      celebration_repo.go
    scheduler/
      cron.go
  migrations/
    001_init.sql
  .env.example
  go.mod
  README.md
```

---

## 7) Data Model (Initial)

### `users`
- `id` (pk)
- `telegram_user_id` (unique)
- `first_name`
- `type` (husband / wife)
- `created_at`

### `love_notes`
- `id` (pk)
- `text`
- `tag`
- `created_at`

### `reminders`
- `id` (pk)
- `user_id`
- `text`
- `schedule_type` (one_time / daily / weekly)
- `schedule_value` (timestamp or cron-like metadata)
- `is_active`

### `memories`
- `id` (pk)
- `user_id`
- `text`
- `created_at`

### `events`
- `id` (pk)
- `user_id`
- `title`
- `event_date`
- `remind_days_before`

---

## 8) Telegram Commands (V1)

- `/start` - warm welcome + help menu
- `/help` - command list
- `/love` - instant love note
- `/remind` - create reminder interactively
- `/reminders` - list active reminders
- `/memory <text>` - save memory
- `/memories` - show recent memories
- `/datenight` - suggest one date plan
- `/event add` - add important date
- `/events` - list dates

---

## 9) Implementation Roadmap

## Phase 0 - Preparation (0.5 day)
- Create Telegram bot using @BotFather
- Get bot token
- Create private test chat with your bot
- Initialize Go module and repository skeleton

## Phase 1 - Core Bot Skeleton (1 day)
- Setup `main.go`, config loading, graceful shutdown
- Add Telegram client + update polling
- Implement `/start` and `/help`
- Add structured logging

Deliverable: bot is online and responds to commands.

## Phase 2 - Emotional Core Features (2 days)
- Implement Love Note service (`/love`)
- Add Memory Jar (`/memory`, `/memories`)
- Store in SQLite

Deliverable: personalized and sentimental features working.

## Phase 3 - Utility Features (2-3 days)
- Implement reminders with one-time + daily options
- Add scheduler for dispatching reminder messages
- Add Celebration Calendar and pre-event reminders

Deliverable: practical assistant behavior working autonomously.

## Phase 4 - Delight & Polish (1-2 days)
- Add Date Night Picker with categories
- Improve message tone and emoji consistency
- Add error handling and user-friendly fallbacks

Deliverable: bot feels polished and gift-ready.

## Phase 5 - Deployment (0.5-1 day)
- Choose hosting (Render/Fly.io/VPS)
- Configure env vars securely
- Set up process restart and log monitoring

Deliverable: always-on bot reachable from Telegram.

## Phase 6 - Additions
- ~~Add Photos to memories~~
- Add Voice Memos to memories
- ~~Split functionality to wife/husband interaction~~
- Add daily / random by command memories sharing
- Add custom date to memories and fix photo memories sharing order

---

## 10) Testing Plan

### Unit Tests
- Service logic (reminder parsing, date calculations)
- Repository methods with test database

### Integration Tests
- Command handlers with mocked Telegram client
- Scheduler-to-send pipeline

### Manual Acceptance Checklist
- `/start` response and keyboard buttons
- Reminder fires at expected time
- Memory save + retrieval works
- Date night suggestion returns varied options
- Event reminder triggers correctly

---

## 11) Security & Privacy

- Never commit bot token
- Restrict sensitive admin commands to your Telegram user ID
- Store only necessary personal data
- Provide simple `/delete_my_data` command (future-friendly)

---

## 12) Gift Presentation Idea

How to reveal it:
1. Prepare 3 "wow" commands (`/love`, `/datenight`, `/memory`)
2. Preload custom love notes and date ideas tied to your story
3. Show live reminder trigger during demo
4. End with a special command: `/surprise` containing a personal message

---

## 13) Future Upgrades (V2+)

- Voice note transcription
- AI-generated personalized responses
- Shared shopping/checklist mode
- Travel planner mini-module
- Photo memory tagging

---

## 14) Effort Estimate

- Basic V1: 5-8 focused days
- Polished V1 with deployment and tests: 8-12 days

If time is short, prioritize:
1. `/love`
2. `/remind`
3. `/datenight`

These three alone already make a great gift bot.
