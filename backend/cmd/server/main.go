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
	authManager := handlers.NewAuthManager()
	authManager.LoadPersistedState()
	authManager.StartJanitor()

	// 2. Initialize our WebSocket manager with access to the store
	ws := &handlers.WebSocketManager{
		Store: serverStore,
		Auth:  authManager,
	}

	// 3. Initialize our UI handlers with access to the store and WebSocket manager
	ui := handlers.NewUIHandler(serverStore, ws, authManager)

	// Static Files (Adjust path based on where you run the binary from)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("../../static"))))

	// 4. Mount Page Routes
	http.HandleFunc("/auth", ui.HandleAuthPage)
	http.HandleFunc("/", ui.RequirePageAuth(ui.HandleDashboard))
	http.HandleFunc("/console", ui.RequirePageAuth(ui.HandleConsole))
	http.HandleFunc("/players", ui.RequirePageAuth(ui.HandlePlayers))
	http.HandleFunc("/worlds", ui.RequirePageAuth(ui.HandleWorlds))
	http.HandleFunc("/files", ui.RequirePageAuth(ui.HandleFiles))
	http.HandleFunc("/files/", ui.RequirePageAuth(ui.HandleFiles))
	http.HandleFunc("/access", ui.RequirePageAuth(ui.HandleAccess))

	// File manager API routes
	http.HandleFunc("/api/auth/magic-link", ui.HandleMagicLinkAuth)
	http.HandleFunc("/api/auth/logout", ui.RequireAPIAuth(ui.HandleLogout))
	http.HandleFunc("/api/session", ui.RequireAPIAuth(ui.HandleSession))
	http.HandleFunc("/api/files/meta", ui.RequireAPIAuth(ui.HandleFilesMeta))
	http.HandleFunc("/api/files/list", ui.RequireAPIAuth(ui.HandleFilesList))
	http.HandleFunc("/api/files/content", ui.RequireAPIAuth(ui.HandleFilesContent))
	http.HandleFunc("/api/files", ui.RequireAPIAuth(ui.HandleFilesDelete))
	http.HandleFunc("/api/files/download", ui.RequireAPIAuth(ui.HandleFilesDownload))
	http.HandleFunc("/api/access/data", ui.RequireAPIAuth(ui.HandleAccessData))
	http.HandleFunc("/api/access/sessions", ui.RequireAPIAuth(ui.HandleAccessSessionDelete))
	http.HandleFunc("/api/access/permissions", ui.RequireAPIAuth(ui.HandleAccessPermissionUpdate))
	http.HandleFunc("/api/gamerules/defaults", ui.HandleGameruleDefaults)

	// 5. Mount WebSocket Routes
	http.HandleFunc("/ws", ws.HandleMinecraft)
	http.HandleFunc("/ws/web", ws.HandleWeb)

	fmt.Println("🚀 Beacon Backend running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
