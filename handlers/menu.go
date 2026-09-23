package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
)

// MenuHandler handles GET /api/menu requests
func MenuHandler(menu *models.Menu) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Method not allowed"})
			return
		}

		json.NewEncoder(w).Encode(menu)
	}
}
