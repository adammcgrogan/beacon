package main

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type ServerStats struct {
	Players    int    `json:"players"`
	MaxPlayers int    `json:"max_players"`
	TPS        string `json:"tps"`
	RamUsed    int64  `json:"ram_used"`
	RamMax     int64  `json:"ram_max"`
}

type DashboardServer struct {
	webClients  map[*websocket.Conn]bool
	clientsLock sync.Mutex

	latestStats ServerStats
	statsLock   sync.Mutex

	mcConn *websocket.Conn
	mcLock sync.Mutex

	logHistory  [][]byte
	historyLock sync.RWMutex
}

func NewDashboardServer() *DashboardServer {
	return &DashboardServer{
		webClients:  make(map[*websocket.Conn]bool),
		latestStats: ServerStats{TPS: "0.00"},
		logHistory:  make([][]byte, 0),
	}
}

// broadcastToWebClients sends a message to all connected web browsers safely
func (s *DashboardServer) broadcastToWebClients(message []byte) {
	s.clientsLock.Lock()
	defer s.clientsLock.Unlock()

	for client := range s.webClients {
		if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
			client.Close()
			delete(s.webClients, client)
		}
	}
}
