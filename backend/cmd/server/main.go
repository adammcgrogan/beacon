package main

import (
	"flag"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"

	beaconassets "github.com/adammcgrogan/beacon"
	"github.com/adammcgrogan/beacon/internal/handlers"
	"github.com/adammcgrogan/beacon/internal/store"
)

var version = "1.0.0"

func main() {
	portFlag := flag.String("port", "8080", "Port for the HTTP/WebSocket server")
	flag.Parse()

	port := strings.TrimSpace(*portFlag)
	if port == "" {
		port = "8080"
	}

	templateSet, err := template.ParseFS(beaconassets.FS, "templates/*.html")
	if err != nil {
		log.Fatalf("failed to parse templates: %v", err)
	}

	staticFS, err := fs.Sub(beaconassets.FS, "static")
	if err != nil {
		log.Fatalf("failed to initialize static assets: %v", err)
	}

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
	ui := handlers.NewUIHandler(serverStore, ws, templateSet, authManager)

	// Static files served from the embedded filesystem.
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

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

	address := net.JoinHostPort("", port)
	baseURL := url.URL{Scheme: "http", Host: net.JoinHostPort("localhost", port), Path: path.Clean("/")}
	fmt.Printf("Beacon Backend %s running on %s\n", version, baseURL.String())
	log.Fatal(http.ListenAndServe(address, nil))
}
