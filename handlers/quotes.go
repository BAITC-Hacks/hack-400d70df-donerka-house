package handlers

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
)

func freshProduct(p models.Product) bool {
	return !p.VerifiedAt.IsZero() && time.Since(p.VerifiedAt) < 5*time.Minute
}

func selectedLines(catalog *models.Catalog, message string, products []models.Product) []pendingLine {
	parts := regexp.MustCompile(`[;\n]+`).Split(message, -1)
	lines := []pendingLine{}
	seen := map[string]bool{}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		product := inferProductForAdd(catalog, part, products)
		if product == nil || seen[product.Article] {
			return nil
		}
		seen[product.Article] = true
		lines = append(lines, pendingLine{Product: *product, Quantity: requestedQuantity(part)})
	}
	if len(lines) > 4 {
		return nil
	}
	return lines
}

// Recheck at confirmation. On an upstream failure, old quotes cannot be used
// to change a basket. Unit tests may supply a recent verified fixture.
func verifyProduct(ctx context.Context, product models.Product, api *EKTAPI, live *LiveCatalog) (models.Product, string) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if api != nil && api.Enabled() {
		detail, err := api.ProductDetail(ctx, product.ID)
		if err != nil || !referencesEqual(detail.Article, product.Article) {
			return product, "Не удалось проверить актуальную цену и остаток EKT. Попробуйте позднее."
		}
		applyProductDetail(&product, detail)
	} else if live != nil {
		product.VerifiedAt = time.Time{}
		product.Price, product.StockQuantity, product.TotalStockQuantity = 0, 0, 0
		product.Availability = ""
		product = live.Enrich(ctx, []models.Product{product})[0]
	}
	if !freshProduct(product) {
		return product, "Актуальная цена и остаток не проверены. Попробуйте позднее."
	}
	return product, ""
}

func quoteReply(products []models.Product) string {
	if len(products) == 0 {
		return "Товар не найден. Уточните артикул или название."
	}
	lines := make([]string, 0, len(products))
	for _, p := range products {
		line := fmt.Sprintf("«%s» (арт. %s): ", p.Name, p.Article)
		if !freshProduct(p) {
			line += "актуальные цена и наличие не подтверждены."
		} else {
			if p.Price > 0 {
				line += fmt.Sprintf("%.2f тг за единицу; ", p.Price)
			} else {
				line += "цена не подтверждена; "
			}
			if p.Availability == "" {
				line += "наличие не подтверждено"
			} else {
				line += p.Availability
			}
			if p.StockLocation != "" {
				line += " · " + p.StockLocation
			}
			if p.StockQuantity > 0 {
				line += fmt.Sprintf(" · в регионе %d шт.", p.StockQuantity)
			}
			if p.TotalStockQuantity > 0 {
				line += fmt.Sprintf(" · всего по EKT %d шт.", p.TotalStockQuantity)
			}
			line += " (проверено " + p.VerifiedAt.UTC().Format("15:04 UTC") + ")."
		}
		if p.URL != "" {
			line += " Источник: " + p.URL
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n\n")
}

func selectionReply(lines []pendingLine) string {
	parts := []string{"Подтвердите добавление в корзину:"}
	for _, line := range lines {
		parts = append(parts, fmt.Sprintf("«%s» (арт. %s) — %d шт. × %.2f тг = %.2f тг", line.Product.Name, line.Product.Article, line.Quantity, line.Product.Price, float64(line.Quantity)*line.Product.Price))
	}
	return strings.Join(parts, "\n") + "\nНапишите «да, добавь» или нажмите кнопку подтверждения. Это не оформляет заказ."
}

func (s *CartStore) addChecked(key string, p models.Product, quantity int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.carts[key][p.Article]
	if message := validateQuantity(p, quantity, current.Quantity); message != "" {
		return message
	}
	if s.carts[key] == nil {
		s.carts[key] = make(map[string]models.CartItem)
	}
	s.carts[key][p.Article] = models.CartItem{Article: p.Article, Name: p.Name, Price: p.Price, Quantity: current.Quantity + quantity}
	return ""
}
