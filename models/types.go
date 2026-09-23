package models

import (
	"strings"
	"sync"
)

// Product represents a single product from the catalog
type Product struct {
	ID                 int               `json:"id"`
	Name               string            `json:"name"`
	Article            string            `json:"article"`
	Price              float64           `json:"price"`
	Image              string            `json:"image"`
	URL                string            `json:"url"`
	URLAPIDetail       string            `json:"url_api_detail"`
	Description        string            `json:"description,omitempty"`
	Properties         map[string]string `json:"properties,omitempty"`
	Source             string            `json:"source,omitempty"`
	Availability       string            `json:"availability,omitempty"`
	StockQuantity      int               `json:"stock_quantity,omitempty"`
	StockLocation      string            `json:"stock_location,omitempty"`
	TotalStockQuantity int               `json:"total_stock_quantity,omitempty"`
	Stores             []Store           `json:"stores,omitempty"`
	Certificates       []string          `json:"certificates,omitempty"`
}

// ProductsPage represents a paginated list of products
type ProductsPage struct {
	Page    int       `json:"page"`
	PerPage int       `json:"per_page"`
	Count   int       `json:"count"`
	Items   []Product `json:"items"`
}

// Store represents stock availability in a specific city/store
type Store struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

// ProductDetail represents detailed product information
type ProductDetail struct {
	ID           int                    `json:"id"`
	Name         string                 `json:"name"`
	Article      string                 `json:"article"`
	Description  string                 `json:"description"`
	Price        float64                `json:"price"`
	Quantity     int                    `json:"quantity"`
	Stores       []Store                `json:"stores"`
	Certificates []string               `json:"certificates,omitempty"`
	Image        string                 `json:"image"`
	URL          string                 `json:"url"`
	Properties   map[string]interface{} `json:"properties"`
}

// Catalog holds all loaded product data for the AI context
type Catalog struct {
	Products []Product
	Detail   *ProductDetail // example detail record
	mu       sync.RWMutex
}

func (c *Catalog) ProductsSnapshot() []Product {
	c.mu.RLock()
	defer c.mu.RUnlock()
	products := make([]Product, len(c.Products))
	copy(products, c.Products)
	return products
}

func (c *Catalog) AddProducts(products []Product) {
	if len(products) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, incoming := range products {
		if incoming.Name == "" {
			continue
		}

		found := -1
		for i := range c.Products {
			sameID := incoming.ID != 0 && c.Products[i].ID == incoming.ID
			sameArticle := incoming.Article != "" && strings.EqualFold(c.Products[i].Article, incoming.Article)
			sameURL := incoming.URL != "" && strings.EqualFold(c.Products[i].URL, incoming.URL)
			if sameID || sameArticle || sameURL {
				found = i
				break
			}
		}

		if found == -1 {
			c.Products = append(c.Products, incoming)
			continue
		}

		current := &c.Products[found]
		if incoming.Name != "" {
			current.Name = incoming.Name
		}
		if incoming.Article != "" {
			current.Article = incoming.Article
		}
		if incoming.Price > 0 {
			current.Price = incoming.Price
		}
		if incoming.Image != "" {
			current.Image = incoming.Image
		}
		if incoming.URL != "" {
			current.URL = incoming.URL
		}
		if incoming.URLAPIDetail != "" {
			current.URLAPIDetail = incoming.URLAPIDetail
		}
		if incoming.Description != "" {
			current.Description = incoming.Description
		}
		if incoming.Properties != nil {
			current.Properties = incoming.Properties
		}
		if incoming.Source != "" {
			current.Source = incoming.Source
		}
		if incoming.Availability != "" {
			current.Availability = incoming.Availability
			current.StockQuantity = incoming.StockQuantity
		}
		if incoming.StockLocation != "" {
			current.StockLocation = incoming.StockLocation
		}
		if incoming.TotalStockQuantity > 0 || len(incoming.Stores) > 0 {
			current.TotalStockQuantity = incoming.TotalStockQuantity
			current.Stores = append([]Store(nil), incoming.Stores...)
		}
		if len(incoming.Certificates) > 0 {
			current.Certificates = append([]string(nil), incoming.Certificates...)
		}
	}
}

func (c *Catalog) ProductCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.Products)
}

// ChatRequest is the incoming user message
type ChatRequest struct {
	Message string `json:"message"`
}

// ProductResult is a matched product returned alongside the AI reply
type ProductResult struct {
	ID                 int               `json:"id"`
	Name               string            `json:"name"`
	Article            string            `json:"article"`
	Price              float64           `json:"price"`
	Image              string            `json:"image"`
	URL                string            `json:"url"`
	Description        string            `json:"description,omitempty"`
	Properties         map[string]string `json:"properties,omitempty"`
	Source             string            `json:"source,omitempty"`
	Availability       string            `json:"availability,omitempty"`
	StockQuantity      int               `json:"stock_quantity,omitempty"`
	StockLocation      string            `json:"stock_location,omitempty"`
	TotalStockQuantity int               `json:"total_stock_quantity,omitempty"`
	Stores             []Store           `json:"stores,omitempty"`
	Certificates       []string          `json:"certificates,omitempty"`
}

// CartAction represents a directive to the frontend to add an item to the cart
type CartAction struct {
	Article  string  `json:"article"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

// CartItem is a validated product currently in a user's session cart.
type CartItem struct {
	Article  string  `json:"article"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

// CartResponse is returned by the cart API and the chat API after a cart change.
type CartResponse struct {
	Items []CartItem `json:"items"`
	Count int        `json:"count"`
	Total float64    `json:"total"`
}

// User is the safe public representation of a local assistant account.
// Password material is never exposed in API responses.
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// AuthRequest is used by the local account API.
type AuthRequest struct {
	Action   string `json:"action"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

// AuthResponse describes the current local login state.
type AuthResponse struct {
	Authenticated bool  `json:"authenticated"`
	User          *User `json:"user,omitempty"`
}

// CartRequest is used to add a catalog product to the current session cart.
type CartRequest struct {
	Article  string `json:"article"`
	Quantity int    `json:"quantity"`
}

// ChatResponse is the AI reply + optional matched products and cart actions
type ChatResponse struct {
	Reply      string          `json:"reply"`
	Products   []ProductResult `json:"products,omitempty"`
	Analogs    []ProductResult `json:"analogs,omitempty"`
	CartAction *CartAction     `json:"cart_action,omitempty"`
	Cart       *CartResponse   `json:"cart,omitempty"`
	CartURL    string          `json:"cart_url,omitempty"`
}

// ErrorResponse is a standard error
type ErrorResponse struct {
	Error string `json:"error"`
}
