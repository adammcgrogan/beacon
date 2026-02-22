package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
)

func (s *DashboardServer) HandleMinecraftWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.mcLock.Lock()
	s.mcConn = conn
	s.mcLock.Unlock()

	s.historyLock.Lock()
	s.logHistory = make([][]byte, 0)
	s.historyLock.Unlock()

	defer func() {
		s.mcLock.Lock()
		if s.mcConn == conn {
			s.mcConn = nil
		}
		s.mcLock.Unlock()
		conn.Close()
	}()

	fmt.Println("🟢 Minecraft Server Connected!")

	for {
		_, messageBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}

		// Cleanly unmarshal only the envelope first
		var incoming struct {
			Event   string          `json:"event"`
			Payload json.RawMessage `json:"payload"`
		}

		if err := json.Unmarshal(messageBytes, &incoming); err == nil {
			switch incoming.Event {
			case "server_stats":
				var stats ServerStats
				if err := json.Unmarshal(incoming.Payload, &stats); err == nil {
					s.statsLock.Lock()
					s.latestStats = stats
					s.statsLock.Unlock()
				}
			case "console_log":
				s.historyLock.Lock()
				if len(s.logHistory) >= 1000 {
					s.logHistory = s.logHistory[1:]
				}
				s.logHistory = append(s.logHistory, messageBytes)
				s.historyLock.Unlock()
			}
		}

		s.broadcastToWebClients(messageBytes)
	}
}

func (s *DashboardServer) HandleWebWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.clientsLock.Lock()
	s.webClients[conn] = true
	s.clientsLock.Unlock()

	defer func() {
		s.clientsLock.Lock()
		delete(s.webClients, conn)
		s.clientsLock.Unlock()
		conn.Close()
	}()

	// Send log history immediately upon connection
	s.historyLock.RLock()
	for _, msg := range s.logHistory {
		conn.WriteMessage(websocket.TextMessage, msg)
	}
	s.historyLock.RUnlock()

	for {
		_, messageBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}

		s.mcLock.Lock()
		if s.mcConn != nil {
			s.mcConn.WriteMessage(websocket.TextMessage, messageBytes)
		}
		s.mcLock.Unlock()
	}
}
