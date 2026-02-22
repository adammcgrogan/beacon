package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// upgrader is used to upgrade standard HTTP requests to WebSocket connections.
// CheckOrigin is set to return true to allow connections from any origin.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// DashboardServer encapsulates the state of the backend, specifically tracking
// connected web browser clients safely to avoid global variables.
type DashboardServer struct {
	webClients map[*websocket.Conn]bool
	mutex      sync.Mutex
}

// NewDashboardServer initialises and returns a new DashboardServer instance
// with an empty map ready to track connected web clients.
func NewDashboardServer() *DashboardServer {
	return &DashboardServer{
		webClients: make(map[*websocket.Conn]bool),
	}
}

// main initialises the server state, registers the HTTP and WebSocket routes,
// and starts listening for connections on port 8080.
func main() {
	server := NewDashboardServer()

	http.HandleFunc("/", server.HandleIndex)
	http.HandleFunc("/ws", server.HandleMinecraftWebSocket)
	http.HandleFunc("/ws/web", server.HandleWebWebSocket)

	fmt.Println("🚀 MCDash Backend running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// HandleIndex serves the main HTML dashboard file to anyone who visits
// http://localhost:8080 in their web browser.
func (s *DashboardServer) HandleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

// HandleMinecraftWebSocket handles the connection originating from the plugin.
// It upgrades the connection to a WebSocket and continuously listens for incoming logs.
// When a log is received, it instantly broadcasts it to all connected web browsers.
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
			fmt.Println("🔴 Minecraft disconnected.")
			break
		}

		// Instantly forward the log payload from Minecraft to the web clients
		s.broadcastToWebClients(messageBytes)
	}
}

// HandleWebWebSocket handles connections originating from the web browsers viewing the dashboard.
// It registers the new browser connection in the tracking map and listens for
// disconnects so we can clean up the tracking map when the tab is closed.
func (s *DashboardServer) HandleWebWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	s.addWebClient(conn)
	fmt.Println("💻 New Web Dashboard Opened!")

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

// addWebClient safely adds a new WebSocket connection to the tracking map.
func (s *DashboardServer) addWebClient(conn *websocket.Conn) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.webClients[conn] = true
}

// removeWebClient safely removes a disconnected WebSocket connection from the tracking map.
func (s *DashboardServer) removeWebClient(conn *websocket.Conn) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.webClients, conn)
}

// broadcastToWebClients iterates through every connected web browser and sends them a message.
func (s *DashboardServer) broadcastToWebClients(message []byte) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for client := range s.webClients {
		err := client.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			client.Close()
			delete(s.webClients, client)
		}
	}
}
