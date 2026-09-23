package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
)

func testCatalog() *models.Catalog {
	return &models.Catalog{Products: []models.Product{{
		ID:      515291,
		Name:    "027228 АВ DRX250 MT 3ф 160А Legrand",
		Article: "200300285_",
		Price:         64920,
		Availability:  "В наличии",
		StockQuantity: 5,
	}}}
}

func doJSONRequest(handler http.Handler, method, target string, body []byte, cookie *http.Cookie) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, bytes.NewReader(body))
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func sessionCookie(t *testing.T, response *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			return cookie
		}
	}
	t.Fatal("expected a session cookie")
	return nil
}

func TestCartHandlerReadsSessionCart(t *testing.T) {
	store := NewCartStore()
	handler := CartHandler(testCatalog(), store)

	initial := doJSONRequest(handler, http.MethodGet, "/api/cart", nil, nil)
	cookie := sessionCookie(t, initial)
	product := testCatalog().Products[0]
	store.add(cookie.Value, product, 2)

	readResponse := doJSONRequest(handler, http.MethodGet, "/api/cart", nil, cookie)
	var readCart models.CartResponse
	if err := json.NewDecoder(readResponse.Body).Decode(&readCart); err != nil {
		t.Fatal(err)
	}
	if readCart.Count != 2 || readCart.Total != 129840 || len(readCart.Items) != 1 || readCart.Items[0].Article != "200300285_" {
		t.Fatalf("cart was not retained in session: %+v", readCart)
	}
}

func TestCartHandlerRejectsDirectAddWithoutChatConfirmation(t *testing.T) {
	handler := CartHandler(testCatalog(), NewCartStore())
	response := doJSONRequest(handler, http.MethodPost, "/api/cart", []byte(`{"article":"200300285_","quantity":1}`), nil)
	if response.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", response.Code, response.Body.String())
	}
}

func TestCartSessionsAreIsolated(t *testing.T) {
	handler := CartHandler(testCatalog(), NewCartStore())
	first := doJSONRequest(handler, http.MethodGet, "/api/cart", nil, nil)
	second := doJSONRequest(handler, http.MethodGet, "/api/cart", nil, nil)
	firstCookie := sessionCookie(t, first)
	secondCookie := sessionCookie(t, second)
	if firstCookie.Value == secondCookie.Value {
		t.Fatal("expected separate browser sessions")
	}
}

func TestDemoChatRequiresConfirmationBeforeAdding(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	store := NewCartStore()
	handler := ChatHandler(testCatalog(), store, nil, NewConversationStore())

	first := doJSONRequest(handler, http.MethodPost, "/api/chat", []byte(`{"message":"Добавь в корзину 027228, 2 шт."}`), nil)
	if first.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", first.Code)
	}
	cookie := sessionCookie(t, first)
	var pending models.ChatResponse
	if err := json.NewDecoder(first.Body).Decode(&pending); err != nil {
		t.Fatal(err)
	}
	if pending.CartAction != nil || pending.Cart != nil {
		t.Fatalf("cart must not change before confirmation: %+v", pending)
	}

	confirm := doJSONRequest(handler, http.MethodPost, "/api/chat", []byte(`{"message":"да, добавь"}`), cookie)
	var confirmed models.ChatResponse
	if err := json.NewDecoder(confirm.Body).Decode(&confirmed); err != nil {
		t.Fatal(err)
	}
	if confirmed.CartAction == nil || confirmed.CartAction.Article != "200300285_" || confirmed.CartAction.Quantity != 2 {
		t.Fatalf("expected confirmed cart action, got %+v", confirmed.CartAction)
	}
	if confirmed.Cart == nil || confirmed.Cart.Count != 2 || confirmed.CartURL != "/#cart" {
		t.Fatalf("expected current cart and direct link, got %+v", confirmed)
	}
}

func TestChatDoesNotAddBeforeConfirmationEvenWithAPIKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "test-key-not-used")
	store := NewCartStore()
	handler := ChatHandler(testCatalog(), store, nil, NewConversationStore())
	response := doJSONRequest(handler, http.MethodPost, "/api/chat", []byte(`{"message":"Купи 3 штуки товара 027228"}`), nil)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	var chat models.ChatResponse
	if err := json.NewDecoder(response.Body).Decode(&chat); err != nil {
		t.Fatal(err)
	}
	if chat.CartAction != nil || chat.Cart != nil {
		t.Fatalf("expected pending confirmation only, got %+v", chat)
	}
}


func TestChatRejectsQuantityAboveStock(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	handler := ChatHandler(testCatalog(), NewCartStore(), nil, NewConversationStore())
	response := doJSONRequest(handler, http.MethodPost, "/api/chat", []byte(`{"message":"Добавь 6 шт товара 027228"}`), nil)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var chat models.ChatResponse
	if err := json.NewDecoder(response.Body).Decode(&chat); err != nil {
		t.Fatal(err)
	}
	if chat.CartAction != nil || chat.Cart != nil {
		t.Fatalf("cart must not change when requested quantity exceeds stock: %+v", chat)
	}
	if !strings.Contains(chat.Reply, "Доступно только 5 шт") {
		t.Fatalf("expected stock-limit explanation, got %q", chat.Reply)
	}
}

