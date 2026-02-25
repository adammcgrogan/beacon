package handlers

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/adammcgrogan/beacon/internal/store"
)

type UIHandler struct {
	Store *store.ServerStore
	WS    *WebSocketManager
	Tmpl  *template.Template
}

type contextKey string

const sessionContextKey contextKey = "session_claims"

func NewUIHandler(s *store.ServerStore, ws *WebSocketManager, tmpl *template.Template, auth *AuthManager) *UIHandler {
	return &UIHandler{
		Store: s,
		WS:    ws,
    Tmpl: tmpl,
		Auth:  auth,
	}
}

func (h *UIHandler) render(w http.ResponseWriter, tabName, title string, extraData map[string]interface{}, claims SessionClaims, grants SessionGrants) {
	data := map[string]interface{}{
		"Title":     title,
		"ActiveTab": tabName,
		"Session":   claims,
		"Grants":    grants,
	}

	for k, v := range extraData {
		data[k] = v
	}

	h.Tmpl.ExecuteTemplate(w, "base", data)
}

func (h *UIHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	claims, permissions, ok := h.requirePagePermission(w, r, PermDashboardView)
	if !ok {
		return
	}
	h.render(w, "dashboard", "Overview", map[string]interface{}{
		"Stats": h.Store.GetStats(),
		"Env":   h.Store.GetEnv(),
	}, claims, DeriveSessionGrants(permissions))
}

func (h *UIHandler) HandleConsole(w http.ResponseWriter, r *http.Request) {
	claims, permissions, ok := h.requirePagePermission(w, r, PermConsoleView)
	if !ok {
		return
	}
	h.render(w, "console", "Live Console", nil, claims, DeriveSessionGrants(permissions))
}

func (h *UIHandler) HandlePlayers(w http.ResponseWriter, r *http.Request) {
	claims, permissions, ok := h.requirePagePermission(w, r, PermPlayersView)
	if !ok {
		return
	}
	h.render(w, "players", "Player List", map[string]interface{}{
		"Stats": h.Store.GetStats(),
	}, claims, DeriveSessionGrants(permissions))
}

func (h *UIHandler) HandleWorlds(w http.ResponseWriter, r *http.Request) {
	claims, permissions, ok := h.requirePagePermission(w, r, PermWorldsView)
	if !ok {
		return
	}
	h.render(w, "worlds", "World Manager", map[string]interface{}{
		"Worlds": h.Store.GetWorlds(),
	}, claims, DeriveSessionGrants(permissions))
}

func (h *UIHandler) HandleFiles(w http.ResponseWriter, r *http.Request) {
	claims, permissions, ok := h.requirePageFileViewPermission(w, r)
	if !ok {
		return
	}
	data := map[string]interface{}{
		"Title":     "File Manager",
		"ActiveTab": "files",
	}
	for k, v := range map[string]interface{}{
		"Session": claims,
		"Grants":  DeriveSessionGrants(permissions),
	} {
		data[k] = v
	}
	tmpl.ExecuteTemplate(w, "base", data)
}

func (h *UIHandler) HandleAuthPage(w http.ResponseWriter, r *http.Request) {
	if h.Auth != nil {
		if _, err := h.Auth.ReadSessionClaims(r); err == nil {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
	}
	_ = tmpl.ExecuteTemplate(w, "auth", map[string]interface{}{})
}

func (h *UIHandler) HandleMagicLinkAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if h.Auth == nil {
		writeJSONError(w, http.StatusInternalServerError, "auth not configured")
		return
	}

	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	claims, err := h.Auth.ConsumeMagicToken(strings.TrimSpace(req.Token))
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	signedToken, err := h.Auth.EncodeSession(claims)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "could not create session")
		return
	}

	h.Auth.SetSessionCookie(w, signedToken, claims.ExpiresAt)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
	})
}

func (h *UIHandler) HandleSession(w http.ResponseWriter, r *http.Request) {
	claims, _, ok := h.requireAuthForAPI(w, r)
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()

	permissions, _, err := h.Auth.GetPermissions(ctx, h.WS, claims.PlayerUUID)
	if err != nil && err != ErrPluginOffline {
		writeJSONError(w, http.StatusServiceUnavailable, "could not refresh permissions")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"player_uuid": claims.PlayerUUID,
		"player_name": claims.PlayerName,
		"permissions": permissions,
		"grants":      DeriveSessionGrants(permissions),
		"expires_at":  claims.ExpiresAt,
	})
}

func (h *UIHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	if h.Auth != nil {
		h.Auth.ClearSessionCookie(w)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *UIHandler) RequirePageAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := h.Auth.ReadSessionClaims(r)
		if err != nil {
			http.Redirect(w, r, "/auth", http.StatusFound)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), sessionContextKey, claims)))
	}
}

func (h *UIHandler) RequireAPIAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, err := h.Auth.ReadSessionClaims(r)
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), sessionContextKey, claims)))
	}
}

func (h *UIHandler) requireAuthForAPI(w http.ResponseWriter, r *http.Request) (SessionClaims, []string, bool) {
	claims := h.sessionFromContext(r)
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	permissions, _, err := h.Auth.GetPermissions(ctx, h.WS, claims.PlayerUUID)
	if err != nil && err != ErrPluginOffline {
		writeJSONError(w, http.StatusServiceUnavailable, "could not load permissions")
		return SessionClaims{}, nil, false
	}
	return claims, permissions, true
}

func (h *UIHandler) requirePagePermission(w http.ResponseWriter, r *http.Request, permission string) (SessionClaims, []string, bool) {
	claims := h.sessionFromContext(r)
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()

	permissions, _, err := h.Auth.GetPermissions(ctx, h.WS, claims.PlayerUUID)
	if err != nil && err != ErrPluginOffline {
		http.Error(w, "permissions unavailable", http.StatusServiceUnavailable)
		return SessionClaims{}, nil, false
	}
	if !HasPermission(permissions, permission) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return SessionClaims{}, nil, false
	}
	return claims, permissions, true
}

func (h *UIHandler) requirePageFileViewPermission(w http.ResponseWriter, r *http.Request) (SessionClaims, []string, bool) {
	claims := h.sessionFromContext(r)
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()

	permissions, _, err := h.Auth.GetPermissions(ctx, h.WS, claims.PlayerUUID)
	if err != nil && err != ErrPluginOffline {
		http.Error(w, "permissions unavailable", http.StatusServiceUnavailable)
		return SessionClaims{}, nil, false
	}
	if !CanAccessAnyFileView(permissions) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return SessionClaims{}, nil, false
	}
	return claims, permissions, true
}

func (h *UIHandler) sessionFromContext(r *http.Request) SessionClaims {
	claims, _ := r.Context().Value(sessionContextKey).(SessionClaims)
	return claims
}

func (h *UIHandler) HandleGameruleDefaults(w http.ResponseWriter, r *http.Request) {
	defaults := h.Store.GetStats().DefaultGamerules

	if defaults == nil {
		defaults = make(map[string]string)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(defaults)
}
