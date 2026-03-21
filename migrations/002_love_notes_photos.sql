ALTER TABLE love_notes ADD COLUMN telegram_file_id TEXT;
ALTER TABLE love_notes ADD COLUMN telegram_file_unique_id TEXT;

CREATE INDEX IF NOT EXISTS idx_love_notes_telegram_file_unique_id
    ON love_notes(telegram_file_unique_id);