func TestPurchaseTermsAnswerIncludesMinimumBatch(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	handler := ChatHandler(testCatalog(), NewCartStore(), nil, NewConversationStore())
	response := doJSONRequest(handler, http.MethodPost, "/api/chat", []byte(`{"message":"Какие условия оплаты, доставки и минимальная партия?"}`), nil)
	var chat models.ChatResponse
	if err := json.NewDecoder(response.Body).Decode(&chat); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(chat.Reply, "минимальная партия") || !strings.Contains(chat.Reply, "1 шт") {
		t.Fatalf("expected purchase terms with minimum batch, got %q", chat.Reply)
	}
}


func TestBareYesDoesNotConfirmPendingCart(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	store := NewCartStore()
	conversations := NewConversationStore()
	handler := ChatHandler(testCatalog(), store, nil, conversations)

	first := doJSONRequest(handler, http.MethodPost, "/api/chat", []byte(`{"message":"Добавь 2 шт товара 027228"}`), nil)
	cookie := sessionCookie(t, first)

	second := doJSONRequest(handler, http.MethodPost, "/api/chat", []byte(`{"message":"да"}`), cookie)
	var chat models.ChatResponse
	if err := json.NewDecoder(second.Body).Decode(&chat); err != nil {
		t.Fatal(err)
	}
	if chat.CartAction != nil || chat.Cart != nil {
		t.Fatalf("bare yes must not change cart: %+v", chat)
	}

	cartResponse := doJSONRequest(CartHandler(testCatalog(), store), http.MethodGet, "/api/cart", nil, cookie)
	var cart models.CartResponse
	if err := json.NewDecoder(cartResponse.Body).Decode(&cart); err != nil {
		t.Fatal(err)
	}
	if cart.Count != 0 {
		t.Fatalf("expected empty cart after bare yes, got %+v", cart)
	}
}


func TestFindAnalogsForUnavailableProduct(t *testing.T) {
	target := models.Product{
		ID:                 10,
		Name:               "Автоматический выключатель Legrand 3P 80A",
		Article:            "OUT-80A",
		Availability:       "Нет в наличии",
		TotalStockQuantity: 0,
		Properties: map[string]string{
			"Полюсов": "3",
			"Ток":     "80 A",
		},
	}
	analog := models.Product{
		ID:                 11,
		Name:               "Автоматический выключатель Schneider 3P 80A",
		Article:            "ALT-80A",
		Availability:       "В наличии",
		StockQuantity:      6,
		TotalStockQuantity: 6,
		Properties: map[string]string{
			"Полюсов": "3",
			"Ток":     "80 A",
		},
	}
	catalog := &models.Catalog{Products: []models.Product{target, analog}}

	analogs := findAnalogs(catalog, target, 3)
	if len(analogs) == 0 {
		t.Fatal("expected at least one analog for unavailable product")
	}
	if analogs[0].Article != "ALT-80A" {
		t.Fatalf("expected ALT-80A as analog, got %+v", analogs)
	}
	reason := analogReason(target, analogs[0])
	if !strings.Contains(reason, "характерист") {
		t.Fatalf("expected short analog explanation, got %q", reason)
	}
}

func TestDemoChatSuggestsAnalogForUnavailableProduct(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	target := models.Product{
		ID:                 10,
		Name:               "Автоматический выключатель Legrand 3P 80A",
		Article:            "OUT-80A",
		Availability:       "Нет в наличии",
		TotalStockQuantity: 0,
		Properties: map[string]string{
			"Полюсов": "3",
			"Ток":     "80 A",
		},
	}
	analog := models.Product{
		ID:                 11,
		Name:               "Автоматический выключатель Schneider 3P 80A",
		Article:            "ALT-80A",
		Availability:       "В наличии",
		StockQuantity:      6,
		Properties: map[string]string{
			"Полюсов": "3",
			"Ток":     "80 A",
		},
	}
	catalog := &models.Catalog{Products: []models.Product{target, analog}}
	handler := ChatHandler(catalog, NewCartStore(), nil, NewConversationStore())

	response := doJSONRequest(handler, http.MethodPost, "/api/chat", []byte(`{"message":"OUT-80A"}`), nil)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var chat models.ChatResponse
	if err := json.NewDecoder(response.Body).Decode(&chat); err != nil {
		t.Fatal(err)
	}
	if len(chat.Analogs) == 0 || chat.Analogs[0].Article != "ALT-80A" {
		t.Fatalf("expected analog in chat response, got %+v", chat.Analogs)
	}
	if !strings.Contains(chat.Reply, "Подходящие аналоги") {
		t.Fatalf("expected analog explanation in reply, got %q", chat.Reply)
	}
}
