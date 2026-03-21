package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"fambow/internal/repository"
)

var ErrLoveNoteContentEmpty = errors.New("love note content cannot be empty")

type LoveNote struct {
	Text               string
	TelegramFileID     string
	TelegramFileUnique string
}

type LoveNoteInput struct {
	Text               string
	TelegramFileID     string
	TelegramFileUnique string
}

type LoveNoteStore interface {
	AddDefaultNotes(ctx context.Context, notes []string) error
	AddLoveNote(ctx context.Context, note repository.LoveNote) error
	RandomNote(ctx context.Context) (repository.LoveNote, error)
}

type LoveNoteService struct {
	mu           sync.Mutex
	rand         *rand.Rand
	store        LoveNoteStore
	defaultNotes []string
}

func NewLoveNoteService(store LoveNoteStore) *LoveNoteService {
	return &LoveNoteService{
		rand:  rand.New(rand.NewSource(time.Now().UnixNano())),
		store: store,
		defaultNotes: []string{
			"Good morning, %s. You make every day feel softer and brighter.",
			"Hey %s, just a reminder: you are deeply loved, today and always.",
			"%s, even ordinary moments feel special because they are with you.",
			"My favorite place is wherever you are, %s.",
			"%s, your smile is still my favorite little miracle.",
			"No matter how busy the day gets, my heart always finds you, %s.",
			"%s, thank you for being the calm, joy, and love in my life.",
		},
	}
}

func (s *LoveNoteService) SeedDefaults(ctx context.Context) error {
	if s.store == nil {
		return nil
	}

	return s.store.AddDefaultNotes(ctx, s.defaultNotes)
}

func (s *LoveNoteService) AddLoveNote(ctx context.Context, input LoveNoteInput) error {
	if s.store == nil {
		return nil
	}

	input.Text = strings.TrimSpace(input.Text)
	input.TelegramFileID = strings.TrimSpace(input.TelegramFileID)
	input.TelegramFileUnique = strings.TrimSpace(input.TelegramFileUnique)
	if input.Text == "" && input.TelegramFileID == "" {
		return ErrLoveNoteContentEmpty
	}

	return s.store.AddLoveNote(ctx, repository.LoveNote{
		Text:               input.Text,
		TelegramFileID:     input.TelegramFileID,
		TelegramFileUnique: input.TelegramFileUnique,
	})
}

func (s *LoveNoteService) RandomNote(ctx context.Context, firstName string) (LoveNote, error) {
	name := strings.TrimSpace(firstName)
	if name == "" {
		name = "my love"
	}

	note := LoveNote{}
	if s.store != nil {
		storedNote, err := s.store.RandomNote(ctx)
		if err != nil {
			return LoveNote{}, err
		}

		note.Text = strings.TrimSpace(storedNote.Text)
		note.TelegramFileID = strings.TrimSpace(storedNote.TelegramFileID)
		note.TelegramFileUnique = strings.TrimSpace(storedNote.TelegramFileUnique)
	}

	if note.Text == "" && note.TelegramFileID == "" {
		note.Text = s.randomDefaultTemplate()
	}

	if note.Text == "" && note.TelegramFileID == "" {
		note.Text = fmt.Sprintf("You are loved so much, %s.", name)
		return note, nil
	}

	note.Text = formatLoveNoteText(note.Text, name)
	return note, nil
}

func (s *LoveNoteService) randomDefaultTemplate() string {
	if len(s.defaultNotes) == 0 {
		return ""
	}

	s.mu.Lock()
	idx := s.rand.Intn(len(s.defaultNotes))
	s.mu.Unlock()

	return s.defaultNotes[idx]
}

func formatLoveNoteText(template string, name string) string {
	if template == "" {
		return ""
	}

	if strings.Contains(template, "%s") {
		return fmt.Sprintf(template, name)
	}

	return template
}
