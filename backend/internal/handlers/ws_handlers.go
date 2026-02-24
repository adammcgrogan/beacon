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

type WebSocketManager struct {
	Store       *store.ServerStore
	webClients  map[*websocket.Conn]bool
	clientsLock sync.Mutex
	mcConn      *websocket.Conn
	mcConnLock  sync.RWMutex
	mcWriteLock sync.Mutex

	fileReqLock        sync.Mutex
	pendingFileReqByID map[string]chan fileManagerResponse
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
	case "file_manager_response":
		var response fileManagerResponse
		if err := json.Unmarshal(envelope.Payload, &response); err == nil {
			m.resolvePendingFileRequest(response)
		}
		return false
	}

	return true
}

// HandleWeb handles browser UI connections
func (m *WebSocketManager) HandleWeb(w http.ResponseWriter, r *http.Request) {
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
			conn.WriteMessage(websocket.TextMessage, msg)
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
		if err := json.Unmarshal(messageBytes, &envelope); err == nil && envelope.Event == "plugin_status_request" {
			m.sendPluginStatus(conn)
			continue
		}

		// Pass commands from web directly to Minecraft
		if !m.isMinecraftConnected() {
			conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"command_rejected","payload":{"reason":"plugin_offline"}}`))
			continue
		}

		m.mcConnLock.RLock()
		mcConn := m.mcConn
		m.mcConnLock.RUnlock()
		if mcConn == nil {
			conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"command_rejected","payload":{"reason":"plugin_offline"}}`))
			continue
		}

		m.mcWriteLock.Lock()
		err = mcConn.WriteMessage(websocket.TextMessage, messageBytes)
		m.mcWriteLock.Unlock()
		if err != nil {
			conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"command_rejected","payload":{"reason":"plugin_offline"}}`))
			m.setMinecraftConn(nil)
			m.broadcastPluginStatus(false)
		}
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
	conn.Close()
}

func (m *WebSocketManager) broadcastToWeb(message []byte) {
	m.clientsLock.Lock()
	defer m.clientsLock.Unlock()
	for client := range m.webClients {
		client.WriteMessage(websocket.TextMessage, message)
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
	conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"event":"plugin_status","payload":{"status":"%s"}}`, status)))
}

func (m *WebSocketManager) broadcastPluginStatus(online bool) {
	status := "offline"
	if online {
		status = "online"
	}
	m.broadcastToWeb([]byte(fmt.Sprintf(`{"event":"plugin_status","payload":{"status":"%s"}}`, status)))
}
