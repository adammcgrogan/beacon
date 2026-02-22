package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/adammcgrogan/beacon/internal/handlers"
	"github.com/adammcgrogan/beacon/internal/store"
)

func main() {
	// 1. Initialize our centralized state/store
	serverStore := store.New()

	// 2. Initialize our UI handlers with access to the store
	ui := handlers.NewUIHandler(serverStore)

	// 3. Initialize our WebSocket manager with access to the store
	ws := &handlers.WebSocketManager{
		Store: serverStore,
	}

	// Static Files (Adjust path based on where you run the binary from)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("../../static"))))

	// Page Routes
	http.HandleFunc("/", ui.HandleDashboard)
	http.HandleFunc("/console", ui.HandleConsole)
	http.HandleFunc("/players", ui.HandlePlayers)

	// WebSocket Routes
	http.HandleFunc("/ws", ws.HandleMinecraft)
	http.HandleFunc("/ws/web", ws.HandleWeb)

	fmt.Println("🚀 Beacon Backend running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
