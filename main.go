package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/handlers"
	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
)

func loadCatalog() (*models.Catalog, error) {
	catalog := &models.Catalog{
		Details:      make(map[int]models.ProductDetail),
		Certificates: make(map[int][]models.Certificate),
	}

	// Load product summaries from both supplied pages and deduplicate by ID.
	files := []string{"data/products.json", "data/products2.json"}
	seen := map[int]bool{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			log.Printf("Skipping %s: %v", file, err)
			continue
		}
		var page models.ProductsPage
		if err := json.Unmarshal(data, &page); err != nil {
			log.Printf("Error parsing %s: %v", file, err)
			continue
		}
		for _, product := range page.Items {
			if !seen[product.ID] {
				catalog.Products = append(catalog.Products, product)
				seen[product.ID] = true
			}
		}
		log.Printf("Loaded %d products from %s", len(page.Items), file)
	}

	if data, err := os.ReadFile("data/detail.json"); err == nil {
		var detail models.ProductDetail
		if err := json.Unmarshal(data, &detail); err != nil {
			log.Printf("Error parsing data/detail.json: %v", err)
		} else {
			catalog.Details[detail.ID] = detail
			log.Printf("Loaded product detail: %s", detail.Name)
		}
	} else {
		log.Printf("No local product detail file: %v", err)
	}

	if data, err := os.ReadFile("data/certificates.json"); err == nil {
		if err := json.Unmarshal(data, &catalog.Certificates); err != nil {
			log.Printf("Error parsing data/certificates.json: %v", err)
		}
	} else {
		log.Printf("No certificate metadata file: %v", err)
	}

	if data, err := os.ReadFile("data/purchase_terms.json"); err == nil {
		if err := json.Unmarshal(data, &catalog.Terms); err != nil {
			log.Printf("Error parsing data/purchase_terms.json: %v", err)
		}
	} else {
		log.Printf("No purchase terms file: %v", err)
	}

	log.Printf("Total products: %d; detailed records: %d", len(catalog.Products), len(catalog.Details))
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
		log.Println("OPENAI_API_KEY not set — deterministic demo flows remain available")
	} else {
		log.Println("OpenAI API key configured")
	}

	store := handlers.NewSessionStore()
	mux := http.NewServeMux()

	mux.HandleFunc("/api/catalog", handlers.CatalogHandler(catalog))
	mux.HandleFunc("/api/product/", handlers.ProductHandler(catalog))
	mux.HandleFunc("/api/cart/", handlers.CartHandler(store))
	mux.HandleFunc("/api/chat", handlers.ChatHandler(catalog, store))
	mux.HandleFunc("/cart/", handlers.CartPageHandler(store))
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.Handle("/", http.FileServer(http.Dir("static")))

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"products": len(catalog.Products),
			"details": len(catalog.Details),
		})
	})

	log.Printf("EKT.KZ AI Assistant running on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, corsMiddleware(mux)); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
