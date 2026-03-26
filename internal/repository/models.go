package repository

import "time"

type User struct {
	ID             int64
	TelegramUserID int64
	FirstName      string
	Type           string
	Money          int64
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

type AdminReminderItem struct {
	ID             int64
	TelegramUserID int64
	FirstName      string
	UserType       string
	Text           string
	ScheduleType   string
	ScheduleValue  string
	IsActive       bool
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
	ID                 int64
	Text               string
	Tag                string
	TelegramFileID     string
	TelegramFileUnique string
	CreatedAt          time.Time
}

type LoveNoteSchedule struct {
	ID           int64
	UserID       int64
	ScheduleTime string
	IsActive     bool
	CreatedAt    time.Time
}

type Event struct {
	ID               int64
	UserID           int64
	Title            string
	EventDate        time.Time
	RemindDaysBefore int
}

type Product struct {
	ID        int64
	Name      string
	Cost      int64
	CreatedAt time.Time
}
