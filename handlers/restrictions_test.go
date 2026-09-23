package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
)

func TestForgedGuestCookieCannotReadAccountCart(t *testing.T) {
	s := NewCartStore()
	s.add("account:victim", testCatalog().Products[0], 1)
	r := doJSONRequest(CartHandler(testCatalog(), s), "GET", "/api/cart", nil, &http.Cookie{Name: sessionCookieName, Value: "account:victim"})
	var cart models.CartResponse
	json.NewDecoder(r.Body).Decode(&cart)
	if cart.Count != 0 {
		t.Fatalf("account cart leaked: %+v", cart)
	}
	if sessionCookie(t, r).Value == "account:victim" {
		t.Fatal("accepted forged session")
	}
}

func TestCSRFProtectsAllMutations(t *testing.T) {
	s := NewCartStore()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/session", s.SessionHandler)
	var writes atomic.Int32
	mux.HandleFunc("/api/cart", func(w http.ResponseWriter, r *http.Request) { writes.Add(1) })
	h := s.BrowserSecurity(mux)
	initial := doJSONRequest(h, "GET", "/api/session", nil, nil)
	cookie := sessionCookie(t, initial)
	for _, tc := range []struct {
		name, token, origin, contentType string
		code                             int
	}{
		{"missing token", "", "", "application/json", 403},
		{"cross origin", csrfToken(cookie.Value), "https://attacker.example", "application/json", 403},
		{"form", csrfToken(cookie.Value), "", "text/plain", 415},
		{"valid", csrfToken(cookie.Value), "http://example.com", "application/json", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("DELETE", "/api/cart", strings.NewReader("{}"))
			r.AddCookie(cookie)
			r.Header.Set("X-CSRF-Token", tc.token)
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Content-Type", tc.contentType)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.code {
				t.Fatalf("code=%d body=%s", w.Code, w.Body)
			}
			if w.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("unexpected CORS permission")
			}
		})
	}
	if writes.Load() != 1 {
		t.Fatalf("unauthorized mutations: %d", writes.Load())
	}
}

func TestConfirmationIsSingleUseAndExpires(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	s := NewCartStore()
	c := NewConversationStore()
	h := ChatHandler(testCatalog(), s, nil, c)
	first := doJSONRequest(h, "POST", "/api/chat", []byte(`{"message":"Добавь 2 шт 200300285_"}`), nil)
	cookie := sessionCookie(t, first)
	var proposal models.ChatResponse
	json.NewDecoder(first.Body).Decode(&proposal)
	if proposal.ConfirmationToken == "" {
		t.Fatalf("no selection: %s", proposal.Reply)
	}
	if s.snapshot(cookie.Value).Count != 0 {
		t.Fatal("cart changed before confirmation")
	}
	wrong, _ := json.Marshal(models.ChatRequest{Message: "да, добавь", ConfirmationToken: "wrong"})
	doJSONRequest(h, "POST", "/api/chat", wrong, cookie)
	if s.snapshot(cookie.Value).Count != 0 {
		t.Fatal("wrong token accepted")
	}
	body, _ := json.Marshal(models.ChatRequest{Message: "да, добавь", ConfirmationToken: proposal.ConfirmationToken})
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); doJSONRequest(h, "POST", "/api/chat", body, cookie) }()
	}
	wg.Wait()
	if got := s.snapshot(cookie.Value).Count; got != 2 {
		t.Fatalf("replayed confirmation: count=%d", got)
	}
	c.setPending(cookie.Value, testCatalog().Products[0], 1)
	c.mu.Lock()
	c.conversations[cookie.Value].pending.Expires = time.Now().Add(-time.Second)
	c.mu.Unlock()
	doJSONRequest(h, "POST", "/api/chat", []byte(`{"message":"да, добавь"}`), cookie)
	if s.snapshot(cookie.Value).Count != 2 {
		t.Fatal("expired confirmation accepted")
	}
}

func TestCartRemovalRequiresFreshScopedConfirmation(t *testing.T) {
	s := NewCartStore()
	h := CartHandler(testCatalog(), s)
	cookie := sessionCookie(t, doJSONRequest(h, "GET", "/api/cart", nil, nil))
	s.add(cookie.Value, testCatalog().Products[0], 2)
	response := doJSONRequest(h, "DELETE", "/api/cart", []byte(`{}`), cookie)
	if response.Code != 409 || s.snapshot(cookie.Value).Count != 2 {
		t.Fatal("unconfirmed removal accepted")
	}
	proposal := doJSONRequest(s.RemovalHandler(nil), "POST", "/api/cart/confirmation", []byte(`{"action":"clear"}`), cookie)
	var data map[string]string
	json.NewDecoder(proposal.Body).Decode(&data)
	token := data["confirmation_token"]
	if token == "" {
		t.Fatal("missing token")
	}
	if s.confirmRemoval("another-session", token) {
		t.Fatal("cross-session token accepted")
	}
	s.add(cookie.Value, testCatalog().Products[0], 1)
	if s.confirmRemoval(cookie.Value, token) {
		t.Fatal("stale cart confirmation accepted")
	}
	proposal = doJSONRequest(s.RemovalHandler(nil), "POST", "/api/cart/confirmation", []byte(`{"action":"clear"}`), cookie)
	json.NewDecoder(proposal.Body).Decode(&data)
	if !s.confirmRemoval(cookie.Value, data["confirmation_token"]) || s.confirmRemoval(cookie.Value, data["confirmation_token"]) {
		t.Fatal("confirmation must work once")
	}
	if s.snapshot(cookie.Value).Count != 0 {
		t.Fatal("clear failed")
	}
}

