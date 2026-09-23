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

func TestCartHandlerAddsAndReadsSessionCart(t *testing.T) {
	store := NewCartStore()
	handler := CartHandler(testCatalog(), store)

	addResponse := doJSONRequest(handler, http.MethodPost, "/api/cart", []byte(`{"article":"027228","quantity":2}`), nil)
	if addResponse.Code != http.StatusOK {
		t.Fatalf("expected add status 200, got %d: %s", addResponse.Code, addResponse.Body.String())
	}
	cookie := sessionCookie(t, addResponse)
	var cart models.CartResponse
	if err := json.NewDecoder(addResponse.Body).Decode(&cart); err != nil {
		t.Fatal(err)
	}
	if cart.Count != 2 || cart.Total != 129840 || len(cart.Items) != 1 {
		t.Fatalf("unexpected cart after add: %+v", cart)
	}

	readResponse := doJSONRequest(handler, http.MethodGet, "/api/cart", nil, cookie)
	var readCart models.CartResponse
	if err := json.NewDecoder(readResponse.Body).Decode(&readCart); err != nil {
		t.Fatal(err)
	}
	if readCart.Count != 2 || readCart.Items[0].Article != "200300285_" {
		t.Fatalf("cart was not retained in session: %+v", readCart)
	}
}

func TestCartHandlerRejectsInvalidQuantity(t *testing.T) {
	handler := CartHandler(testCatalog(), NewCartStore())
	response := doJSONRequest(handler, http.MethodPost, "/api/cart", []byte(`{"article":"200300285_","quantity":0}`), nil)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", response.Code)
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


func TestCartHandlerRejectsQuantityAboveStock(t *testing.T) {
	handler := CartHandler(testCatalog(), NewCartStore())
	response := doJSONRequest(handler, http.MethodPost, "/api/cart", []byte(`{"article":"200300285_","quantity":6}`), nil)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for quantity above stock, got %d: %s", response.Code, response.Body.String())
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
