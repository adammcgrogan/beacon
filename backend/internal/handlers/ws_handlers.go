package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/adammcgrogan/beacon/internal/models"
	"github.com/adammcgrogan/beacon/internal/store"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

type WebSocketManager struct {
	Store       *store.ServerStore
	Auth        *AuthManager
	webClients  map[*websocket.Conn]bool
	clientsLock sync.Mutex
	mcConn      *websocket.Conn
	mcConnLock  sync.RWMutex
	mcWriteLock sync.Mutex

	fileReqLock        sync.Mutex
	pendingFileReqByID map[string]chan fileManagerResponse

	permReqLock        sync.Mutex
	pendingPermReqByID map[string]chan playerPermissionsResponse

	permissionAdminReqLock        sync.Mutex
	pendingPermissionAdminReqByID map[string]chan permissionAdminResponse
}

// HandleMinecraft handles the connection from the Java plugin
func (m *WebSocketManager) HandleMinecraft(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	m.setMinecraftConn(conn)
	m.Store.ClearLogs()
	fmt.Println("🟢 Minecraft Server Connected!")
	m.broadcastPluginStatus(true)

	defer func() {
		m.setMinecraftConn(nil)
		m.failAllPendingFileRequests("plugin disconnected")
		m.failAllPendingPermissionRequests()
		m.failAllPendingPermissionAdminRequests()
		m.broadcastPluginStatus(false)
		fmt.Println("🔴 Minecraft Server Disconnected.")
	}()

	for {
		_, messageBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}

		shouldBroadcast := m.processMinecraftMessage(messageBytes)
		if shouldBroadcast {
			m.broadcastToWeb(messageBytes)
		}
	}
}

func (m *WebSocketManager) processMinecraftMessage(messageBytes []byte) bool {
	var envelope struct {
		Event   string          `json:"event"`
		Payload json.RawMessage `json:"payload"`
	}

	if err := json.Unmarshal(messageBytes, &envelope); err != nil {
		return true
	}

	switch envelope.Event {
	case "server_stats":
		var stats models.ServerStats
		if err := json.Unmarshal(envelope.Payload, &stats); err == nil {
			m.Store.UpdateStats(stats)
		}
	case "console_log":
		m.Store.AddLog(messageBytes)
	case "world_stats":
		var worlds []models.WorldInfo
		if err := json.Unmarshal(envelope.Payload, &worlds); err == nil {
			m.Store.UpdateWorlds(worlds)
		}
	case "server_env":
		var env models.ServerEnv
		if err := json.Unmarshal(envelope.Payload, &env); err == nil {
			m.Store.UpdateEnv(env)
		}
	case "plugin_paths":
		if m.Auth != nil {
			var payload struct {
				PluginDataDir string `json:"plugin_data_dir"`
			}
			if err := json.Unmarshal(envelope.Payload, &payload); err == nil {
				m.Auth.SetPluginDataDir(payload.PluginDataDir)
			}
		}
		return false
	case "file_manager_response":
		var response fileManagerResponse
		if err := json.Unmarshal(envelope.Payload, &response); err == nil {
			m.resolvePendingFileRequest(response)
		}
		return false
	case "auth_token_issued":
		if m.Auth != nil {
			var payload struct {
				Token         string   `json:"token"`
				PlayerUUID    string   `json:"player_uuid"`
				PlayerName    string   `json:"player_name"`
				ExpiresAtUnix int64    `json:"expires_at_unix"`
				Permissions   []string `json:"permissions"`
			}
			if err := json.Unmarshal(envelope.Payload, &payload); err == nil {
				m.Auth.StoreMagicToken(payload.Token, payload.PlayerUUID, payload.PlayerName, payload.ExpiresAtUnix, payload.Permissions)
			}
		}
		return false
	case "player_permissions_response":
		var response playerPermissionsResponse
		if err := json.Unmarshal(envelope.Payload, &response); err == nil {
			m.resolvePendingPermissionsRequest(response)
		}
		return false
	case "permission_admin_response":
		var response permissionAdminResponse
		if err := json.Unmarshal(envelope.Payload, &response); err == nil {
			m.resolvePendingPermissionAdminRequest(response)
		}
		return false
	}

	return true
}

// HandleWeb handles browser UI connections
func (m *WebSocketManager) HandleWeb(w http.ResponseWriter, r *http.Request) {
	if m.Auth == nil {
		http.Error(w, "auth unavailable", http.StatusServiceUnavailable)
		return
	}
	session, err := m.Auth.ReadSessionClaims(r)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	m.registerWebClient(conn)
	defer m.unregisterWebClient(conn)
	m.sendPluginStatus(conn)

	// Send latest.log snapshot on connect, then continue with live socket stream.
	if err := m.sendLatestLogSnapshot(conn); err != nil {
		// Fallback to in-memory history if file snapshot is unavailable.
		for _, msg := range m.Store.GetLogs() {
			_ = conn.WriteMessage(websocket.TextMessage, msg)
		}
	}

	for {
		_, messageBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var envelope struct {
			Event string `json:"event"`
		}
		if err := json.Unmarshal(messageBytes, &envelope); err != nil {
			continue
		}

		switch envelope.Event {
		case "plugin_status_request":
			m.sendPluginStatus(conn)
			continue
		case "clear_logs":
			if !m.authorizeSessionEvent(r.Context(), session, envelope.Event, messageBytes) {
				_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"permission_denied","payload":{"reason":"clear_logs"}}`))
				continue
			}
			m.Store.ClearLogs()
			m.broadcastToWeb([]byte(`{"event":"clear_logs"}`))
			continue
		}

		if !m.authorizeSessionEvent(r.Context(), session, envelope.Event, messageBytes) {
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"permission_denied","payload":{"reason":"forbidden"}}`))
			continue
		}

		m.forwardToMinecraft(conn, messageBytes)
	}
}

