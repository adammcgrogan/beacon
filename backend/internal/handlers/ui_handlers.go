package handlers

import (
	"html/template"
	"net/http"

	"github.com/adammcgrogan/beacon/internal/store"
)

type UIHandler struct {
	Store *store.ServerStore
	WS    *WebSocketManager
	Tmpl  *template.Template
}

func NewUIHandler(s *store.ServerStore, ws *WebSocketManager, tmpl *template.Template) *UIHandler {
	return &UIHandler{
		Store: s,
		WS:    ws,
		Tmpl:  tmpl,
	}
}

func (h *UIHandler) render(w http.ResponseWriter, tabName, title string, extraData map[string]interface{}) {
	data := map[string]interface{}{
		"Title":     title,
		"ActiveTab": tabName,
	}

	for k, v := range extraData {
		data[k] = v
	}

	h.Tmpl.ExecuteTemplate(w, "base", data)
}

func (h *UIHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	h.render(w, "dashboard", "Overview", map[string]interface{}{
		"Stats": h.Store.GetStats(),
		"Env":   h.Store.GetEnv(),
	})
}

func (h *UIHandler) HandleConsole(w http.ResponseWriter, r *http.Request) {
	h.render(w, "console", "Live Console", nil)
}

func (h *UIHandler) HandlePlayers(w http.ResponseWriter, r *http.Request) {
	h.render(w, "players", "Player List", map[string]interface{}{
		"Stats": h.Store.GetStats(),
	})
}

func (h *UIHandler) HandleWorlds(w http.ResponseWriter, r *http.Request) {
	h.render(w, "worlds", "World Manager", map[string]interface{}{
		"Worlds": h.Store.GetWorlds(),
	})
}

func (h *UIHandler) HandleFiles(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":     "File Manager",
		"ActiveTab": "files",
	}
	h.Tmpl.ExecuteTemplate(w, "base", data)
}
