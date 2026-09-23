package models

// Product represents a single product from the catalog.
type Product struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	Article      string  `json:"article"`
	Price        float64 `json:"price"`
	Image        string  `json:"image"`
	URL          string  `json:"url"`
	URLAPIDetail string  `json:"url_api_detail"`
}

// ProductsPage represents a paginated list of products.
type ProductsPage struct {
	Page    int       `json:"page"`
	PerPage int       `json:"per_page"`
	Count   int       `json:"count"`
	Items   []Product `json:"items"`
}

// Store represents stock availability in a specific city/store.
type Store struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

// Certificate is a link to a product certificate or a clearly marked demo link.
type Certificate struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	IsDemo bool   `json:"is_demo"`
}

// ProductDetail represents detailed product information.
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

// PurchaseTerms contains the currently configured payment and delivery terms.
type PurchaseTerms struct {
	Payment      []string `json:"payment"`
	Delivery     string   `json:"delivery"`
	MinimumOrder string   `json:"minimum_order"`
	Returns      string   `json:"returns"`
}

// Catalog holds product summaries, optional detailed records, certificates, and terms.
type Catalog struct {
	Products     []Product                 `json:"products"`
	Details      map[int]ProductDetail     `json:"details"`
	Certificates map[int][]Certificate     `json:"certificates"`
	Terms        PurchaseTerms             `json:"terms"`
}

func (c *Catalog) DetailFor(id int) (*ProductDetail, bool) {
	detail, ok := c.Details[id]
	if !ok {
		return nil, false
	}
	return &detail, true
}

// ChatMessage is a persisted conversation turn for one anonymous browser session.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CartItem is a product explicitly added after confirmation.
type CartItem struct {
	ProductID int     `json:"product_id"`
	Article   string  `json:"article"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
}

// PendingCart describes an item waiting for explicit user confirmation.
type PendingCart struct {
	ProductID int     `json:"product_id"`
	Article   string  `json:"article"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
	Stock     int     `json:"stock"`
	Total     float64 `json:"total"`
}

// CartResponse is returned by the cart API and by the chat endpoint after a cart change.
type CartResponse struct {
	SessionID string     `json:"session_id"`
	Items     []CartItem `json:"items"`
	Total     float64    `json:"total"`
}

// ChatRequest is the incoming user message.
type ChatRequest struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
}

// ChatResponse is the AI reply.
type ChatResponse struct {
	Reply     string       `json:"reply"`
	SessionID string       `json:"session_id"`
	CartURL   string       `json:"cart_url,omitempty"`
	CartItems []CartItem   `json:"cart_items,omitempty"`
	Pending   *PendingCart `json:"pending,omitempty"`
}

// ErrorResponse is a standard error.
type ErrorResponse struct {
	Error string `json:"error"`
}
