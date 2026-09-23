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
	if enriched[0].Availability == "" {
		t.Fatalf("expected live availability status, got %+v", enriched[0])
	}
	t.Logf("live product: name=%q article=%q price=%.0f availability=%q stock=%d location=%q description=%q properties=%v", enriched[0].Name, enriched[0].Article, enriched[0].Price, enriched[0].Availability, enriched[0].StockQuantity, enriched[0].StockLocation, enriched[0].Description, enriched[0].Properties)
}

func TestLiveCatalogAvailabilityIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	live := NewLiveCatalog("https://nursultan.ekt.kz")
	products, err := live.Search(ctx, "027228")
	if err != nil {
		t.Fatal(err)
	}
	if len(products) == 0 {
		t.Fatal("expected product 027228 from live catalog")
	}
	products = live.Enrich(ctx, products[:1])
	if products[0].Availability != "В наличии" || products[0].StockQuantity <= 0 {
		t.Fatalf("expected in-stock product with a positive regional quantity, got %+v", products[0])
	}
	t.Logf("in-stock product: name=%q article=%q stock=%d location=%q", products[0].Name, products[0].Article, products[0].StockQuantity, products[0].StockLocation)
}
