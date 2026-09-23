package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
)

func testCatalog() *models.Catalog {
	return &models.Catalog{Products: []models.Product{{
		ID:      515291,
		Name:    "027228 АВ DRX250 MT 3ф 160А Legrand",
		Article: "200300285_",
		Price:   64920,
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

func TestDemoChatAddsExplicitProductRequest(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	store := NewCartStore()
	handler := ChatHandler(testCatalog(), store)
	response := doJSONRequest(handler, http.MethodPost, "/api/chat", []byte(`{"message":"Добавь в корзину 027228, 2 шт."}`), nil)
	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	var chat models.ChatResponse
	if err := json.NewDecoder(response.Body).Decode(&chat); err != nil {
		t.Fatal(err)
	}
	if chat.CartAction == nil || chat.CartAction.Article != "200300285_" || chat.CartAction.Quantity != 2 {
		t.Fatalf("expected validated cart action, got %+v", chat.CartAction)
	}
	if chat.Cart == nil || chat.Cart.Count != 2 {
		t.Fatalf("expected cart summary, got %+v", chat.Cart)
	}
}
