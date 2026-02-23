package store

import (
	"sync"
	"time"

	"github.com/adammcgrogan/beacon/internal/models"
)

const (
	MaxLogLines = 1000
	MaxLogBytes = 5 * 1024 * 1024 // 5MB limit for total memory buffer
	MaxLogAge   = 1 * time.Hour   // Clear logs if the server has been silent this long
)

// ServerStore holds all thread-safe data for the application
type ServerStore struct {
	mu sync.RWMutex

	latestStats models.ServerStats
	env         models.ServerEnv
	worlds      []models.WorldInfo
	logHistory  [][]byte

	totalBytes  int
	lastLogTime time.Time
}

func New() *ServerStore {
	return &ServerStore{
		latestStats: models.ServerStats{TPS: "0.00"},
		env:         models.ServerEnv{Software: "Awaiting Data...", Java: "Awaiting Data...", OS: "Awaiting Data..."},
		worlds:      make([]models.WorldInfo, 0),
		logHistory:  make([][]byte, 0),
		lastLogTime: time.Now(),
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

	if time.Since(s.lastLogTime) > MaxLogAge && len(s.logHistory) > 0 {
		s.logHistory = make([][]byte, 0)
		s.totalBytes = 0
	}

	if len(s.logHistory) >= MaxLogLines {
		s.totalBytes -= len(s.logHistory[0])
		s.logHistory = s.logHistory[1:]
	}

	s.totalBytes += len(log)
	for s.totalBytes > MaxLogBytes && len(s.logHistory) > 0 {
		s.totalBytes -= len(s.logHistory[0])
		s.logHistory = s.logHistory[1:]
	}

	s.logHistory = append(s.logHistory, log)
	s.lastLogTime = time.Now()
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
