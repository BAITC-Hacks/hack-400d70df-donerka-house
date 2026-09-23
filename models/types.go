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
	ID          int                    `json:"id"`
	Name        string                 `json:"name"`
	Article     string                 `json:"article"`
	Description string                 `json:"description"`
	Price       float64                `json:"price"`
	Quantity    int                    `json:"quantity"`
	Stores      []Store                `json:"stores"`
	Image       string                 `json:"image"`
	URL         string                 `json:"url"`
	Properties  map[string]interface{} `json:"properties"`
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
	seen := make(map[string]struct{}, len(c.Products)+len(products))
	for _, product := range c.Products {
		seen[strings.ToLower(product.Article)+"|"+strings.ToLower(product.URL)] = struct{}{}
	}
	for _, product := range products {
		key := strings.ToLower(product.Article) + "|" + strings.ToLower(product.URL)
		if product.Name != "" && product.URL != "" {
			if _, exists := seen[key]; !exists {
				c.Products = append(c.Products, product)
				seen[key] = struct{}{}
			}
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

// CartRequest is used to add a catalog product to the current session cart.
type CartRequest struct {
	Article  string `json:"article"`
	Quantity int    `json:"quantity"`
}

// ChatResponse is the AI reply + optional matched products and cart actions
type ChatResponse struct {
	Reply      string          `json:"reply"`
	Products   []ProductResult `json:"products,omitempty"`
	CartAction *CartAction     `json:"cart_action,omitempty"`
	Cart       *CartResponse   `json:"cart,omitempty"`
}

// ErrorResponse is a standard error
type ErrorResponse struct {
	Error string `json:"error"`
}
