package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
)

// CatalogHandler handles GET /api/catalog — returns product list
func CatalogHandler(catalog *models.Catalog) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Method not allowed"})
			return
		}

		products := catalog.ProductsSnapshot()
		page := models.ProductsPage{
			Page:    1,
			PerPage: len(products),
			Count:   len(products),
			Items:   products,
		}
		json.NewEncoder(w).Encode(page)
	}
}
