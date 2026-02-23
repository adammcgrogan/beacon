package store

import (
	"sync"

	"github.com/adammcgrogan/beacon/internal/models"
)

const maxLogLines = 1000

// ServerStore holds all thread-safe data for the application
type ServerStore struct {
	mu sync.RWMutex

	latestStats models.ServerStats
	env         models.ServerEnv
	worlds      []models.WorldInfo
	logHistory  [][]byte
}

func New() *ServerStore {
	return &ServerStore{
		latestStats: models.ServerStats{TPS: "0.00"},
		env:         models.ServerEnv{Software: "Awaiting Data...", Java: "Awaiting Data...", OS: "Awaiting Data..."},
		worlds:      make([]models.WorldInfo, 0),
		logHistory:  make([][]byte, 0),
	}
}

// --- Server Stats ---

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

// --- Server Environment ---

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

// --- Worlds ---

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

// --- Console Logs ---

func (s *ServerStore) AddLog(log []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.logHistory) >= maxLogLines {
		s.logHistory = s.logHistory[1:]
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
