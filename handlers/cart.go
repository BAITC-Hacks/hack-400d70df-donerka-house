package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"fmt"
	"sync"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
)

const (
	sessionCookieName = "ekt_session"
	maxCartQuantity   = 999
)

// CartStore keeps carts isolated by a browser session. It is intentionally
// in-memory for this demo; a production deployment should replace it with a
// database or Redis-backed store.
type CartStore struct {
	mu    sync.RWMutex
	carts map[string]map[string]models.CartItem
}

func NewCartStore() *CartStore {
	return &CartStore{carts: make(map[string]map[string]models.CartItem)}
}

func newSessionID() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (s *CartStore) sessionID(w http.ResponseWriter, r *http.Request) (string, error) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		return cookie.Value, nil
	}

	id, err := newSessionID()
	if err != nil {
		return "", err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		MaxAge:   60 * 60 * 24 * 30,
	})
	return id, nil
}

func (s *CartStore) snapshot(sessionID string) models.CartResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	items := make([]models.CartItem, 0)
	for _, item := range s.carts[sessionID] {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Article < items[j].Article })

	response := models.CartResponse{Items: items}
	for _, item := range items {
		response.Count += item.Quantity
		response.Total += item.Price * float64(item.Quantity)
	}
	return response
}

func (s *CartStore) currentQuantity(sessionID, article string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if items := s.carts[sessionID]; items != nil {
		return items[article].Quantity
	}
	return 0
}

func validateCartQuantity(store *CartStore, sessionID string, product models.Product, quantity int) string {
	if quantity < 1 || quantity > maxCartQuantity {
		return "Количество должно быть от 1 до 999."
	}
	if strings.EqualFold(product.Availability, "Под заказ") {
		return "Товар сейчас под заказ и не может быть добавлен в корзину как имеющийся в наличии."
	}
	if product.StockQuantity > 0 {
		current := store.currentQuantity(sessionID, product.Article)
		if current+quantity > product.StockQuantity {
			return fmt.Sprintf("Доступно только %d шт. товара «%s». В корзине уже %d шт.", product.StockQuantity, product.Name, current)
		}
	}
	return ""
}

func (s *CartStore) add(sessionID string, product models.Product, quantity int) models.CartResponse {
	s.mu.Lock()
	if s.carts[sessionID] == nil {
		s.carts[sessionID] = make(map[string]models.CartItem)
	}
	item := s.carts[sessionID][product.Article]
	item.Article = product.Article
	item.Name = product.Name
	item.Price = product.Price
	item.Quantity += quantity
	s.carts[sessionID][product.Article] = item
	s.mu.Unlock()
	return s.snapshot(sessionID)
}

func (s *CartStore) remove(sessionID, article string) models.CartResponse {
	s.mu.Lock()
	if article == "" {
		delete(s.carts, sessionID)
	} else if items := s.carts[sessionID]; items != nil {
		delete(items, article)
	}
	s.mu.Unlock()
	return s.snapshot(sessionID)
}

// CartHandler exposes GET/POST/DELETE /api/cart for the current browser session.
func CartHandler(catalog *models.Catalog, store *CartStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		sessionID, err := store.sessionID(w, r)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Could not create a cart session")
			return
		}

		switch r.Method {
		case http.MethodGet:
			writeJSON(w, store.snapshot(sessionID))
		case http.MethodPost:
			var req models.CartRequest
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
			if err := decoder.Decode(&req); err != nil {
				writeJSONError(w, http.StatusBadRequest, "Invalid cart request")
				return
			}
			product := findProductByReference(catalog, req.Article)
			if product == nil {
				writeJSONError(w, http.StatusNotFound, "Product was not found in the catalog")
				return
			}
			if message := validateCartQuantity(store, sessionID, *product, req.Quantity); message != "" {
				writeJSONError(w, http.StatusBadRequest, message)
				return
			}
			writeJSON(w, store.add(sessionID, *product, req.Quantity))
		case http.MethodDelete:
			article := strings.TrimSpace(r.URL.Query().Get("article"))
			writeJSON(w, store.remove(sessionID, article))
		default:
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.WriteHeader(status)
	writeJSON(w, models.ErrorResponse{Error: message})
}
