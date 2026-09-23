package handlers

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func (s *CartStore) validSession(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	expires, ok := s.sessions[id]
	return ok && time.Now().Before(expires)
}

func csrfToken(session string) string {
	digest := sha256.Sum256([]byte("ekt-csrf:" + session))
	return hex.EncodeToString(digest[:])
}

func (s *CartStore) SessionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, 405, "Method not allowed")
		return
	}
	id, err := s.sessionID(w, r)
	if err != nil {
		writeJSONError(w, 500, "Не удалось создать сессию")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, map[string]string{"csrf_token": csrfToken(id)})
}

// BrowserSecurity protects every mutation, including login and logout. A
// configured PUBLIC_ORIGIN is useful behind a trusted HTTPS reverse proxy.
func (s *CartStore) BrowserSecurity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'self'; object-src 'none'; base-uri 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store")
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			expected := scheme + "://" + r.Host
			if configured := os.Getenv("PUBLIC_ORIGIN"); configured != "" {
				expected = strings.TrimRight(configured, "/")
			}
			origin := r.Header.Get("Origin")
			if origin == "" && r.Header.Get("Referer") != "" {
				if ref, err := url.Parse(r.Header.Get("Referer")); err == nil {
					origin = ref.Scheme + "://" + ref.Host
				}
			}
			if origin != "" && origin != expected || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				writeJSONError(w, 403, "Запрос с другого сайта запрещён")
				return
			}
			cookie, err := r.Cookie(sessionCookieName)
			if err != nil || !s.validSession(cookie.Value) || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(csrfToken(cookie.Value))) != 1 {
				writeJSONError(w, 403, "Обновите страницу: защитный токен сессии отсутствует или устарел")
				return
			}
			contentType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if contentType != "application/json" {
				writeJSONError(w, 415, "Expected application/json")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func secureCookie(r *http.Request) bool {
	return r.TLS != nil || strings.HasPrefix(os.Getenv("PUBLIC_ORIGIN"), "https://")
}

type removalConfirmation struct {
	Token       string
	Article     string
	Fingerprint string
	Expires     time.Time
}

func (s *CartStore) RemovalHandler(auth *AuthStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, 405, "Method not allowed")
			return
		}
		var request struct {
			Action  string `json:"action"`
			Article string `json:"article"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&request); err != nil || request.Action != "remove" && request.Action != "clear" || request.Action == "remove" && request.Article == "" {
			writeJSONError(w, 400, "Укажите действие и товар")
			return
		}
		key, err := s.keyForRequest(w, r, auth)
		if err != nil {
			writeJSONError(w, 500, "Session error")
			return
		}
		token, err := randomToken(24)
		if err != nil {
			writeJSONError(w, 500, "Token error")
			return
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		items := s.carts[key]
		if len(items) == 0 {
			writeJSONError(w, 409, "Корзина пуста")
			return
		}
		article, summary := "", "Подтвердите удаление всех товаров из корзины."
		if request.Action == "remove" {
			item, ok := items[request.Article]
			if !ok {
				writeJSONError(w, 404, "Товар отсутствует в корзине")
				return
			}
			article = request.Article
			summary = fmt.Sprintf("Удалить «%s» — %d шт. из корзины?", item.Name, item.Quantity)
		}
		encoded, _ := json.Marshal(items)
		s.removals[key] = removalConfirmation{token, article, string(encoded), time.Now().Add(5 * time.Minute)}
		writeJSON(w, map[string]string{"confirmation_token": token, "summary": summary})
	}
}

func (s *CartStore) confirmRemoval(key, token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	pending, ok := s.removals[key]
	if !ok || token == "" || pending.Token != token || time.Now().After(pending.Expires) {
		return false
	}
	delete(s.removals, key)
	encoded, _ := json.Marshal(s.carts[key])
	if string(encoded) != pending.Fingerprint {
		return false
	}
	if pending.Article == "" {
		delete(s.carts, key)
	} else {
		delete(s.carts[key], pending.Article)
	}
	return true
}

func (s *CartStore) pruneExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, expires := range s.sessions {
		if time.Now().After(expires) {
			delete(s.sessions, id)
			delete(s.carts, id)
			delete(s.removals, id)
		}
	}
	for id, pending := range s.removals {
		if time.Now().After(pending.Expires) {
			delete(s.removals, id)
		}
	}
}
