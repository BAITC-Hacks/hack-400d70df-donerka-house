package handlers

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
	htmlnode "golang.org/x/net/html"
)

const defaultEKTCatalogURL = "https://nursultan.ekt.kz"

var liveArticlePattern = regexp.MustCompile(`(?i)(?:код\s+товара|арт\.\s*поставщика)\s*([A-Za-zА-Яа-я0-9_./-]+)`)
var liveIDPattern = regexp.MustCompile(`^bx_[^_]+_(\d+)_`)
var liveNumberPattern = regexp.MustCompile(`\d+(?:[.,]\d+)?`)

// LiveCatalog searches the public EKT regional catalog pages. The public JSON
// API requires separate EKT credentials, so this reader intentionally uses
// the same public search and product pages a browser can access.
type LiveCatalog struct {
	baseURL string
	client  *http.Client
}

func NewLiveCatalog(baseURL string) *LiveCatalog {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultEKTCatalogURL
	}
	return &LiveCatalog{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 8 * time.Second},
	}
}

func (l *LiveCatalog) request(ctx context.Context, target string) (*htmlnode.Node, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "EKT-AI-Assistant/1.0 (+https://ekt.kz)")
	response, err := l.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("EKT page returned HTTP %d", response.StatusCode)
	}
	return htmlnode.Parse(response.Body)
}

// Search uses the public catalog search page and returns current product cards.
func (l *LiveCatalog) Search(ctx context.Context, query string) ([]models.Product, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	searchURL, err := url.Parse(l.baseURL + "/catalog/")
	if err != nil {
		return nil, err
	}
	searchURL.RawQuery = url.Values{"q": []string{query}}.Encode()
	root, err := l.request(ctx, searchURL.String())
	if err != nil {
		return nil, err
	}
	stockLocation := parseLiveCurrentCity(root)

	products := make([]models.Product, 0, 8)
	walk(root, func(node *htmlnode.Node) {
		if len(products) >= 8 || node.Type != htmlnode.ElementNode || node.Data != "div" || !hasClass(node, "product-card-out-catalog") {
			return
		}
		product := parseLiveCard(l.baseURL, node)
		if product.Name != "" && product.URL != "" {
			product.StockLocation = stockLocation
			products = append(products, product)
		}
	})
	return products, nil
}

// Enrich fetches product pages so the model receives descriptions and
// characteristics instead of only names and prices from search results.
func (l *LiveCatalog) Enrich(ctx context.Context, products []models.Product) []models.Product {
	if len(products) > 4 {
		products = products[:4]
	}
	for i := range products {
		if ctx.Err() != nil {
			break
		}
		root, err := l.request(ctx, products[i].URL)
		if err != nil {
			continue
		}
		parseLiveDetail(&products[i], root)
		if products[i].StockLocation == "" {
			products[i].StockLocation = parseLiveCurrentCity(root)
		}
	}
	return products
}

func parseLiveCard(baseURL string, card *htmlnode.Node) models.Product {
	product := models.Product{Source: baseURL}
	if id := attr(card, "id"); id != "" {
		if match := liveIDPattern.FindStringSubmatch(id); len(match) == 2 {
			product.ID, _ = strconv.Atoi(match[1])
		}
	}

	title := findDescendant(card, func(node *htmlnode.Node) bool {
		return node.Type == htmlnode.ElementNode && node.Data == "h3" && hasClass(node, "product-title")
	})
	if title != nil {
		product.Name = cleanText(textContent(title))
		if anchor := ancestorWithTag(title, "a"); anchor != nil {
			product.URL = absoluteURL(baseURL, attr(anchor, "href"))
		}
	}

	imageBox := findDescendant(card, func(node *htmlnode.Node) bool {
		return node.Type == htmlnode.ElementNode && hasClass(node, "product-card-image")
	})
	if imageBox != nil {
		if image := findDescendant(imageBox, func(node *htmlnode.Node) bool { return node.Type == htmlnode.ElementNode && node.Data == "img" }); image != nil {
			product.Image = absoluteURL(baseURL, attr(image, "src"))
		}
	}

	if price := findDescendant(card, func(node *htmlnode.Node) bool { return node.Type == htmlnode.ElementNode && hasClass(node, "price") }); price != nil {
		product.Price = parseLivePrice(textContent(price))
	}
	if article := findDescendant(card, func(node *htmlnode.Node) bool {
		return node.Type == htmlnode.ElementNode && hasClass(node, "product-article") && !hasClass(node, "product-post_article")
	}); article != nil {
		product.Article = parseLiveArticle(textContent(article))
	}
	product.Availability, product.StockQuantity = parseLiveAvailability(card)
	return product
}

