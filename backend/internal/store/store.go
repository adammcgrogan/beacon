package store

import (
	"mcdash/internal/models"
	"sync"
)

// ServerStore holds all thread-safe data for the application
type ServerStore struct {
	mu          sync.RWMutex
	latestStats models.ServerStats
	logHistory  [][]byte
}

func New() *ServerStore {
	return &ServerStore{
		latestStats: models.ServerStats{TPS: "0.00"},
		logHistory:  make([][]byte, 0),
	}
}

func (s *ServerStore) UpdateStats(stats models.ServerStats) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latestStats = stats
}

func (s *ServerStore) GetStats() models.ServerStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latestStats
}

func (s *ServerStore) AddLog(log []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.logHistory) >= 1000 {
		s.logHistory = s.logHistory[1:] // Keep max 1000 lines
	}
	s.logHistory = append(s.logHistory, log)
}

func (s *ServerStore) GetLogs() [][]byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.logHistory
}

func (s *ServerStore) ClearLogs() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logHistory = make([][]byte, 0)
}
