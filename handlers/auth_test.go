package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
)

func cookieByName(t *testing.T, response *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("expected %s cookie", name)
	return nil
}

func requestWithCookies(handler http.Handler, method, target string, body []byte, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, bytes.NewReader(body))
	for _, cookie := range cookies {
		if cookie != nil {
			request.AddCookie(cookie)
		}
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestAccountOwnsCartAcrossLogoutAndLogin(t *testing.T) {
	auth, err := NewAuthStore(filepath.Join(t.TempDir(), "accounts.json"))
	if err != nil {
		t.Fatal(err)
	}
	carts := NewCartStore()
	catalog := testCatalog()
	authHandler := AuthHandler(auth, carts)
	cartHandler := CartHandlerWithAuth(catalog, carts, auth)

	guestCart := requestWithCookies(cartHandler, http.MethodGet, "/api/cart", nil)
	if guestCart.Code != http.StatusOK {
		t.Fatalf("guest cart status = %d", guestCart.Code)
	}
	session := cookieByName(t, guestCart, sessionCookieName)
	carts.add(session.Value, catalog.Products[0], 2)

	register := requestWithCookies(authHandler, http.MethodPost, "/api/auth", []byte(`{"action":"register","email":"buyer@example.com","name":"Buyer","password":"password123","merge_cart":true}`), session)
	if register.Code != http.StatusOK {
		t.Fatalf("register status = %d: %s", register.Code, register.Body.String())
	}
	authCookie := cookieByName(t, register, authCookieName)

	accountCart := requestWithCookies(cartHandler, http.MethodGet, "/api/cart", nil, session, authCookie)
	var cart models.CartResponse
	if err := json.NewDecoder(accountCart.Body).Decode(&cart); err != nil {
		t.Fatal(err)
	}
	if cart.Count != 2 {
		t.Fatalf("guest cart was not moved to account: %+v", cart)
	}

	logout := requestWithCookies(authHandler, http.MethodDelete, "/api/auth", nil, session, authCookie)
	if logout.Code != http.StatusOK {
		t.Fatalf("logout status = %d", logout.Code)
	}
	login := requestWithCookies(authHandler, http.MethodPost, "/api/auth", []byte(`{"action":"login","email":"buyer@example.com","password":"password123"}`), session)
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d: %s", login.Code, login.Body.String())
	}
	newAuthCookie := cookieByName(t, login, authCookieName)

	accountCart = requestWithCookies(cartHandler, http.MethodGet, "/api/cart", nil, session, newAuthCookie)
	if err := json.NewDecoder(accountCart.Body).Decode(&cart); err != nil {
		t.Fatal(err)
	}
	if cart.Count != 2 || len(cart.Items) != 1 {
		t.Fatalf("account cart was not retained after re-login: %+v", cart)
	}
}
