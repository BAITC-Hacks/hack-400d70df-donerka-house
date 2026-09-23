package models

// MenuItem represents a single item on the menu
type MenuItem struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Weight      string  `json:"weight"`
	Spicy       bool    `json:"spicy"`
}

// MenuCategory represents a category of menu items
type MenuCategory struct {
	ID    string     `json:"id"`
	Name  string     `json:"name"`
	Items []MenuItem `json:"items"`
}

// RestaurantInfo holds general restaurant information
type RestaurantInfo struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Address      string `json:"address"`
	Phone        string `json:"phone"`
	WorkingHours string `json:"working_hours"`
}

// Menu is the full menu structure from JSON
type Menu struct {
	Restaurant RestaurantInfo `json:"restaurant"`
	Categories []MenuCategory `json:"categories"`
}

// ChatRequest is the incoming chat message from the user
type ChatRequest struct {
	Message string `json:"message"`
}

// ChatResponse is the AI reply sent back to the user
type ChatResponse struct {
	Reply string `json:"reply"`
}

// ErrorResponse is a standard error response
type ErrorResponse struct {
	Error string `json:"error"`
}
