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

		var incoming map[string]interface{}
		if err := json.Unmarshal(messageBytes, &incoming); err == nil {
			if event, ok := incoming["event"].(string); ok {
				if event == "server_stats" {
					if payload, ok := incoming["payload"].(map[string]interface{}); ok {
						s.statsLock.Lock()
						s.latestStats.Players = int(payload["players"].(float64))
						s.latestStats.MaxPlayers = int(payload["max_players"].(float64))
						s.latestStats.TPS = payload["tps"].(string)
						s.latestStats.RamUsed = int64(payload["ram_used"].(float64))
						s.latestStats.RamMax = int64(payload["ram_max"].(float64))
						s.statsLock.Unlock()
					}
				} else if event == "console_log" {
					s.historyLock.Lock()
					if len(s.logHistory) >= 1000 {
						s.logHistory = s.logHistory[1:]
					}
					s.logHistory = append(s.logHistory, messageBytes)
					s.historyLock.Unlock()
				}
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