func TestPaymentInputNeverReachesHistoryOrUpstream(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "must-not-be-used")
	var requests atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); http.Error(w, "unexpected", 500) }))
	defer upstream.Close()
	c := NewConversationStore()
	s := NewCartStore()
	h := ChatHandler(testCatalog(), s, NewLiveCatalog(upstream.URL), c)
	for _, message := range []string{"4111 1111 1111 1111", "CVV 123", "PIN 1234", "SMS 123456", "KZ86125KZT5004100100"} {
		body, _ := json.Marshal(models.ChatRequest{Message: message})
		response := doJSONRequest(h, "POST", "/api/chat", body, nil)
		if strings.Contains(response.Body.String(), message) || !strings.Contains(response.Body.String(), "Не отправляйте") {
			t.Fatalf("unsafe reply: %s", response.Body)
		}
	}
	if requests.Load() != 0 || len(c.conversations) != 0 {
		t.Fatal("payment data reached upstream/history")
	}
	if got := redactContactData("buyer@example.com +7 (777) 123-45-67 автомат 010400273"); strings.Contains(got, "buyer@") || strings.Contains(got, "777") || !strings.Contains(got, "010400273") {
		t.Fatalf("redaction: %s", got)
	}
}

func TestPriceChangeAndUpstreamFailureDoNotMutateCart(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	var price atomic.Int64
	price.Store(100)
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			http.Error(w, "unavailable", 503)
			return
		}
		json.NewEncoder(w).Encode(models.ProductDetail{ID: 515291, Article: "200300285_", Name: "Товар", Price: float64(price.Load()), Quantity: 5})
	}))
	defer server.Close()
	api := NewEKTAPI(server.URL, "user", "test")
	s := NewCartStore()
	c := NewConversationStore()
	h := ChatHandler(testCatalog(), s, nil, c, api)
	first := doJSONRequest(h, "POST", "/api/chat", []byte(`{"message":"Добавь 1 шт 200300285_"}`), nil)
	cookie := sessionCookie(t, first)
	price.Store(125)
	response := doJSONRequest(h, "POST", "/api/chat", []byte(`{"message":"да, добавь"}`), cookie)
	var changed models.ChatResponse
	json.NewDecoder(response.Body).Decode(&changed)
	if s.snapshot(cookie.Value).Count != 0 || changed.ConfirmationToken == "" || !strings.Contains(changed.Reply, "125.00") {
		t.Fatalf("price silently changed: %+v", changed)
	}
	fail.Store(true)
	doJSONRequest(h, "POST", "/api/chat", []byte(`{"message":"да, добавь"}`), cookie)
	if s.snapshot(cookie.Value).Count != 0 {
		t.Fatal("upstream failure added cart item")
	}
	if c.getPending(cookie.Value) != nil {
		t.Fatal("failed confirmation not consumed")
	}
}

func TestZeroAndMissingEKTStockCannotReuseOldStock(t *testing.T) {
	p := testCatalog().Products[0]
	applyProductDetail(&p, models.ProductDetail{ID: p.ID, Article: p.Article, Price: 0, Quantity: 0})
	if p.Price != 0 || p.StockQuantity != 0 || p.Availability != "Нет в наличии" || validateQuantity(p, 1, 0) == "" {
		t.Fatalf("stale facts reused: %+v", p)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id":515291,"article":"200300285_","price":100}`))
	}))
	defer server.Close()
	if _, err := NewEKTAPI(server.URL, "u", "p").ProductDetail(context.Background(), 515291); err == nil {
		t.Fatal("missing stock accepted")
	}
}

func TestUnrelatedInStockProductIsNotAnAnalog(t *testing.T) {
	p := testCatalog().Products[0]
	unrelated := models.Product{ID: 8, Name: "Кабель медный", Article: "CABLE", Availability: "В наличии", StockQuantity: 100}
	if analogSimilarity(p, unrelated) > 0 {
		t.Fatal("availability alone created a recommendation")
	}
}

func TestPrivacyTransportDisablesStorage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["store"] != false {
			t.Error("store:false missing")
		}
		w.Write([]byte(`{}`))
	}))
	defer server.Close()
	r, _ := http.NewRequest("POST", server.URL, bytes.NewBufferString(`{"messages":[],"store":true}`))
	response, err := (privateCompletionTransport{base: http.DefaultTransport}).RoundTrip(r)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
}
