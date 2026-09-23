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

func loadCatalog() (*models.Catalog, error) {
	catalog := &models.Catalog{}

	// Load products from both files and merge
	files := []string{"data/products.json", "data/products2.json"}
	seen := map[int]bool{}

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			log.Printf("Skipping %s: %v", f, err)
			continue
		}
		var page models.ProductsPage
		if err := json.Unmarshal(data, &page); err != nil {
			log.Printf("Error parsing %s: %v", f, err)
			continue
		}
		for _, p := range page.Items {
			if !seen[p.ID] {
				catalog.Products = append(catalog.Products, p)
				seen[p.ID] = true
			}
		}
		log.Printf("✅ Loaded %d products from %s", len(page.Items), f)
	}

	// Load product detail example
	detailData, err := os.ReadFile("data/detail.json")
	if err == nil {
		var detail models.ProductDetail
		if err := json.Unmarshal(detailData, &detail); err == nil {
			catalog.Detail = &detail
			log.Printf("✅ Loaded product detail: %s", detail.Name)
		}
	}

	log.Printf("📦 Total products in catalog: %d", len(catalog.Products))
	return catalog, nil
}

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

	catalog, err := loadCatalog()
	if err != nil {
		log.Fatalf("Failed to load catalog: %v", err)
	}

	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Println("⚠️  OPENAI_API_KEY not set — demo mode active")
	} else {
		log.Println("✅ OpenAI API key configured")
	}

	mux := http.NewServeMux()
	cartStore := handlers.NewCartStore()

	// Static files
	mux.Handle("/", http.FileServer(http.Dir("static")))

	// API
	mux.HandleFunc("/api/catalog", handlers.CatalogHandler(catalog))
	mux.HandleFunc("/api/chat", handlers.ChatHandler(catalog, cartStore))
	mux.HandleFunc("/api/cart", handlers.CartHandler(catalog, cartStore))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","products":%d}`, len(catalog.Products))
	})

	log.Printf("🚀 ekt.kz AI Assistant running on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, corsMiddleware(mux)); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
