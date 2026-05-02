CREATE TABLE IF NOT EXISTS love_note_user_cycle (
    user_id INTEGER NOT NULL,
    love_note_id INTEGER NOT NULL,
    delivered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, love_note_id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (love_note_id) REFERENCES love_notes(id)
);

CREATE TABLE IF NOT EXISTS memory_user_cycle (
    user_id INTEGER NOT NULL,
    memory_id INTEGER NOT NULL,
    delivered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, memory_id),
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (memory_id) REFERENCES memories(id)
);

CREATE INDEX IF NOT EXISTS idx_love_note_user_cycle_user_id
    ON love_note_user_cycle(user_id);

CREATE INDEX IF NOT EXISTS idx_love_note_user_cycle_love_note_id
    ON love_note_user_cycle(love_note_id);

CREATE INDEX IF NOT EXISTS idx_memory_user_cycle_user_id
    ON memory_user_cycle(user_id);

CREATE INDEX IF NOT EXISTS idx_memory_user_cycle_memory_id
    ON memory_user_cycle(memory_id);
