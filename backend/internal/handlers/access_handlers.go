package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"time"
)

type accessPermissionCategory struct {
	ID          string                     `json:"id"`
	Label       string                     `json:"label"`
	Permissions []accessPermissionCheckbox `json:"permissions"`
}

type accessPermissionCheckbox struct {
	Node  string `json:"node"`
	Label string `json:"label"`
}

func panelPermissionCategories() []accessPermissionCategory {
	return []accessPermissionCategory{
		{
			ID:    "global",
			Label: "Global",
			Permissions: []accessPermissionCheckbox{
				{Node: "beacon.access.*", Label: "Full Access (All Pages)"},
				{Node: "beacon.access.access", Label: "Access Page"},
				{Node: "beacon.access.access.manage", Label: "Manage Access (Sessions + Permissions)"},
			},
		},
		{
			ID:    "dashboard",
			Label: "Dashboard",
			Permissions: []accessPermissionCheckbox{
				{Node: "beacon.access.dashboard", Label: "Dashboard Pack"},
				{Node: "beacon.access.dashboard.view", Label: "View Dashboard"},
				{Node: "beacon.access.stop", Label: "Stop Server"},
				{Node: "beacon.access.restart", Label: "Restart Server"},
				{Node: "beacon.access.saveall", Label: "Save All"},
			},
		},
		{
			ID:    "console",
			Label: "Console",
			Permissions: []accessPermissionCheckbox{
				{Node: "beacon.access.console", Label: "Console Pack"},
				{Node: "beacon.access.console.view", Label: "View Console"},
				{Node: "beacon.access.console.use", Label: "Use Console"},
			},
		},
		{
			ID:    "players",
			Label: "Players",
			Permissions: []accessPermissionCheckbox{
				{Node: "beacon.access.players", Label: "Players Pack"},
				{Node: "beacon.access.players.view", Label: "View Players"},
				{Node: "beacon.access.players.kick", Label: "Kick Players"},
				{Node: "beacon.access.players.ban", Label: "Ban Players"},
			},
		},
		{
			ID:    "worlds",
			Label: "Worlds",
			Permissions: []accessPermissionCheckbox{
				{Node: "beacon.access.worlds", Label: "Worlds Pack"},
				{Node: "beacon.access.worlds.view", Label: "View Worlds"},
				{Node: "beacon.access.worlds.manage", Label: "Manage Worlds"},
				{Node: "beacon.access.worlds.reset", Label: "Reset Worlds"},
				{Node: "beacon.access.worlds.gamerules", Label: "Edit Gamerules"},
			},
		},
		{
			ID:    "files",
			Label: "Files",
			Permissions: []accessPermissionCheckbox{
				{Node: "beacon.access.files", Label: "Files Pack"},
				{Node: "beacon.access.files.all", Label: "All Files (Bypass Path Scope)"},
				{Node: "beacon.access.files.view", Label: "View Files"},
				{Node: "beacon.access.files.edit", Label: "Edit Files"},
				{Node: "beacon.access.files.delete", Label: "Delete Files"},
				{Node: "beacon.access.files.download", Label: "Download Files"},
			},
		},
	}
}

func flatPermissionNodes(categories []accessPermissionCategory) []string {
	out := make([]string, 0)
	seen := make(map[string]struct{})
	for _, category := range categories {
		for _, perm := range category.Permissions {
			if _, ok := seen[perm.Node]; ok {
				continue
			}
			seen[perm.Node] = struct{}{}
			out = append(out, perm.Node)
		}
	}
	slices.Sort(out)
	return out
}

func (h *UIHandler) HandleAccess(w http.ResponseWriter, r *http.Request) {
	claims, permissions, ok := h.requirePagePermission(w, r, PermAccessView)
	if !ok {
		return
	}
	h.render(w, "access", "Access Management", map[string]interface{}{}, claims, DeriveSessionGrants(permissions))
}

func (h *UIHandler) HandleAccessData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	_, permissions, ok := h.requireAuthForAPI(w, r)
	if !ok {
		return
	}
	if !HasAnyPermission(permissions, PermAccessAll, PermAccessView, PermAccessManage) {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}

	categories := panelPermissionCategories()
	nodes := flatPermissionNodes(categories)
	users, sessions := h.Auth.ListKnownUsersWithSessions()
	slices.SortFunc(users, func(a, b knownUser) int {
		return b.LastSeen.Compare(a.LastSeen)
	})

	sessionByUser := make(map[string][]webSession)
	for _, s := range sessions {
		sessionByUser[s.PlayerUUID] = append(sessionByUser[s.PlayerUUID], s)
	}
	for key := range sessionByUser {
		slices.SortFunc(sessionByUser[key], func(a, b webSession) int {
			return b.LastSeenAt.Compare(a.LastSeenAt)
		})
	}

	outputUsers := make([]map[string]any, 0, len(users))
	for _, user := range users {
		snapshot := map[string]bool{}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		if h.WS != nil {
			if values, err := h.WS.RequestPermissionSnapshot(ctx, user.PlayerUUID, user.PlayerName, nodes); err == nil {
				snapshot = values
			}
		}
		cancel()

		effective := make([]string, 0)
		for _, node := range nodes {
			if snapshot[node] {
				effective = append(effective, node)
			}
		}

		userSessions := make([]map[string]any, 0)
		for _, s := range sessionByUser[user.PlayerUUID] {
			userSessions = append(userSessions, map[string]any{
				"id":         s.ID,
				"created_at": s.CreatedAt.Unix(),
				"last_seen":  s.LastSeenAt.Unix(),
				"expires_at": s.ExpiresAt.Unix(),
				"revoked":    s.Revoked,
			})
		}

		outputUsers = append(outputUsers, map[string]any{
			"player_uuid": user.PlayerUUID,
			"player_name": user.PlayerName,
			"first_seen":  user.FirstSeen.Unix(),
			"last_seen":   user.LastSeen.Unix(),
			"sessions":    userSessions,
			"permissions": snapshot,
			"grants":      DeriveSessionGrants(effective),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"categories": categories,
		"users":      outputUsers,
	})
}

func (h *UIHandler) HandleAccessSessionDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		methodNotAllowed(w)
		return
	}
	_, permissions, ok := h.requireAuthForAPI(w, r)
	if !ok {
		return
	}
	if !HasAnyPermission(permissions, PermAccessAll, PermAccessManage) {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing session_id")
		return
	}

	if !h.Auth.RevokeSession(sessionID) {
		writeJSONError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *UIHandler) HandleAccessPermissionUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}
	_, permissions, ok := h.requireAuthForAPI(w, r)
	if !ok {
		return
	}
	if !HasAnyPermission(permissions, PermAccessAll, PermAccessManage) {
		writeJSONError(w, http.StatusForbidden, "forbidden")
		return
	}

	var req struct {
		PlayerUUID string `json:"player_uuid"`
		PlayerName string `json:"player_name"`
		Node       string `json:"node"`
		Enabled    bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.PlayerUUID == "" || req.Node == "" {
		writeJSONError(w, http.StatusBadRequest, "player_uuid and node are required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := h.WS.RequestPermissionSet(ctx, req.PlayerUUID, req.PlayerName, req.Node, req.Enabled); err != nil {
		writeJSONError(w, http.StatusBadGateway, err.Error())
		return
	}

	h.Auth.InvalidatePermissionCache(req.PlayerUUID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
