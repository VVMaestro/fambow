package telegram

import "sync"

type loveScheduleWizardStep string

const (
	loveScheduleWizardStepSelectUser loveScheduleWizardStep = "select_user"
	loveScheduleWizardStepAwaitTime  loveScheduleWizardStep = "await_time"
)

type loveScheduleWizardSession struct {
	Step             loveScheduleWizardStep
	TargetTelegramID int64
	TargetFirstName  string
	TargetUserType   string
}

type loveScheduleWizardState struct {
	mx       sync.Mutex
	sessions map[int64]loveScheduleWizardSession
}

func newLoveScheduleWizardState() *loveScheduleWizardState {
	return &loveScheduleWizardState{sessions: make(map[int64]loveScheduleWizardSession)}
}

func (s *loveScheduleWizardState) Start(telegramUserID int64) loveScheduleWizardSession {
	session := loveScheduleWizardSession{Step: loveScheduleWizardStepSelectUser}
	if s == nil || telegramUserID == 0 {
		return session
	}

	s.mx.Lock()
	s.sessions[telegramUserID] = session
	s.mx.Unlock()
	return session
}

func (s *loveScheduleWizardState) Set(telegramUserID int64, session loveScheduleWizardSession) {
	if s == nil || telegramUserID == 0 {
		return
	}

	s.mx.Lock()
	s.sessions[telegramUserID] = session
	s.mx.Unlock()
}

func (s *loveScheduleWizardState) Get(telegramUserID int64) (loveScheduleWizardSession, bool) {
	if s == nil || telegramUserID == 0 {
		return loveScheduleWizardSession{}, false
	}

	s.mx.Lock()
	session, ok := s.sessions[telegramUserID]
	s.mx.Unlock()
	return session, ok
}

func (s *loveScheduleWizardState) Delete(telegramUserID int64) {
	if s == nil || telegramUserID == 0 {
		return
	}

	s.mx.Lock()
	delete(s.sessions, telegramUserID)
	s.mx.Unlock()
}
