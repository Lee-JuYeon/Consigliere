package ui

import (
	"embed"
	"fmt"
	"net/http"

	"github.com/cavss/ledger/internal/api"
	"github.com/cavss/ledger/internal/store"
)

//go:embed index.html
var indexHTML embed.FS

// StartUI starts both the API server and the web UI on the same port.
func StartUI(s *store.Store, baseDir string, port int) error {
	apiServer := api.NewServer(s, baseDir, port)

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/search", cors(apiServer.HandleSearch))
	mux.HandleFunc("/api/status", cors(apiServer.HandleStatus))
	mux.HandleFunc("/api/chunks", cors(apiServer.HandleChunks))
	mux.HandleFunc("/api/check", cors(apiServer.HandleCheck))
	mux.HandleFunc("/api/health", cors(apiServer.HandleHealth))

	// UI
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data, _ := indexHTML.ReadFile("index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("Ledger UI: http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func cors(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(200)
			return
		}
		h(w, r)
	}
}
