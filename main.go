package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/handlers"
	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
)

func loadRemoteDetails(catalog *models.Catalog) {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("LOAD_REMOTE_DETAILS")), "false") {
		log.Println("Remote detail loading disabled by LOAD_REMOTE_DETAILS=false")
		return
	}

	type result struct {
		detail models.ProductDetail
		err    error
	}
	client := &http.Client{Timeout: 5 * time.Second}
	jobs := make(chan models.Product)
	results := make(chan result)
	var workers sync.WaitGroup

	worker := func() {
		defer workers.Done()
		for product := range jobs {
			if product.URLAPIDetail == "" {
				continue
			}
			request, err := http.NewRequest(http.MethodGet, product.URLAPIDetail, nil)
			if err != nil {
				results <- result{err: err}
				continue
			}
			request.Header.Set("User-Agent", "ekt-ai-assistant/1.0")
			response, err := client.Do(request)
			if err != nil {
				results <- result{err: err}
				continue
			}
			var detail models.ProductDetail
			err = json.NewDecoder(response.Body).Decode(&detail)
			_ = response.Body.Close()
			if err == nil && detail.ID == 0 {
				err = os.ErrInvalid
			}
			results <- result{detail: detail, err: err}
		}
	}

	workerCount := 6
	if len(catalog.Products) < workerCount {
		workerCount = len(catalog.Products)
	}
	for i := 0; i < workerCount; i++ {
		workers.Add(1)
		go worker()
	}
	go func() {
		for _, product := range catalog.Products {
			if _, loaded := catalog.Details[product.ID]; !loaded {
				jobs <- product
			}
		}
		close(jobs)
		workers.Wait()
		close(results)
	}()

	loaded := 0
	for item := range results {
		if item.err != nil {
			continue
		}
		catalog.Details[item.detail.ID] = item.detail
		loaded++
	}
	log.Printf("Loaded %d additional product details from url_api_detail", loaded)
}

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

	loadRemoteDetails(catalog)

	if data, err := os.ReadFile("data/certificates.json"); err == nil {
		if err := json.Unmarshal(data, &catalog.Certificates); err != nil {
			log.Printf("Error parsing data/certificates.json: %v", err)
		}
	} else {
		log.Printf("No certificate metadata file: %v", err)
	}
	for _, product := range catalog.Products {
		if len(catalog.Certificates[product.ID]) == 0 {
			catalog.Certificates[product.ID] = []models.Certificate{{
				Name:   "Сертификат: подтвердить у поставщика (демо-ссылка)",
				URL:    product.URL,
				IsDemo: true,
			}}
		}
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
			"status":   "ok",
			"products": len(catalog.Products),
			"details":  len(catalog.Details),
		})
	})

	log.Printf("EKT.KZ AI Assistant running on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, corsMiddleware(mux)); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
