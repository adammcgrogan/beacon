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

	// 2. Initialize our WebSocket manager with access to the store
	ws := &handlers.WebSocketManager{
		Store: serverStore,
	}

	// 3. Initialize our UI handlers with access to the store and WebSocket manager
	ui := handlers.NewUIHandler(serverStore, ws, templateSet)

	// Static files served from the embedded filesystem.
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

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
	http.HandleFunc("/api/files", ui.HandleFilesDelete)
	http.HandleFunc("/api/files/download", ui.HandleFilesDownload)

	// 5. Mount WebSocket Routes
	http.HandleFunc("/ws", ws.HandleMinecraft)
	http.HandleFunc("/ws/web", ws.HandleWeb)

	address := net.JoinHostPort("", port)
	baseURL := url.URL{Scheme: "http", Host: net.JoinHostPort("localhost", port), Path: path.Clean("/")}
	fmt.Printf("Beacon Backend %s running on %s\n", version, baseURL.String())
	log.Fatal(http.ListenAndServe(address, nil))
}
