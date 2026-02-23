package store

import (
	"sync"

	"github.com/adammcgrogan/beacon/internal/models"
)

// ServerStore holds all thread-safe data for the application
type ServerStore struct {
	mu          sync.RWMutex
	latestStats models.ServerStats
	logHistory  [][]byte
	worlds      []models.WorldInfo
	env         models.ServerEnv
}

func New() *ServerStore {
	return &ServerStore{
		latestStats: models.ServerStats{TPS: "0.00"},
		logHistory:  make([][]byte, 0),
		worlds:      make([]models.WorldInfo, 0),
		env:         models.ServerEnv{Software: "Awaiting Data...", Java: "Awaiting Data...", OS: "Awaiting Data..."},
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

func (s *ServerStore) UpdateWorlds(w []models.WorldInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.worlds = w
}

func (s *ServerStore) GetWorlds() []models.WorldInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.worlds
}

func (s *ServerStore) UpdateEnv(e models.ServerEnv) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.env = e
}

func (s *ServerStore) GetEnv() models.ServerEnv {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.env
}
