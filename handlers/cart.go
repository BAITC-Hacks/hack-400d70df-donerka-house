package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"html"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
)

type sessionState struct {
	mu      sync.Mutex
	History []models.ChatMessage
	Pending *models.PendingCart
	Items   []models.CartItem
}

type SessionStore struct {
	mu       sync.Mutex
	sessions map[string]*sessionState
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*sessionState)}
}

func newSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
}

func (s *SessionStore) Get(id string) (string, *sessionState) {
	id = strings.TrimSpace(id)
	if id == "" {
		id = newSessionID()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if session, ok := s.sessions[id]; ok {
		return id, session
	}
	session := &sessionState{}
	s.sessions[id] = session
	return id, session
}

func (s *SessionStore) Snapshot(id string) models.CartResponse {
	id, session := s.Get(id)
	session.mu.Lock()
	defer session.mu.Unlock()

	items := append([]models.CartItem(nil), session.Items...)
	var total float64
	for _, item := range items {
		total += item.Price * float64(item.Quantity)
	}
	return models.CartResponse{SessionID: id, Items: items, Total: total}
}

// CartHandler handles GET /api/cart/{session_id}.
func CartHandler(store *SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Method not allowed"})
			return
		}

		sessionID := strings.TrimPrefix(r.URL.Path, "/api/cart/")
		if strings.TrimSpace(sessionID) == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Session ID is required"})
			return
		}
		_ = json.NewEncoder(w).Encode(store.Snapshot(sessionID))
	}
}

const cartPageTemplate = `<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Корзина — ГК Электрокомплект</title>
  <style>
    body { margin: 0; padding: 32px; font: 16px Arial, sans-serif; background: #f4f7fb; color: #172033; }
    main { max-width: 760px; margin: 0 auto; background: #fff; padding: 28px; border-radius: 16px; box-shadow: 0 10px 30px rgba(20,40,80,.08); }
    h1 { margin-top: 0; }
    li { padding: 12px 0; border-bottom: 1px solid #e7ecf3; }
    .total { font-size: 20px; font-weight: 700; margin-top: 22px; }
    .muted { color: #68758a; }
    a { color: #1959d1; }
  </style>
</head>
<body>
  <main>
    <p><a href="/">← Вернуться в каталог</a></p>
    <h1>Корзина</h1>
    <p class="muted">Корзина сохранена для этой сессии браузера.</p>
    <ul id="items"><li>Загрузка…</li></ul>
    <div class="total" id="total"></div>
  </main>
  <script>
    const sessionId = "__SESSION_ID__";
    async function loadCart() {
      const response = await fetch("/api/cart/" + encodeURIComponent(sessionId));
      const data = await response.json();
      const list = document.getElementById("items");
      list.textContent = "";
      if (!data.items || data.items.length === 0) {
        const empty = document.createElement("li");
        empty.textContent = "Корзина пока пуста.";
        list.appendChild(empty);
      } else {
        data.items.forEach(function(item) {
          const row = document.createElement("li");
          row.textContent = item.name + " · " + item.quantity + " шт. · " +
            Number(item.price * item.quantity).toLocaleString("ru-KZ") + " тг";
          list.appendChild(row);
        });
      }
      document.getElementById("total").textContent =
        "Итого: " + Number(data.total || 0).toLocaleString("ru-KZ") + " тг";
    }
    loadCart().catch(function() {
      document.getElementById("items").textContent = "Не удалось загрузить корзину.";
    });
  </script>
</body>
</html>`

// CartPageHandler renders a shareable cart URL: /cart/{session_id}.
func CartPageHandler(store *SessionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		sessionID := strings.TrimPrefix(r.URL.Path, "/cart/")
		if strings.TrimSpace(sessionID) == "" {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		sessionID, _ = store.Get(sessionID)
		page := strings.Replace(cartPageTemplate, "__SESSION_ID__", html.EscapeString(sessionID), 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	}
}
