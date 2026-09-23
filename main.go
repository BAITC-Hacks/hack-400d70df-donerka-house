package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/handlers"
	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
)

func loadCatalog(api *handlers.EKTAPI) (*models.Catalog, error) {
	catalog := &models.Catalog{}
	// Start immediately from the local snapshot. The authenticated EKT catalog
	// is synchronized in the background below so a large paginated catalog
	// cannot block the web server from starting.
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

	if api != nil && api.Enabled() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			products, err := api.LoadProducts(ctx)
			if err != nil {
				log.Printf("⚠️ Authenticated EKT catalog sync failed: %v", err)
				return
			}
			catalog.AddProducts(products)
			log.Printf("✅ Authenticated EKT catalog sync complete: %d products", len(products))
		}()
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

	ektAPI := handlers.NewEKTAPI(
		os.Getenv("EKT_API_BASE_URL"),
		os.Getenv("EKT_API_USERNAME"),
		os.Getenv("EKT_API_PASSWORD"),
	)
	catalog, err := loadCatalog(ektAPI)
	if err != nil {
		log.Fatalf("Failed to load catalog: %v", err)
	}

	if os.Getenv("OPENAI_API_KEY") == "" {
		log.Println("⚠️  OPENAI_API_KEY not set — demo mode active")
	} else {
		log.Println("✅ OpenAI API key configured")
	}
	if ektAPI.Enabled() {
		log.Println("✅ Authenticated EKT API configured")
	} else {
		log.Println("⚠️ EKT API credentials not set — local snapshot/live HTML mode active")
	}

	mux := http.NewServeMux()
	cartStore := handlers.NewCartStore()
	liveCatalog := handlers.NewLiveCatalog(os.Getenv("EKT_CATALOG_URL"))
	conversationStore := handlers.NewConversationStore()

	// Static files
	mux.Handle("/", http.FileServer(http.Dir("static")))

	// API
	mux.HandleFunc("/api/catalog", handlers.CatalogHandler(catalog))
	mux.HandleFunc("/api/chat", handlers.ChatHandler(catalog, cartStore, liveCatalog, conversationStore, ektAPI))
	mux.HandleFunc("/api/cart", handlers.CartHandler(catalog, cartStore))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","products":%d}`, catalog.ProductCount())
	})

	log.Printf("🚀 ekt.kz AI Assistant running on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, corsMiddleware(mux)); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
