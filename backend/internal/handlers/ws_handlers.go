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
}

// HandleMinecraft handles the connection from the Java plugin
func (m *WebSocketManager) HandleMinecraft(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	m.mcConn = conn
	m.Store.ClearLogs()
	fmt.Println("🟢 Minecraft Server Connected!")

	for {
		_, messageBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}

		m.processMinecraftMessage(messageBytes)
		m.broadcastToWeb(messageBytes)
	}
	m.mcConn = nil
}

func (m *WebSocketManager) processMinecraftMessage(messageBytes []byte) {
	var envelope struct {
		Event   string          `json:"event"`
		Payload json.RawMessage `json:"payload"`
	}

	if err := json.Unmarshal(messageBytes, &envelope); err != nil {
		return
	}

	switch envelope.Event {
	case "server_stats":
		var stats models.ServerStats
		if err := json.Unmarshal(envelope.Payload, &stats); err == nil {
			m.Store.UpdateStats(stats)
		}
	case "console_log":
		m.Store.AddLog(messageBytes)
	}
}

// HandleWeb handles browser UI connections
func (m *WebSocketManager) HandleWeb(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	m.registerWebClient(conn)
	defer m.unregisterWebClient(conn)

	// Send history on connect
	for _, msg := range m.Store.GetLogs() {
		conn.WriteMessage(websocket.TextMessage, msg)
	}

	for {
		_, messageBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}

		// Pass commands from web directly to Minecraft
		if m.mcConn != nil {
			m.mcConn.WriteMessage(websocket.TextMessage, messageBytes)
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
