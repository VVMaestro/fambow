package telegram

import (
	"sync"

	"fambow/internal/service"
)

type memoryWizardStep string

const (
	memoryWizardStepCapture    memoryWizardStep = "capture"
	memoryWizardStepSelectDate memoryWizardStep = "select_date"
	memoryWizardStepAwaitDate  memoryWizardStep = "await_date"
)

type memoryWizardSession struct {
	Step  memoryWizardStep
	Input service.MemoryInput
}

type memoryWizardState struct {
	mx       sync.Mutex
	sessions map[int64]memoryWizardSession
}

func newMemoryWizardState() *memoryWizardState {
	return &memoryWizardState{sessions: make(map[int64]memoryWizardSession)}
}

func (s *memoryWizardState) Start(telegramUserID int64) memoryWizardSession {
	session := memoryWizardSession{Step: memoryWizardStepCapture}
	if s == nil || telegramUserID == 0 {
		return session
	}

	s.mx.Lock()
	s.sessions[telegramUserID] = cloneMemoryWizardSession(session)
	s.mx.Unlock()
	return session
}

func (s *memoryWizardState) Set(telegramUserID int64, session memoryWizardSession) {
	if s == nil || telegramUserID == 0 {
		return
	}

	s.mx.Lock()
	s.sessions[telegramUserID] = cloneMemoryWizardSession(session)
	s.mx.Unlock()
}

func (s *memoryWizardState) Get(telegramUserID int64) (memoryWizardSession, bool) {
	if s == nil || telegramUserID == 0 {
		return memoryWizardSession{}, false
	}

	s.mx.Lock()
	session, ok := s.sessions[telegramUserID]
	s.mx.Unlock()
	return cloneMemoryWizardSession(session), ok
}

func (s *memoryWizardState) Delete(telegramUserID int64) {
	if s == nil || telegramUserID == 0 {
		return
	}

	s.mx.Lock()
	delete(s.sessions, telegramUserID)
	s.mx.Unlock()
}

func cloneMemoryWizardSession(session memoryWizardSession) memoryWizardSession {
	cloned := session
	if session.Input.CreatedAt != nil {
		copied := *session.Input.CreatedAt
		cloned.Input.CreatedAt = &copied
	}
	return cloned
}
