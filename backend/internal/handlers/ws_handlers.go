package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/adammcgrogan/beacon/internal/models"
	"github.com/adammcgrogan/beacon/internal/store"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

type wsMessage struct {
	Event   string          `json:"event"`
	Command string          `json:"command,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type WebSocketManager struct {
	Store       *store.ServerStore
	webClients  map[*websocket.Conn]bool
	clientsLock sync.Mutex
	mcConn      *websocket.Conn
	mcConnLock  sync.RWMutex
	mcWriteLock sync.Mutex

	fileReqLock        sync.Mutex
	pendingFileReqByID map[string]chan fileManagerResponse
	Store      *store.ServerStore
	webClients sync.Map

	mcConn     *websocket.Conn
	mcConnLock sync.RWMutex
}

// ==========================================
// MINECRAFT PLUGIN WEBSOCKET
// ==========================================

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

	// Clean up when connection drops
	defer func() {
		m.setMinecraftConn(nil)
		m.broadcastPluginStatus(false)
		fmt.Println("🔴 Minecraft Server Disconnected.")
	}()

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}

		shouldBroadcast := m.processMinecraftMessage(messageBytes)
		if shouldBroadcast {
			m.broadcastToWeb(messageBytes)
		}
	}
	m.setMinecraftConn(nil)
	m.failAllPendingFileRequests("plugin disconnected")
	m.broadcastPluginStatus(false)
}

func (m *WebSocketManager) processMinecraftMessage(messageBytes []byte) bool {
	var envelope struct {
		Event   string          `json:"event"`
		Payload json.RawMessage `json:"payload"`
	}

	if err := json.Unmarshal(messageBytes, &envelope); err != nil {
		return true
		m.processMinecraftMessage(msgBytes)
		m.broadcastToWeb(msgBytes)
	}
}

func (m *WebSocketManager) processMinecraftMessage(msgBytes []byte) {
	var msg wsMessage
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		return
	}

	switch msg.Event {
	case "server_stats":
		var stats models.ServerStats
		if json.Unmarshal(msg.Payload, &stats) == nil {
			m.Store.UpdateStats(stats)
		}
	case "world_stats":
		var worlds []models.WorldInfo
		if json.Unmarshal(msg.Payload, &worlds) == nil {
			m.Store.UpdateWorlds(worlds)
		}
	case "server_env":
		var env models.ServerEnv
		if json.Unmarshal(msg.Payload, &env) == nil {
			m.Store.UpdateEnv(env)
		}
	case "file_manager_response":
		var response fileManagerResponse
		if err := json.Unmarshal(envelope.Payload, &response); err == nil {
			m.resolvePendingFileRequest(response)
		}
		return false
	case "console_log":
		m.Store.AddLog(msgBytes)
	}

	return true
}

// ==========================================
// WEB DASHBOARD WEBSOCKET
// ==========================================

func (m *WebSocketManager) HandleWeb(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	m.webClients.Store(conn, true)
	defer func() {
		m.webClients.Delete(conn)
		conn.Close()
	}()

	m.sendPluginStatus(conn)

	// Send latest.log snapshot on connect, then continue with live socket stream.
	if err := m.sendLatestLogSnapshot(conn); err != nil {
		// Fallback to in-memory history if file snapshot is unavailable.
		for _, msg := range m.Store.GetLogs() {
			conn.WriteMessage(websocket.TextMessage, msg)
		}
	// Send historical logs on connect
	for _, logMsg := range m.Store.GetLogs() {
		conn.WriteMessage(websocket.TextMessage, logMsg)
	}

	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var msg wsMessage
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			continue
		}

		switch msg.Event {
		case "plugin_status_request":
			m.sendPluginStatus(conn)

		case "clear_logs":
			m.Store.ClearLogs()
			m.broadcastToWeb([]byte(`{"event":"clear_logs"}`))

		m.mcWriteLock.Lock()
		err = mcConn.WriteMessage(websocket.TextMessage, messageBytes)
		m.mcWriteLock.Unlock()
		if err != nil {
			conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"command_rejected","payload":{"reason":"plugin_offline"}}`))
			m.setMinecraftConn(nil)
			m.broadcastPluginStatus(false)
		case "console_command":
			m.forwardCommandToMinecraft(conn, msg.Command, msgBytes)
		}
	}
}

func (m *WebSocketManager) forwardCommandToMinecraft(webConn *websocket.Conn, command string, rawBytes []byte) {
	m.mcConnLock.RLock()
	mcConn := m.mcConn
	m.mcConnLock.RUnlock()

	// Reject if Minecraft plugin isn't connected
	if mcConn == nil {
		webConn.WriteMessage(websocket.TextMessage, []byte(`{"event":"command_rejected","payload":{"reason":"plugin_offline"}}`))
		return
	}

	// Echo the command back to the web console visually
	cmdJson, _ := json.Marshal("> " + command)
	echoMsg := []byte(fmt.Sprintf(`{"event":"console_log","payload":{"message":%s,"level":"INFO"}}`, string(cmdJson)))
	m.Store.AddLog(echoMsg)
	m.broadcastToWeb(echoMsg)

	// Send actual command to Minecraft
	if err := mcConn.WriteMessage(websocket.TextMessage, rawBytes); err != nil {
		webConn.WriteMessage(websocket.TextMessage, []byte(`{"event":"command_rejected","payload":{"reason":"plugin_offline"}}`))
		m.setMinecraftConn(nil)
		m.broadcastPluginStatus(false)
	}
}

// ==========================================
// UTILITIES
// ==========================================

func (m *WebSocketManager) broadcastToWeb(message []byte) {
	m.webClients.Range(func(key, value interface{}) bool {
		client := key.(*websocket.Conn)
		client.WriteMessage(websocket.TextMessage, message)
		return true // Continue iterating
	})
}

func (m *WebSocketManager) setMinecraftConn(conn *websocket.Conn) {
	m.mcConnLock.Lock()
	defer m.mcConnLock.Unlock()
	m.mcConn = conn
}

func (m *WebSocketManager) sendPluginStatus(conn *websocket.Conn) {
	status := "offline"

	m.mcConnLock.RLock()
	if m.mcConn != nil {
		status = "online"
	}
	m.mcConnLock.RUnlock()

	conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"event":"plugin_status","payload":{"status":"%s"}}`, status)))
}

func (m *WebSocketManager) broadcastPluginStatus(online bool) {
	status := "offline"
	if online {
		status = "online"
	}
	m.broadcastToWeb([]byte(fmt.Sprintf(`{"event":"plugin_status","payload":{"status":"%s"}}`, status)))
}
