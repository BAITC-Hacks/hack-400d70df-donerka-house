package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
)

type productResponse struct {
	Product          models.Product       `json:"product"`
	Detail           *models.ProductDetail `json:"detail,omitempty"`
	DetailAvailable  bool                 `json:"detail_available"`
	Certificates     []models.Certificate  `json:"certificates"`
	CertificateNote  string               `json:"certificate_note,omitempty"`
}

// ProductHandler handles GET /api/product/{id}.
func ProductHandler(catalog *models.Catalog) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Method not allowed"})
			return
		}

		idText := strings.TrimPrefix(r.URL.Path, "/api/product/")
		id, err := strconv.Atoi(idText)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Invalid product ID"})
			return
		}

		var product models.Product
		found := false
		for _, candidate := range catalog.Products {
			if candidate.ID == id {
				product = candidate
				found = true
				break
			}
		}
		if !found {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Product not found"})
			return
		}

		detail, hasDetail := catalog.DetailFor(id)
		certificates := catalog.Certificates[id]
		if certificates == nil {
			certificates = []models.Certificate{}
		}

		response := productResponse{
			Product:         product,
			Detail:          detail,
			DetailAvailable: hasDetail,
			Certificates:    certificates,
		}
		if !hasDetail {
			response.CertificateNote = "Точная характеристика, остаток и сертификат для этой позиции пока не загружены из detail API."
		}
		_ = json.NewEncoder(w).Encode(response)
	}
}
