package repository

import "time"

type User struct {
	ID             int64
	TelegramUserID int64
	FirstName      string
	Type           string
	CreatedAt      time.Time
}

type Reminder struct {
	ID            int64
	UserID        int64
	Text          string
	ScheduleType  string
	ScheduleValue string
	IsActive      bool
}

type Memory struct {
	ID                 int64
	UserID             int64
	Text               string
	TelegramFileID     string
	TelegramFileUnique string
	CreatedAt          time.Time
}

type LoveNote struct {
	ID        int64
	Text      string
	Tag       string
	CreatedAt time.Time
}

type Event struct {
	ID               int64
	UserID           int64
	Title            string
	EventDate        time.Time
	RemindDaysBefore int
}
