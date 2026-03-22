CREATE TABLE IF NOT EXISTS love_note_schedules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    schedule_time TEXT NOT NULL,
    is_active INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE IF NOT EXISTS love_note_schedule_dispatches (
    schedule_id INTEGER NOT NULL,
    dispatch_date DATE NOT NULL,
    PRIMARY KEY (schedule_id, dispatch_date),
    FOREIGN KEY (schedule_id) REFERENCES love_note_schedules(id)
);

CREATE INDEX IF NOT EXISTS idx_love_note_schedules_active_time
    ON love_note_schedules(is_active, schedule_time);
