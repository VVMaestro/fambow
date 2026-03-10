package telegram

import "sync"

type memoryIntakeState struct {
	mx      sync.Mutex
	pending map[int64]struct{}
}

func newMemoryIntakeState() *memoryIntakeState {
	return &memoryIntakeState{pending: make(map[int64]struct{})}
}

func (s *memoryIntakeState) Arm(telegramUserID int64) {
	if s == nil || telegramUserID == 0 {
		return
	}

	s.mx.Lock()
	s.pending[telegramUserID] = struct{}{}
	s.mx.Unlock()
}

func (s *memoryIntakeState) Disarm(telegramUserID int64) {
	if s == nil || telegramUserID == 0 {
		return
	}

	s.mx.Lock()
	delete(s.pending, telegramUserID)
	s.mx.Unlock()
}

func (s *memoryIntakeState) IsArmed(telegramUserID int64) bool {
	if s == nil || telegramUserID == 0 {
		return false
	}

	s.mx.Lock()
	_, ok := s.pending[telegramUserID]
	s.mx.Unlock()
	return ok
}
