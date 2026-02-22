package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
)

// Pre-compile our Go templates
var tmpl = template.Must(template.ParseGlob("templates/*.html"))

func main() {
	server := NewDashboardServer()

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Page Routes
	http.HandleFunc("/", server.HandleDashboard)
	http.HandleFunc("/console", server.HandleConsole)

	// WebSocket Routes
	http.HandleFunc("/ws", server.HandleMinecraftWebSocket)
	http.HandleFunc("/ws/web", server.HandleWebWebSocket)

	fmt.Println("🚀 Beacon Backend running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
