package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

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
	mu       sync.RWMutex
	carts    map[string]map[string]models.CartItem
	sessions map[string]time.Time
	removals map[string]removalConfirmation
}

func NewCartStore() *CartStore {
	return &CartStore{carts: make(map[string]map[string]models.CartItem), sessions: make(map[string]time.Time), removals: make(map[string]removalConfirmation)}
}

func newSessionID() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (s *CartStore) sessionID(w http.ResponseWriter, r *http.Request) (string, error) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil && s.validSession(cookie.Value) {
		return cookie.Value, nil
	}

	id, err := newSessionID()
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	s.sessions[id] = time.Now().Add(24 * time.Hour)
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secureCookie(r),
		MaxAge:   60 * 60 * 24,
	})
	return id, nil
}

func accountCartKey(userID string) string {
	return "account:" + userID
}

// MergeSessionIntoUser moves the current guest basket into the authenticated
// account. Existing quantities are combined instead of being overwritten.
func (s *CartStore) MergeSessionIntoUser(w http.ResponseWriter, r *http.Request, userID string) error {
	sessionID, err := s.sessionID(w, r)
	if err != nil {
		return err
	}
	if strings.TrimSpace(userID) == "" || sessionID == accountCartKey(userID) {
		return nil
	}

	s.mu.Lock()
	guestItems := s.carts[sessionID]
	if len(guestItems) > 0 {
		accountKey := accountCartKey(userID)
		if s.carts[accountKey] == nil {
			s.carts[accountKey] = make(map[string]models.CartItem)
		}
		for article, item := range guestItems {
			if existing, ok := s.carts[accountKey][article]; ok {
				existing.Quantity += item.Quantity
				s.carts[accountKey][article] = existing
			} else {
				s.carts[accountKey][article] = item
			}
		}
		delete(s.carts, sessionID)
	}
	s.mu.Unlock()
	return nil
}

func (s *CartStore) keyForRequest(w http.ResponseWriter, r *http.Request, auth *AuthStore) (string, error) {
	sessionID, err := s.sessionID(w, r)
	if err != nil {
		return "", err
	}
	if auth == nil {
		return sessionID, nil
	}
	if user := auth.User(r); user != nil {
		return accountCartKey(user.ID), nil
	}
	return sessionID, nil
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

func (s *CartStore) currentQuantity(cartKey, article string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if items := s.carts[cartKey]; items != nil {
		return items[article].Quantity
	}
	return 0
}

func validateCartQuantity(store *CartStore, cartKey string, product models.Product, quantity int) string {
	return validateQuantity(product, quantity, store.currentQuantity(cartKey, product.Article))
}

func validateQuantity(product models.Product, quantity, current int) string {
	if quantity < 1 || quantity > maxCartQuantity {
		return "Количество должно быть от 1 до 999."
	}
	if strings.EqualFold(product.Availability, "Под заказ") || strings.EqualFold(product.Availability, "Нет в наличии") {
		return "Товар сейчас недоступен для добавления в корзину."
	}
	if product.Price <= 0 || product.Availability != "В наличии" || product.StockQuantity <= 0 && product.TotalStockQuantity <= 0 {
		return "Цена или доступный остаток не подтверждены. Добавление недоступно до проверки данных EKT."
	}
	limit := product.StockQuantity
	if limit <= 0 && product.TotalStockQuantity > 0 {
		limit = product.TotalStockQuantity
	}
	if limit > 0 {
		if current+quantity > limit {
			return fmt.Sprintf("Доступно только %d шт. товара «%s». В корзине уже %d шт.", limit, product.Name, current)
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

// CartHandler keeps the original anonymous-session behavior for callers that
// do not configure local accounts.
func CartHandler(catalog *models.Catalog, store *CartStore) http.HandlerFunc {
	return CartHandlerWithAuth(catalog, store, nil)
}

// CartHandlerWithAuth exposes GET/POST/DELETE /api/cart. Authenticated users
// get an account-owned cart, while guests continue to use their browser cart.
func CartHandlerWithAuth(catalog *models.Catalog, store *CartStore, auth *AuthStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		cartKey, err := store.keyForRequest(w, r, auth)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Could not create a cart session")
			return
		}

		switch r.Method {
		case http.MethodGet:
			writeJSON(w, store.snapshot(cartKey))
		case http.MethodPost:
			writeJSONError(w, http.StatusConflict, "Добавление в корзину доступно только через подтверждённый сценарий чата: выберите товар и количество, затем отправьте «да, добавь».")
		case http.MethodDelete:
			var request struct {
				Token string `json:"confirmation_token"`
			}
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&request); err != nil || !store.confirmRemoval(cartKey, request.Token) {
				writeJSONError(w, http.StatusConflict, "Сначала подтвердите удаление выбранного товара или очистку корзины.")
				return
			}
			writeJSON(w, store.snapshot(cartKey))
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
