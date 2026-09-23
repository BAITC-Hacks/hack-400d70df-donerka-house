package handlers

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestParseLiveCardAndDetail(t *testing.T) {
	cardHTML := `<div id="bx_1_56259_hash" class="col-md-3 product-card-out product-card-out-catalog">
  <div class="product-card-image"><a href="/catalog/tool/"><img src="/upload/tool.jpg"></a></div>
  <a href="/catalog/tool/"><h3 class="product-title">Тестовый стриппер</h3></a>
  <div class="price">29 990 ₸</div>
  <div class="product-article">Код товара 311101258_</div>
  <div class="productarea-hidden"><div class="left-hidden-block"><input class="tq_quantity" max="8"></div><div class="right-hidden-block"><a data-action="add2basket">Купить</a></div></div>
</div>`
	root, err := html.Parse(strings.NewReader(cardHTML))
	if err != nil {
		t.Fatal(err)
	}
	card := findDescendant(root, func(node *html.Node) bool { return hasClass(node, "product-card-out-catalog") })
	product := parseLiveCard("https://nursultan.ekt.kz", card)
	if product.ID != 56259 || product.Article != "311101258_" || product.Price != 29990 || product.URL != "https://nursultan.ekt.kz/catalog/tool/" || product.Availability != "В наличии" || product.StockQuantity != 8 {
		t.Fatalf("unexpected parsed card: %+v", product)
	}

	detailHTML := `<div class="detail_tabs__body__item" tab="description"><div class="detail_tabs__body__item__value">Используется для снятия изоляции.</div></div>
<div class="detail_info__price__site__value">29 990 ₸</div>
<div class="tab_item_chars__item"><div class="tab_item_chars__item__name">Диапазон:</div><div class="tab_item_chars__item__value">6-16 мм²</div></div>
<div class="detail_info__buttons"><a data-action="add2basketPreOrder"><span>Под заказ</span></a></div>
<a href="/upload/certificates/test-certificate.pdf">Сертификат соответствия</a>
<div class="select-city__block__text-city">Астана</div>`
	detailRoot, err := html.Parse(strings.NewReader(detailHTML))
	if err != nil {
		t.Fatal(err)
	}
	parseLiveDetail(&product, detailRoot)
	if product.Description != "Используется для снятия изоляции." || product.Properties["Диапазон"] != "6-16 мм²" || product.Availability != "Под заказ" || product.StockLocation != "Астана" {
		t.Fatalf("unexpected parsed detail: %+v", product)
	}
	if len(product.Certificates) != 1 || product.Certificates[0] != "https://nursultan.ekt.kz/upload/certificates/test-certificate.pdf" {
		t.Fatalf("expected certificate link, got %+v", product.Certificates)
	}
}

func TestLiveSearchQueryPrefersProductReference(t *testing.T) {
	if query := liveSearchQuery("Есть ли в наличии автомат 027228? Не добавляй в корзину."); query != "027228" {
		t.Fatalf("expected product reference query, got %q", query)
	}
	if query := liveSearchQuery("Купи 3 штуки анкера PAL-2000 (70-120) UNIT"); query != "PAL-2000" {
		t.Fatalf("expected hyphenated product reference query, got %q", query)
	}
	if query := liveSearchQuery("Что есть в наличии?"); query != "Что есть в наличии?" {
		t.Fatalf("expected original natural-language query, got %q", query)
	}
}
