package telegram

import "sync"

type reminderWizardStep string

const (
	reminderWizardStepTarget    reminderWizardStep = "target"
	reminderWizardStepSchedule  reminderWizardStep = "schedule"
	reminderWizardStepAwaitTime reminderWizardStep = "await_time"
	reminderWizardStepText      reminderWizardStep = "text"
)

type reminderWizardSession struct {
	Step           reminderWizardStep
	TargetUserType string
	TargetLabel    string
	ScheduleType   string
	TimeValue      string
}

type reminderWizardState struct {
	mx       sync.Mutex
	sessions map[int64]reminderWizardSession
}

func newReminderWizardState() *reminderWizardState {
	return &reminderWizardState{sessions: make(map[int64]reminderWizardSession)}
}

func (s *reminderWizardState) Start(telegramUserID int64) reminderWizardSession {
	if s == nil || telegramUserID == 0 {
		return reminderWizardSession{}
	}

	s.mx.Lock()
	s.sessions[telegramUserID] = reminderWizardSession{Step: reminderWizardStepTarget}
	s.mx.Unlock()
	return reminderWizardSession{Step: reminderWizardStepTarget}
}

func (s *reminderWizardState) Set(telegramUserID int64, session reminderWizardSession) {
	if s == nil || telegramUserID == 0 {
		return
	}

	s.mx.Lock()
	s.sessions[telegramUserID] = session
	s.mx.Unlock()
}

func (s *reminderWizardState) Get(telegramUserID int64) (reminderWizardSession, bool) {
	if s == nil || telegramUserID == 0 {
		return reminderWizardSession{}, false
	}

	s.mx.Lock()
	session, ok := s.sessions[telegramUserID]
	s.mx.Unlock()
	return session, ok
}

func (s *reminderWizardState) Delete(telegramUserID int64) {
	if s == nil || telegramUserID == 0 {
		return
	}

	s.mx.Lock()
	delete(s.sessions, telegramUserID)
	s.mx.Unlock()
}
