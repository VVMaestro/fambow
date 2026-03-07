CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    telegram_user_id INTEGER NOT NULL UNIQUE,
    first_name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('husband', 'wife')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS love_notes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    text TEXT NOT NULL,
    tag TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS reminders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    text TEXT NOT NULL,
    schedule_type TEXT NOT NULL,
    schedule_value TEXT NOT NULL,
    is_active INTEGER NOT NULL DEFAULT 1,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS reminder_dispatches (
    reminder_id INTEGER NOT NULL,
    dispatch_date DATE NOT NULL,
    PRIMARY KEY (reminder_id, dispatch_date),
    FOREIGN KEY (reminder_id) REFERENCES reminders(id)
);

CREATE TABLE IF NOT EXISTS memories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    text TEXT NOT NULL,
    telegram_file_id TEXT,
    telegram_file_unique_id TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    event_date DATE NOT NULL,
    remind_days_before INTEGER NOT NULL DEFAULT 1,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS event_dispatches (
    event_id INTEGER NOT NULL,
    dispatch_date DATE NOT NULL,
    PRIMARY KEY (event_id, dispatch_date),
    FOREIGN KEY (event_id) REFERENCES events(id)
);

CREATE INDEX IF NOT EXISTS idx_reminders_active_type_value ON reminders(is_active, schedule_type, schedule_value);
CREATE INDEX IF NOT EXISTS idx_events_date_days_before ON events(event_date, remind_days_before);
CREATE INDEX IF NOT EXISTS idx_memories_telegram_file_unique_id ON memories(telegram_file_unique_id);
