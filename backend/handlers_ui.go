package main

import (
	"net/http"
)

func (s *DashboardServer) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	s.statsLock.Lock()
	currentStats := s.latestStats
	s.statsLock.Unlock()

	data := map[string]interface{}{
		"Title":     "Overview",
		"ActiveTab": "dashboard",
		"Status":    "Online",
		"Stats":     currentStats,
	}
	tmpl.ExecuteTemplate(w, "base", data)
}

func (s *DashboardServer) HandleConsole(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{"Title": "Live Console", "ActiveTab": "console"}
	tmpl.ExecuteTemplate(w, "base", data)
}

func (s *DashboardServer) HandlePlayers(w http.ResponseWriter, r *http.Request) {
	s.statsLock.Lock()
	currentStats := s.latestStats
	s.statsLock.Unlock()

	data := map[string]interface{}{
		"Title":     "Player List",
		"ActiveTab": "players",
		"Stats":     currentStats,
	}
	tmpl.ExecuteTemplate(w, "base", data)
}
