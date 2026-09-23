//go:build live

package handlers

import (
	"context"
	"testing"
	"time"
)

func TestLiveCatalogSearchIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	live := NewLiveCatalog("https://nursultan.ekt.kz")
	products, err := live.Search(ctx, "автомат")
	if err != nil {
		t.Fatal(err)
	}
	if len(products) == 0 || products[0].Name == "" || products[0].URL == "" || products[0].Article == "" {
		t.Fatalf("expected live product data, got %+v", products)
	}
	enriched := live.Enrich(ctx, products[:1])
	t.Logf("live product: name=%q article=%q price=%.0f description=%q properties=%v", enriched[0].Name, enriched[0].Article, enriched[0].Price, enriched[0].Description, enriched[0].Properties)
}
