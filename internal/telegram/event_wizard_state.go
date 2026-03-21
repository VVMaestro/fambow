package telegram

import "sync"

type eventWizardStep string

const (
	eventWizardStepSelectDate eventWizardStep = "select_date"
	eventWizardStepAwaitDate  eventWizardStep = "await_date"
	eventWizardStepAwaitTitle eventWizardStep = "await_title"
	eventWizardStepSelectDays eventWizardStep = "select_days"
	eventWizardStepAwaitDays  eventWizardStep = "await_days"
)

type eventWizardSession struct {
	Step             eventWizardStep
	EventDate        string
	Title            string
	RemindDaysBefore int
}

type eventWizardState struct {
	mx       sync.Mutex
	sessions map[int64]eventWizardSession
}

func newEventWizardState() *eventWizardState {
	return &eventWizardState{sessions: make(map[int64]eventWizardSession)}
}

func (s *eventWizardState) Start(telegramUserID int64) eventWizardSession {
	session := eventWizardSession{Step: eventWizardStepSelectDate}
	if s == nil || telegramUserID == 0 {
		return session
	}

	s.mx.Lock()
	s.sessions[telegramUserID] = session
	s.mx.Unlock()
	return session
}

func (s *eventWizardState) Set(telegramUserID int64, session eventWizardSession) {
	if s == nil || telegramUserID == 0 {
		return
	}

	s.mx.Lock()
	s.sessions[telegramUserID] = session
	s.mx.Unlock()
}

func (s *eventWizardState) Get(telegramUserID int64) (eventWizardSession, bool) {
	if s == nil || telegramUserID == 0 {
		return eventWizardSession{}, false
	}

	s.mx.Lock()
	session, ok := s.sessions[telegramUserID]
	s.mx.Unlock()
	return session, ok
}

func (s *eventWizardState) Delete(telegramUserID int64) {
	if s == nil || telegramUserID == 0 {
		return
	}

	s.mx.Lock()
	delete(s.sessions, telegramUserID)
	s.mx.Unlock()
}
