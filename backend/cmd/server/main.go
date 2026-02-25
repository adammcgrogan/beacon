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

	// 2. Initialize our WebSocket manager with access to the store
	ws := &handlers.WebSocketManager{
		Store: serverStore,
	}

	// 3. Initialize our UI handlers with access to the store and WebSocket manager
	ui := handlers.NewUIHandler(serverStore, ws)

	// Static Files (Adjust path based on where you run the binary from)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("../../static"))))

	// 4. Mount Page Routes
	http.HandleFunc("/", ui.HandleDashboard)
	http.HandleFunc("/console", ui.HandleConsole)
	http.HandleFunc("/players", ui.HandlePlayers)
	http.HandleFunc("/worlds", ui.HandleWorlds)
	http.HandleFunc("/files", ui.HandleFiles)
	http.HandleFunc("/files/", ui.HandleFiles)

	// File manager API routes
	http.HandleFunc("/api/files/meta", ui.HandleFilesMeta)
	http.HandleFunc("/api/files/list", ui.HandleFilesList)
	http.HandleFunc("/api/files/content", ui.HandleFilesContent)
	http.HandleFunc("/api/files/create", ui.HandleFilesCreate)
	http.HandleFunc("/api/files/upload", ui.HandleFilesUpload)
	http.HandleFunc("/api/files", ui.HandleFilesDelete)
	http.HandleFunc("/api/files/download", ui.HandleFilesDownload)
	http.HandleFunc("/api/gamerules/defaults", ui.HandleGameruleDefaults)

	// 5. Mount WebSocket Routes
	http.HandleFunc("/ws", ws.HandleMinecraft)
	http.HandleFunc("/ws/web", ws.HandleWeb)

	fmt.Println("🚀 Beacon Backend running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
