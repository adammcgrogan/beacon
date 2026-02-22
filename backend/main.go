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

// NEW: A struct to hold the latest server health data
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

	// NEW: Store the latest stats safely
	latestStats ServerStats
	statsLock   sync.Mutex
}

func NewDashboardServer() *DashboardServer {
	return &DashboardServer{
		webClients: make(map[*websocket.Conn]bool),
		// Set some default stats for before the server connects
		latestStats: ServerStats{TPS: "0.00"},
	}
}

func main() {
	server := NewDashboardServer()

	http.HandleFunc("/", server.HandleDashboard)
	http.HandleFunc("/console", server.HandleConsole)
	http.HandleFunc("/ws", server.HandleMinecraftWebSocket)
	http.HandleFunc("/ws/web", server.HandleWebWebSocket)

	fmt.Println("🚀 MCDash Backend running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func (s *DashboardServer) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	s.statsLock.Lock()
	currentStats := s.latestStats
	s.statsLock.Unlock()

	// NEW: Pass the real stats to the HTML!
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
	defer conn.Close()

	fmt.Println("🟢 Minecraft Server Connected!")

	for {
		_, messageBytes, err := conn.ReadMessage()
		if err != nil {
			break
		}

		// NEW: Inspect the JSON to see if it's a stats update
		var incoming map[string]interface{}
		if err := json.Unmarshal(messageBytes, &incoming); err == nil {
			if event, ok := incoming["event"].(string); ok && event == "server_stats" {

				// Parse the payload and update our Go memory
				if payload, ok := incoming["payload"].(map[string]interface{}); ok {
					s.statsLock.Lock()
					s.latestStats.Players = int(payload["players"].(float64))
					s.latestStats.MaxPlayers = int(payload["max_players"].(float64))
					s.latestStats.TPS = payload["tps"].(string)
					s.latestStats.RamUsed = int64(payload["ram_used"].(float64))
					s.latestStats.RamMax = int64(payload["ram_max"].(float64))
					s.statsLock.Unlock()
				}
			}
		}

		// Always broadcast everything to the web clients (for the live console)
		s.broadcastToWebClients(messageBytes)
	}
}

func (s *DashboardServer) HandleWebWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.addWebClient(conn)
	defer func() {
		s.removeWebClient(conn)
		conn.Close()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (s *DashboardServer) addWebClient(conn *websocket.Conn) {
	s.clientsLock.Lock()
	defer s.clientsLock.Unlock()
	s.webClients[conn] = true
}

func (s *DashboardServer) removeWebClient(conn *websocket.Conn) {
	s.clientsLock.Lock()
	defer s.clientsLock.Unlock()
	delete(s.webClients, conn)
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
