package handlers

import (
	"html/template"
	"net/http"

	"github.com/adammcgrogan/beacon/internal/store"
)

// Note: Ensure the path to templates points to the correct relative or absolute directory
var tmpl = template.Must(template.ParseGlob("../../templates/*.html"))

type UIHandler struct {
	Store *store.ServerStore
}

func NewUIHandler(s *store.ServerStore) *UIHandler {
	return &UIHandler{Store: s}
}

func (h *UIHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	// We no longer need s.statsLock.Lock() here! The Store handles it.
	data := map[string]interface{}{
		"Title":     "Overview",
		"ActiveTab": "dashboard",
		"Status":    "Online",
		"Stats":     h.Store.GetStats(), 
	}
	tmpl.ExecuteTemplate(w, "base", data)
}

func (h *UIHandler) HandleConsole(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":     "Live Console", 
		"ActiveTab": "console",
	}
	tmpl.ExecuteTemplate(w, "base", data)
}

func (h *UIHandler) HandlePlayers(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":     "Player List",
		"ActiveTab": "players",
		"Stats":     h.Store.GetStats(),
	}
	tmpl.ExecuteTemplate(w, "base", data)
}