package service

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

type LoveNoteStore interface {
	AddNotes(ctx context.Context, notes []string) error
	RandomNoteText(ctx context.Context) (string, error)
	AddDefaultNotes(ctx context.Context, notes []string) error
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

func (s *LoveNoteService) AddLoveNote(ctx context.Context, note string) error {
	if s.store == nil {
		return nil
	}

	notes := []string{note}

	return s.store.AddNotes(ctx, notes)
}

func (s *LoveNoteService) RandomNote(ctx context.Context, firstName string) (string, error) {
	name := strings.TrimSpace(firstName)
	if name == "" {
		name = "my love"
	}

	noteTemplate := ""
	if s.store != nil {
		storedText, err := s.store.RandomNoteText(ctx)

		if err != nil {
			return "", err
		}
		noteTemplate = strings.TrimSpace(storedText)
	}

	if noteTemplate == "" {
		noteTemplate = s.randomDefaultTemplate()
	}

	if noteTemplate == "" {
		return fmt.Sprintf("You are loved so much, %s.", name), nil
	}

	return fmt.Sprintf(noteTemplate, name), nil
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