func parseLiveDetail(product *models.Product, root *htmlnode.Node) {
	if product.Description == "" {
		descriptionBox := findDescendant(root, func(node *htmlnode.Node) bool {
			return node.Type == htmlnode.ElementNode && hasClass(node, "detail_tabs__body__item") && attr(node, "tab") == "description"
		})
		if descriptionBox != nil {
			if value := findDescendant(descriptionBox, func(node *htmlnode.Node) bool {
				return node.Type == htmlnode.ElementNode && hasClass(node, "detail_tabs__body__item__value")
			}); value != nil {
				product.Description = cleanText(textContent(value))
			}
		}
	}
	if price := findDescendant(root, func(node *htmlnode.Node) bool {
		return node.Type == htmlnode.ElementNode && hasClass(node, "detail_info__price__site__value")
	}); price != nil {
		product.Price = parseLivePrice(textContent(price))
	}
	if meta := findDescendant(root, func(node *htmlnode.Node) bool {
		return node.Type == htmlnode.ElementNode && node.Data == "meta" && attr(node, "property") == "og:image"
	}); meta != nil {
		product.Image = absoluteURL(product.Source, attr(meta, "content"))
	}
	if buttons := findDescendant(root, func(node *htmlnode.Node) bool {
		return node.Type == htmlnode.ElementNode && hasClass(node, "detail_info__buttons")
	}); buttons != nil {
		availability, quantity := parseLiveAvailability(buttons)
		if availability != "" {
			product.Availability = availability
		}
		if quantity > 0 || availability == "Под заказ" {
			product.StockQuantity = quantity
		}
	}
	if product.StockLocation == "" {
		product.StockLocation = parseLiveCurrentCity(root)
	}

	if product.Properties == nil {
		product.Properties = make(map[string]string)
	}
	walk(root, func(node *htmlnode.Node) {
		if node.Type != htmlnode.ElementNode || !hasClass(node, "tab_item_chars__item") {
			return
		}
		nameNode := findDescendant(node, func(child *htmlnode.Node) bool {
			return child.Type == htmlnode.ElementNode && hasClass(child, "tab_item_chars__item__name")
		})
		valueNode := findDescendant(node, func(child *htmlnode.Node) bool {
			return child.Type == htmlnode.ElementNode && hasClass(child, "tab_item_chars__item__value")
		})
		name, value := cleanText(textContent(nameNode)), cleanText(textContent(valueNode))
		if name != "" && value != "" {
			product.Properties[strings.TrimSuffix(name, ":")] = value
		}
	})
	if product.Article == "" {
		if article, ok := product.Properties["Артикул"]; ok {
			product.Article = article
		}
	}
}

// parseLiveAvailability reads the availability controls rendered by EKT's
// public catalog. The page uses add2basket for regional stock and
// add2basketPreOrder for products that are not currently in that region.
func parseLiveAvailability(root *htmlnode.Node) (string, int) {
	if root == nil {
		return "", 0
	}
	action := ""
	walk(root, func(node *htmlnode.Node) {
		if action != "" || node.Type != htmlnode.ElementNode {
			return
		}
		candidate := attr(node, "data-action")
		if candidate == "add2basket" || candidate == "add2basketPreOrder" {
			action = candidate
		}
	})

	quantity := 0
	if input := findDescendant(root, func(node *htmlnode.Node) bool {
		return node.Type == htmlnode.ElementNode && node.Data == "input" && hasClass(node, "tq_quantity")
	}); input != nil {
		quantity, _ = strconv.Atoi(attr(input, "max"))
		if quantity < 0 {
			quantity = 0
		}
	}

	switch action {
	case "add2basket":
		return "В наличии", quantity
	case "add2basketPreOrder":
		return "Под заказ", 0
	}

	text := strings.ToLower(cleanText(textContent(root)))
	switch {
	case strings.Contains(text, "под заказ"):
		return "Под заказ", 0
	case strings.Contains(text, "в наличии"):
		return "В наличии", quantity
	}
	return "", quantity
}

func parseLiveCurrentCity(root *htmlnode.Node) string {
	city := findDescendant(root, func(node *htmlnode.Node) bool {
		return node.Type == htmlnode.ElementNode && hasClass(node, "select-city__block__text-city")
	})
	if city != nil {
		return cleanText(textContent(city))
	}
	city = findDescendant(root, func(node *htmlnode.Node) bool {
		return node.Type == htmlnode.ElementNode && node.Data == "span" && attr(node, "id") == "select-city__js"
	})
	return cleanText(textContent(city))
}

func parseLiveArticle(value string) string {
	if match := liveArticlePattern.FindStringSubmatch(cleanText(value)); len(match) == 2 {
		return match[1]
	}
	return cleanText(value)
}

func parseLivePrice(value string) float64 {
	match := liveNumberPattern.FindString(strings.ReplaceAll(cleanText(value), " ", ""))
	if match == "" {
		return 0
	}
	match = strings.ReplaceAll(match, ",", ".")
	price, _ := strconv.ParseFloat(match, 64)
	return price
}

func absoluteURL(baseURL, path string) string {
	if path == "" {
		return ""
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return path
	}
	parsed, err := url.Parse(html.UnescapeString(strings.TrimSpace(path)))
	if err != nil {
		return path
	}
	return base.ResolveReference(parsed).String()
}

func cleanText(value string) string {
	value = html.UnescapeString(value)
	return strings.Join(strings.Fields(value), " ")
}

func walk(node *htmlnode.Node, visit func(*htmlnode.Node)) {
	if node == nil {
		return
	}
	visit(node)
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		walk(child, visit)
	}
}

func findDescendant(node *htmlnode.Node, match func(*htmlnode.Node) bool) *htmlnode.Node {
	var found *htmlnode.Node
	walk(node, func(candidate *htmlnode.Node) {
		if found == nil && candidate != node && match(candidate) {
			found = candidate
		}
	})
	return found
}

func ancestorWithTag(node *htmlnode.Node, tag string) *htmlnode.Node {
	for parent := node.Parent; parent != nil; parent = parent.Parent {
		if parent.Type == htmlnode.ElementNode && parent.Data == tag {
			return parent
		}
	}
	return nil
}

func hasClass(node *htmlnode.Node, class string) bool {
	for _, candidate := range strings.Fields(attr(node, "class")) {
		if candidate == class {
			return true
		}
	}
	return false
}

func attr(node *htmlnode.Node, name string) string {
	if node == nil {
		return ""
	}
	for _, attribute := range node.Attr {
		if attribute.Key == name {
			return attribute.Val
		}
	}
	return ""
}

func textContent(node *htmlnode.Node) string {
	if node == nil {
		return ""
	}
	if node.Type == htmlnode.TextNode {
		return node.Data
	}
	var builder strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		builder.WriteString(textContent(child))
		builder.WriteByte(' ')
	}
	return builder.String()
}
