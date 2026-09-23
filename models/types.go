package models

// Product represents a single product from the catalog
type Product struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Article      string  `json:"article"`
	Price        float64 `json:"price"`
	Image        string  `json:"image"`
	URL          string  `json:"url"`
	URLAPIDetail string  `json:"url_api_detail"`
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
}

// ChatRequest is the incoming user message
type ChatRequest struct {
	Message string `json:"message"`
}

// ProductResult is a matched product returned alongside the AI reply
type ProductResult struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	Article string  `json:"article"`
	Price   float64 `json:"price"`
	Image   string  `json:"image"`
	URL     string  `json:"url"`
}

// ChatResponse is the AI reply + optional matched products
type ChatResponse struct {
	Reply    string          `json:"reply"`
	Products []ProductResult `json:"products,omitempty"`
}

// ErrorResponse is a standard error
type ErrorResponse struct {
	Error string `json:"error"`
}
