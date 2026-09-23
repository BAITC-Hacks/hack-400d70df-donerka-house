package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/handlers"
	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
)

func loadMenu(path string) (*models.Menu, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read menu file: %w", err)
	}
	var menu models.Menu
	if err := json.Unmarshal(data, &menu); err != nil {
		return nil, fmt.Errorf("failed to parse menu JSON: %w", err)
	}
	return &menu, nil
}

// corsMiddleware adds CORS headers to allow browser requests
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Load menu data
	menuPath := os.Getenv("MENU_PATH")
	if menuPath == "" {
		menuPath = "data/menu.json"
	}

	menu, err := loadMenu(menuPath)
	if err != nil {
		log.Fatalf("Error loading menu: %v", err)
	}
	log.Printf("✅ Menu loaded: %s (%d categories)", menu.Restaurant.Name, len(menu.Categories))

	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Println("⚠️  OPENAI_API_KEY not set — running in demo mode")
	} else {
		log.Println("✅ OpenAI API key found — AI mode enabled")
	}

	mux := http.NewServeMux()

	// Static files (frontend)
	fs := http.FileServer(http.Dir("static"))
	mux.Handle("/", fs)

	// API routes
	mux.HandleFunc("/api/menu", handlers.MenuHandler(menu))
	mux.HandleFunc("/api/chat", handlers.ChatHandler(menu))

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","restaurant":"%s"}`, menu.Restaurant.Name)
	})

	handler := corsMiddleware(mux)

	log.Printf("🚀 Server running on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