func (m *WebSocketManager) forwardToMinecraft(webConn *websocket.Conn, raw []byte) {
	if !m.isMinecraftConnected() {
		_ = webConn.WriteMessage(websocket.TextMessage, []byte(`{"event":"command_rejected","payload":{"reason":"plugin_offline"}}`))
		return
	}

	m.mcConnLock.RLock()
	mcConn := m.mcConn
	m.mcConnLock.RUnlock()
	if mcConn == nil {
		_ = webConn.WriteMessage(websocket.TextMessage, []byte(`{"event":"command_rejected","payload":{"reason":"plugin_offline"}}`))
		return
	}

	m.mcWriteLock.Lock()
	err := mcConn.WriteMessage(websocket.TextMessage, raw)
	m.mcWriteLock.Unlock()
	if err != nil {
		_ = webConn.WriteMessage(websocket.TextMessage, []byte(`{"event":"command_rejected","payload":{"reason":"plugin_offline"}}`))
		m.setMinecraftConn(nil)
		m.failAllPendingFileRequests("plugin disconnected")
		m.failAllPendingPermissionRequests()
		m.failAllPendingPermissionAdminRequests()
		m.broadcastPluginStatus(false)
	}
}

func (m *WebSocketManager) registerWebClient(conn *websocket.Conn) {
	m.clientsLock.Lock()
	defer m.clientsLock.Unlock()
	if m.webClients == nil {
		m.webClients = make(map[*websocket.Conn]bool)
	}
	m.webClients[conn] = true
}

func (m *WebSocketManager) unregisterWebClient(conn *websocket.Conn) {
	m.clientsLock.Lock()
	defer m.clientsLock.Unlock()
	delete(m.webClients, conn)
	_ = conn.Close()
}

func (m *WebSocketManager) broadcastToWeb(message []byte) {
	m.clientsLock.Lock()
	defer m.clientsLock.Unlock()
	for client := range m.webClients {
		_ = client.WriteMessage(websocket.TextMessage, message)
	}
}

func (m *WebSocketManager) setMinecraftConn(conn *websocket.Conn) {
	m.mcConnLock.Lock()
	defer m.mcConnLock.Unlock()
	m.mcConn = conn
}

func (m *WebSocketManager) isMinecraftConnected() bool {
	m.mcConnLock.RLock()
	defer m.mcConnLock.RUnlock()
	return m.mcConn != nil
}

func (m *WebSocketManager) sendPluginStatus(conn *websocket.Conn) {
	status := "offline"
	if m.isMinecraftConnected() {
		status = "online"
	}
	_ = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"event":"plugin_status","payload":{"status":"%s"}}`, status)))
}

func (m *WebSocketManager) broadcastPluginStatus(online bool) {
	status := "offline"
	if online {
		status = "online"
	}
	m.broadcastToWeb([]byte(fmt.Sprintf(`{"event":"plugin_status","payload":{"status":"%s"}}`, status)))
}

func (m *WebSocketManager) authorizeSessionEvent(parent context.Context, session SessionClaims, event string, raw []byte) bool {
	if m.Auth == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(parent, 4*time.Second)
	defer cancel()
	permissions, _, err := m.Auth.GetPermissions(ctx, m, session.PlayerUUID)
	if err != nil && err != ErrPluginOffline {
		return false
	}

	switch event {
	case "console_tab_complete":
		return HasPermission(permissions, PermConsoleUse)
	case "console_command":
		var cmdEnvelope struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(raw, &cmdEnvelope); err != nil {
			return false
		}
		command := strings.ToLower(strings.TrimSpace(cmdEnvelope.Command))
		switch {
		case command == "stop" || strings.HasPrefix(command, "stop "):
			return HasPermission(permissions, PermServerStop)
		case command == "restart" || strings.HasPrefix(command, "restart "):
			return HasPermission(permissions, PermServerRestart)
		case command == "save-all" || strings.HasPrefix(command, "save-all "):
			return HasPermission(permissions, PermServerSaveAll)
		case command == "kick" || strings.HasPrefix(command, "kick "):
			return HasPermission(permissions, PermPlayersKick)
		case command == "ban" || strings.HasPrefix(command, "ban "):
			return HasPermission(permissions, PermPlayersBan)
		default:
			return HasPermission(permissions, PermConsoleUse)
		}
	case "world_action":
		var worldEnvelope struct {
			Payload struct {
				Action string `json:"action"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(raw, &worldEnvelope); err != nil {
			return false
		}
		switch worldEnvelope.Payload.Action {
		case "reset":
			return HasPermission(permissions, PermWorldsReset)
		case "set_gamerule":
			return HasPermission(permissions, PermWorldsGamerules)
		default:
			return HasPermission(permissions, PermWorldsManage)
		}
	case "clear_logs":
		return HasPermission(permissions, PermConsoleUse)
	default:
		return false
	}
}
