package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
)

func TestEKTAPILoadsPagesAndDetailsWithBasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if !ok || username != "apiuser" || password != "secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/products" {
			page := 1
			if rawPage := r.URL.Query().Get("page"); rawPage != "" {
				page, _ = strconv.Atoi(rawPage)
			}
			items := []models.Product{{ID: 1, Name: "Первый", Article: "A1", Price: 100}, {ID: 3, Name: "Третий", Article: "A3", Price: 300}}
			if page == 2 {
				items = []models.Product{{ID: 2, Name: "Второй", Article: "A2", Price: 200}}
			}
			_ = json.NewEncoder(w).Encode(models.ProductsPage{Page: page, PerPage: 2, Count: len(items), Items: items})
			return
		}
		if r.URL.Path == "/api/products/detail" && r.URL.Query().Get("id") == "1" {
			_ = json.NewEncoder(w).Encode(models.ProductDetail{
				ID: 1, Name: "Первый", Article: "A1", Price: 110, Quantity: 7,
				Stores:     []models.Store{{ID: 3, Name: "Алматы", Quantity: 7}},
				Properties: map[string]interface{}{"Бренд": "EKT"},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	api := NewEKTAPI(server.URL+"/api", "apiuser", "secret")
	products, err := api.LoadProducts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 3 || products[2].ID != 2 {
		t.Fatalf("unexpected paginated products: %+v", products)
	}

	enriched := api.Enrich(context.Background(), products[:1])
	if enriched[0].TotalStockQuantity != 7 || enriched[0].Availability != "В наличии" || enriched[0].Stores[0].Name != "Алматы" || enriched[0].Properties["Бренд"] != "EKT" {
		t.Fatalf("unexpected enriched product: %+v", enriched[0])
	}
}
