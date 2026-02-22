package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var tmpl = template.Must(template.ParseGlob("templates/*.html"))

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

	// NEW: A buffer to hold historical logs, and a lock to keep it thread-safe
	logHistory  [][]byte
	historyLock sync.RWMutex
}

func NewDashboardServer() *DashboardServer {
	return &DashboardServer{
		webClients:  make(map[*websocket.Conn]bool),
		latestStats: ServerStats{TPS: "0.00"},
		logHistory:  make([][]byte, 0), // Initialize empty history
	}
}

func main() {
	server := NewDashboardServer()

	http.HandleFunc("/", server.HandleDashboard)
	http.HandleFunc("/console", server.HandleConsole)
	http.HandleFunc("/ws", server.HandleMinecraftWebSocket)
	http.HandleFunc("/ws/web", server.HandleWebWebSocket)

	fmt.Println("🚀 Beacon Backend running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func (s *DashboardServer) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	s.statsLock.Lock()
	currentStats := s.latestStats
	s.statsLock.Unlock()

	data := map[string]interface{}{
		"Title":     "Overview",
		"ActiveTab": "dashboard",
		"Status":    "Online",
		"Stats":     currentStats,
	}
	tmpl.ExecuteTemplate(w, "base", data)
}

func (s *DashboardServer) HandleConsole(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{"Title": "Live Console", "ActiveTab": "console"}
	tmpl.ExecuteTemplate(w, "base", data)
}

func (s *DashboardServer) HandleMinecraftWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.mcLock.Lock()
	s.mcConn = conn
	s.mcLock.Unlock()

	// NEW: Clear the history when the Minecraft server restarts/reconnects
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
					// NEW: Save the log to our history buffer
					s.historyLock.Lock()
					// Keep only the last 1000 logs to prevent memory leaks
					if len(s.logHistory) >= 1000 {
						s.logHistory = s.logHistory[1:] // Remove the oldest log
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

	// NEW: As soon as a browser connects, send them the entire log history
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
