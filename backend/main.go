package main

import (
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

// Pre-compile our Go templates so they are fast to serve
var tmpl = template.Must(template.ParseGlob("templates/*.html"))

type DashboardServer struct {
	webClients map[*websocket.Conn]bool
	mutex      sync.Mutex
}

func NewDashboardServer() *DashboardServer {
	return &DashboardServer{
		webClients: make(map[*websocket.Conn]bool),
	}
}

func main() {
	server := NewDashboardServer()

	// Page Routes
	http.HandleFunc("/", server.HandleDashboard)
	http.HandleFunc("/console", server.HandleConsole)

	// WebSocket Routes
	http.HandleFunc("/ws", server.HandleMinecraftWebSocket)
	http.HandleFunc("/ws/web", server.HandleWebWebSocket)

	fmt.Println("🚀 Beacon Backend running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

// --- Page Handlers ---

func (s *DashboardServer) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	// We can pass data from Go directly into the HTML!
	data := map[string]interface{}{
		"Title":     "Overview",
		"ActiveTab": "dashboard",
		"Status":    "Online",
		"Players":   0, // We can wire this up to the Java plugin later!
	}
	tmpl.ExecuteTemplate(w, "base", data)
}

func (s *DashboardServer) HandleConsole(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":     "Live Console",
		"ActiveTab": "console",
	}
	tmpl.ExecuteTemplate(w, "base", data)
}

// --- WebSocket Handlers (Unchanged) ---

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
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.webClients[conn] = true
}

func (s *DashboardServer) removeWebClient(conn *websocket.Conn) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.webClients, conn)
}

func (s *DashboardServer) broadcastToWebClients(message []byte) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for client := range s.webClients {
		if err := client.WriteMessage(websocket.TextMessage, message); err != nil {
			client.Close()
			delete(s.webClients, client)
		}
	}
}
