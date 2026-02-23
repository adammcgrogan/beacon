package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/adammcgrogan/beacon/internal/handlers"
	"github.com/adammcgrogan/beacon/internal/store"
)

func main() {
	// 1. Initialize centralized state
	serverStore := store.New()

	// 2. Initialize Handlers
	ui := handlers.NewUIHandler(serverStore)
	ws := &handlers.WebSocketManager{
		Store: serverStore,
	}

	// 3. Mount Static Files
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("../../static"))))

	// 4. Mount Page Routes
	http.HandleFunc("/", ui.HandleDashboard)
	http.HandleFunc("/console", ui.HandleConsole)
	http.HandleFunc("/players", ui.HandlePlayers)
	http.HandleFunc("/worlds", ui.HandleWorlds)

	// 5. Mount WebSocket Routes
	http.HandleFunc("/ws", ws.HandleMinecraft)
	http.HandleFunc("/ws/web", ws.HandleWeb)

	fmt.Println("🚀 Beacon Backend running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
