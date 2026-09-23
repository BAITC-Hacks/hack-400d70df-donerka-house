package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
)

const defaultEKTAPIBaseURL = "https://ekt.kz/api"

// EKTAPI is the authenticated EKT catalog client. Credentials are read from
// environment variables by main.go and are never sent to the OpenAI API.
type EKTAPI struct {
	baseURL  string
	username string
	password string
	client   *http.Client
}

func NewEKTAPI(baseURL, username, password string) *EKTAPI {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultEKTAPIBaseURL
	}
	return &EKTAPI{
		baseURL:  strings.TrimRight(baseURL, "/"),
		username: username,
		password: password,
		client:   &http.Client{Timeout: 12 * time.Second},
	}
}

func (a *EKTAPI) Enabled() bool {
	return a != nil && strings.TrimSpace(a.username) != "" && strings.TrimSpace(a.password) != ""
}

func (a *EKTAPI) requestJSON(ctx context.Context, endpoint string, query url.Values, target any) error {
	if !a.Enabled() {
		return errors.New("EKT API credentials are not configured")
	}
	requestURL, err := url.Parse(a.baseURL + "/" + strings.TrimLeft(endpoint, "/"))
	if err != nil {
		return err
	}
	requestURL.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return err
	}
	request.SetBasicAuth(a.username, a.password)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "EKT-AI-Assistant/1.0")

	response, err := a.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("EKT API returned HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return fmt.Errorf("decode EKT API response: %w", err)
	}
	return nil
}

func (a *EKTAPI) loadProductsPage(ctx context.Context, page int) (models.ProductsPage, error) {
	query := url.Values{}
	if page > 1 {
		query.Set("page", strconv.Itoa(page))
	}
	var result models.ProductsPage
	if err := a.requestJSON(ctx, "products", query, &result); err != nil {
		return models.ProductsPage{}, err
	}
	return result, nil
}

// LoadProducts reads the paginated authenticated catalog until the API returns
// an empty or short page. Some EKT responses use count as page size, so the
// client does not rely on count alone to decide when pagination is complete.
func (a *EKTAPI) LoadProducts(ctx context.Context) ([]models.Product, error) {
	if !a.Enabled() {
		return nil, errors.New("EKT API credentials are not configured")
	}
	products := make([]models.Product, 0, 128)
	seen := make(map[int]struct{})
	for page := 1; page <= 5000; page++ {
		result, err := a.loadProductsPage(ctx, page)
		if err != nil {
			return nil, fmt.Errorf("load EKT products page %d: %w", page, err)
		}
		if len(result.Items) == 0 {
			break
		}
		for _, product := range result.Items {
			if product.ID == 0 {
				continue
			}
			if _, exists := seen[product.ID]; exists {
				continue
			}
			seen[product.ID] = struct{}{}
			products = append(products, product)
		}
		if result.PerPage > 0 && len(result.Items) < result.PerPage {
			break
		}
	}
	if len(products) == 0 {
		return nil, errors.New("EKT API returned no products")
	}
	return products, nil
}

func (a *EKTAPI) ProductDetail(ctx context.Context, id int) (models.ProductDetail, error) {
	if id <= 0 {
		return models.ProductDetail{}, errors.New("invalid EKT product id")
	}
	var detail models.ProductDetail
	if err := a.requestJSON(ctx, "products/detail", url.Values{"id": []string{strconv.Itoa(id)}}, &detail); err != nil {
		return models.ProductDetail{}, err
	}
	return detail, nil
}

// Enrich fetches authenticated details for the first four relevant products.
// The limit keeps chat latency bounded while still providing exact quantity,
// store and characteristic data for the products shown to the user.
func (a *EKTAPI) Enrich(ctx context.Context, products []models.Product) []models.Product {
	if !a.Enabled() {
		return products
	}
	if len(products) > 4 {
		products = products[:4]
	}
	for index := range products {
		if ctx.Err() != nil || products[index].ID <= 0 {
			break
		}
		detail, err := a.ProductDetail(ctx, products[index].ID)
		if err != nil {
			continue
		}
		applyProductDetail(&products[index], detail)
	}
	return products
}

func applyProductDetail(product *models.Product, detail models.ProductDetail) {
	if detail.ID != 0 {
		product.ID = detail.ID
	}
	if detail.Name != "" {
		product.Name = detail.Name
	}
	if detail.Article != "" {
		product.Article = detail.Article
	}
	if detail.Description != "" {
		product.Description = detail.Description
	}
	if detail.Price > 0 {
		product.Price = detail.Price
	}
	if detail.Image != "" {
		product.Image = detail.Image
	}
	if detail.URL != "" {
		product.URL = detail.URL
	}
	product.TotalStockQuantity = detail.Quantity
	if len(detail.Stores) > 0 {
		product.Stores = append([]models.Store(nil), detail.Stores...)
	}
	if detail.Quantity > 0 {
		product.Availability = "В наличии"
	} else if product.Availability == "" {
		product.Availability = "Нет в наличии"
	}
	if product.Properties == nil {
		product.Properties = make(map[string]string)
	}
	for name, value := range detail.Properties {
		product.Properties[name] = stringifyProperty(value)
	}
}

func stringifyProperty(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	encoded, err := json.Marshal(value)
	if err == nil {
		return string(encoded)
	}
	return fmt.Sprint(value)
}
