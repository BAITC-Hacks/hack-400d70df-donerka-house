package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
	openai "github.com/sashabaranov/go-openai"
)

const systemPromptBase = `Ты — умный AI-ассистент интернет-магазина ГК Электрокомплект (ekt.kz).

О компании:
- Название: ГК Электрокомплект (сайт: ekt.kz)
- Крупнейший производитель и поставщик электротехнической продукции в Казахстане
- Офисы в городах: Алматы, Астана, Шымкент, Тараз, Атырау, Актау, Караганда, Талдыкорган, Усть-Каменогорск
- Телефон: +7 (727) 346-88-88 | +7 (778) 046-88-88
- B2B платформа: pro.ekt.kz
- WhatsApp: +7 (778) 276-88-88

Категории товаров:
- Кабель / Провод
- Светильники / Лампы
- Низковольтная аппаратура (автоматические выключатели, реле, контакторы)
- Кабеленесущие системы
- Изделия для монтажа и инструмент
- Шкафы / Щиты
- Розетки / Выключатели / Коробки
- Автоматизация
- Видеонаблюдение / СКУД / Сигнализация
- Инструмент / КИП

Сервис:
- Доставка по всему Казахстану
- Онлайн-оплата (AirbaPay, Cloudpayments)
- Рассрочка
- Возврат и обмен
- Щиты под заказ

Твоя задача:
1. Помогать клиентам найти нужные товары по описанию или артикулу
2. Отвечать на вопросы о наличии, ценах, характеристиках
3. Консультировать по выбору оборудования
4. Направлять на нужные разделы сайта
5. Если клиент хочет купить товар, сначала зафиксируй товар и количество, затем попроси отдельное подтверждение «да, добавь». Корзина меняется только после такого подтверждения.

Важно:
- Фразы «хочу купить», «беру», «дай 5 шт» или «добавь 5 шт» означают намерение купить, но НЕ являются финальным подтверждением.
- После выбора товара и количества обязательно попроси отдельное подтверждение «да, добавь».
- Корзина должна меняться только после отдельного подтверждения клиента.
- Если товар недоступен, предложи релевантные аналоги и кратко объясни, почему они подходят.
- Если в данных есть сертификаты, сообщи ссылки. Если сертификатов нет, не выдумывай их.
- Условия покупки: доставка по Казахстану; онлайн-оплата AirbaPay/Cloudpayments; рассрочка. Для прототипа минимальная партия — 1 шт., если для конкретного товара не указано иное.
- Если клиент спрашивает «для чего это», «где применяется», «чем отличается» или задаёт технический вопрос, объясни назначение, применение, ограничения и ключевые характеристики выбранного товара по данным из актуального контекста сайта.
- Говори о наличии только по полям «Наличие», «Количество в регионе» и «Регион» из актуальных данных EKT. Не выдумывай остатки.
- Если указано «В наличии», сообщай, что товар доступен в регионе страницы EKT; если указано количество — называй его как доступный региональный лимит. Если «Под заказ», прямо сообщай это.
- Если данных о наличии нет, честно скажи, что публичная страница не показала статус, и дай ссылку на товар. Не говори, что проверить наличие невозможно, когда статус есть в контексте.
- Все цены в тенге (тг / KZT)
- Если клиент спрашивает конкретный товар — он будет показан отдельно карточками с фото, просто дай краткое текстовое описание
- Отвечай на русском (или на языке клиента)
- Будь лаконичным, профессиональным и дружелюбным
- НЕ перечисляй товары списком в тексте — они будут показаны карточками автоматически

`

var addIntentPattern = regexp.MustCompile(`(?i)(добав(?:ь|ить|ьте)|хочу\s+куп|куп(?:и|ить|лю)|покупаю|закаж(?:и|у|ем)|оформля(?:ю|ем)|беру|выбира(?:ю|ем)|выбрал|выбрала|add\s+(?:this\s+)?(?:to\s+)?cart|buy|purchase|i\s+(?:choose|want\s+this)|this\s+one|that\s+one|first\s+one)`)
var quantityAfterLabelPattern = regexp.MustCompile(`(?i)(?:x|×|колич(?:ество|\-во)?|шт\.?|штук(?:и)?)\s*[:=]?\s*(\d+)`)
var quantityBeforeUnitPattern = regexp.MustCompile(`(?i)\b(\d+)\s*(?:шт\.?|штук(?:и)?|pcs)`)
var productChoicePattern = regexp.MustCompile(`(?i)(перв|втор|трет|четверт|номер\s*\d+|№\s*\d+|first|second|third|fourth)`)
var availabilityQuestionPattern = regexp.MustCompile(`(?i)(налич|склад|остат|есть\s+ли|доступн|под\s+заказ|в\s+налич|stock|availab|inventory)`)
var catalogReferencePattern = regexp.MustCompile(`(?i)[a-zа-я0-9]+(?:[-_/][a-zа-я0-9]+)+|[a-zа-я0-9]*[a-zа-я][a-zа-я0-9_/-]*\d[a-zа-я0-9_/-]*|\d{4,}`)
var quantityFollowupPurchasePattern = regexp.MustCompile(`(?i)^\s*(?:(?:дай|дайте)(?:\s+мне)?|добав(?:ь|ьте)(?:\s+мне)?|беру|возьму)?\s*\d+\s*(?:шт\.?|штук(?:и)?|pcs)\s*$`)
var confirmationPattern = regexp.MustCompile(`(?i)^\s*(?:да\s*,?\s*добав(?:ь|ьте)|подтверждаю\s+добавление|confirm\s+add)\s*[.!]?\s*package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
	openai "github.com/sashabaranov/go-openai"
)

const systemPromptBase = `Ты — умный AI-ассистент интернет-магазина ГК Электрокомплект (ekt.kz).

О компании:
- Название: ГК Электрокомплект (сайт: ekt.kz)
- Крупнейший производитель и поставщик электротехнической продукции в Казахстане
- Офисы в городах: Алматы, Астана, Шымкент, Тараз, Атырау, Актау, Караганда, Талдыкорган, Усть-Каменогорск
- Телефон: +7 (727) 346-88-88 | +7 (778) 046-88-88
- B2B платформа: pro.ekt.kz
- WhatsApp: +7 (778) 276-88-88

Категории товаров:
- Кабель / Провод
- Светильники / Лампы
- Низковольтная аппаратура (автоматические выключатели, реле, контакторы)
- Кабеленесущие системы
- Изделия для монтажа и инструмент
- Шкафы / Щиты
- Розетки / Выключатели / Коробки
- Автоматизация
- Видеонаблюдение / СКУД / Сигнализация
- Инструмент / КИП

Сервис:
- Доставка по всему Казахстану
- Онлайн-оплата (AirbaPay, Cloudpayments)
- Рассрочка
- Возврат и обмен
- Щиты под заказ

Твоя задача:
1. Помогать клиентам найти нужные товары по описанию или артикулу
2. Отвечать на вопросы о наличии, ценах, характеристиках
3. Консультировать по выбору оборудования
4. Направлять на нужные разделы сайта
5. Если клиент хочет купить товар, сначала зафиксируй товар и количество, затем попроси отдельное подтверждение «да, добавь». Корзина меняется только после такого подтверждения.

Важно:
- Фразы «хочу купить», «беру», «дай 5 шт» или «добавь 5 шт» означают намерение купить, но НЕ являются финальным подтверждением.
- После выбора товара и количества обязательно попроси отдельное подтверждение «да, добавь».
- Корзина должна меняться только после отдельного подтверждения клиента.
- Если товар недоступен, предложи релевантные аналоги и кратко объясни, почему они подходят.
- Если в данных есть сертификаты, сообщи ссылки. Если сертификатов нет, не выдумывай их.
- Условия покупки: доставка по Казахстану; онлайн-оплата AirbaPay/Cloudpayments; рассрочка. Для прототипа минимальная партия — 1 шт., если для конкретного товара не указано иное.
- Если клиент спрашивает «для чего это», «где применяется», «чем отличается» или задаёт технический вопрос, объясни назначение, применение, ограничения и ключевые характеристики выбранного товара по данным из актуального контекста сайта.
- Говори о наличии только по полям «Наличие», «Количество в регионе» и «Регион» из актуальных данных EKT. Не выдумывай остатки.
- Если указано «В наличии», сообщай, что товар доступен в регионе страницы EKT; если указано количество — называй его как доступный региональный лимит. Если «Под заказ», прямо сообщай это.
- Если данных о наличии нет, честно скажи, что публичная страница не показала статус, и дай ссылку на товар. Не говори, что проверить наличие невозможно, когда статус есть в контексте.
- Все цены в тенге (тг / KZT)
- Если клиент спрашивает конкретный товар — он будет показан отдельно карточками с фото, просто дай краткое текстовое описание
- Отвечай на русском (или на языке клиента)
- Будь лаконичным, профессиональным и дружелюбным
- НЕ перечисляй товары списком в тексте — они будут показаны карточками автоматически

`

var addIntentPattern = regexp.MustCompile(`(?i)(добав(?:ь|ить|ьте)|хочу\s+куп|куп(?:и|ить|лю)|покупаю|закаж(?:и|у|ем)|оформля(?:ю|ем)|беру|выбира(?:ю|ем)|выбрал|выбрала|add\s+(?:this\s+)?(?:to\s+)?cart|buy|purchase|i\s+(?:choose|want\s+this)|this\s+one|that\s+one|first\s+one)`)
var quantityAfterLabelPattern = regexp.MustCompile(`(?i)(?:x|×|колич(?:ество|\-во)?|шт\.?|штук(?:и)?)\s*[:=]?\s*(\d+)`)
var quantityBeforeUnitPattern = regexp.MustCompile(`(?i)\b(\d+)\s*(?:шт\.?|штук(?:и)?|pcs)`)
var productChoicePattern = regexp.MustCompile(`(?i)(перв|втор|трет|четверт|номер\s*\d+|№\s*\d+|first|second|third|fourth)`)
var availabilityQuestionPattern = regexp.MustCompile(`(?i)(налич|склад|остат|есть\s+ли|доступн|под\s+заказ|в\s+налич|stock|availab|inventory)`)
var catalogReferencePattern = regexp.MustCompile(`(?i)[a-zа-я0-9]+(?:[-_/][a-zа-я0-9]+)+|[a-zа-я0-9]*[a-zа-я][a-zа-я0-9_/-]*\d[a-zа-я0-9_/-]*|\d{4,}`)
var quantityFollowupPurchasePattern = regexp.MustCompile(`(?i)^\s*(?:(?:дай|дайте)(?:\s+мне)?|добав(?:ь|ьте)(?:\s+мне)?|беру|возьму)?\s*\d+\s*(?:шт\.?|штук(?:и)?|pcs)\s*$`)
)
var cancellationPattern = regexp.MustCompile(`(?i)^\s*(?:нет|не\s+надо|отмена|отмени|cancel)\s*[.!]?\s*$`)
var purchaseTermsPattern = regexp.MustCompile(`(?i)(услови.{0,8}(?:покуп|заказ)|оплат|достав|минимальн.{0,8}(?:парт|заказ)|рассроч)`)

// buildSystemPrompt creates the AI prompt with embedded product data.
func buildSystemPrompt(catalog *models.Catalog, relevant ...[]models.Product) string {
	prompt := systemPromptBase
	products := catalog.ProductsSnapshot()

	if len(products) > 0 {
		prompt += "## Товары в каталоге (используй для ответов):\n"
		limit := len(products)
		if limit > 50 {
			limit = 50
		}
		for _, p := range products[:limit] {
			prompt += fmt.Sprintf("- [Арт: %s | ID: %d] %s — %.0f тг\n", p.Article, p.ID, p.Name, p.Price)
		}
	}

	if len(relevant) > 0 && len(relevant[0]) > 0 {
		prompt += "\n## Актуальные товары и сведения EKT (API/региональная страница):\n"
		for index, product := range relevant[0] {
			prompt += fmt.Sprintf("%d. [Арт: %s] %s — %.0f тг\n", index+1, product.Article, product.Name, product.Price)
			if product.Availability != "" {
				prompt += "   Наличие: " + product.Availability
				if product.StockQuantity > 0 {
					prompt += fmt.Sprintf("; количество в регионе: %d шт.", product.StockQuantity)
				}
				if product.StockLocation != "" {
					prompt += "; регион: " + product.StockLocation
				}
				prompt += "\n"
			}
			if product.TotalStockQuantity > 0 {
				prompt += fmt.Sprintf("   Общий остаток по API EKT: %d шт.\n", product.TotalStockQuantity)
			}
			for _, store := range product.Stores {
				if store.Quantity > 0 {
					prompt += fmt.Sprintf("   Склад: %s — %d шт.\n", store.Name, store.Quantity)
				}
			}
			if product.Description != "" {
				prompt += "   Описание и назначение: " + product.Description + "\n"
			}
			for name, value := range product.Properties {
				prompt += fmt.Sprintf("   Характеристика — %s: %s\n", name, value)
			}
			for _, certificate := range product.Certificates {
				prompt += "   Сертификат: " + certificate + "\n"
			}
			prompt += "   Ссылка: " + product.URL + "\n"
		}
		prompt += "Объясняй назначение, применение, ограничения и ключевые характеристики только по этим сведениям или явно отмечай, если данных недостаточно. Намерение купить не меняет корзину: после выбора товара и количества нужно отдельное подтверждение «да, добавь».\n"
	}

	if catalog.Detail != nil {
		d := catalog.Detail
		prompt += fmt.Sprintf(`
## Пример детальной информации:
Артикул: %s | Название: %s | Цена: %.0f тг | Наличие: %d шт.
`, d.Article, d.Name, d.Price, d.Quantity)
	}

	return prompt
}

// tokenize splits a string into lowercase words, stripping punctuation.
func tokenize(s string) []string {
	s = strings.ToLower(s)
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	return fields
}

// scoreProduct returns how well a product matches the query tokens.
func scoreProduct(p models.Product, tokens []string) int {
	name := strings.ToLower(p.Name)
	article := strings.ToLower(p.Article)
	score := 0
	for _, t := range tokens {
		if len(t) < 2 {
			continue
		}
		if strings.Contains(article, t) {
			score += 10
		}
		if strings.Contains(name, t) {
			score += 3
		}
	}
	return score
}

// searchProducts finds up to limit products matching the query.
func searchProducts(catalog *models.Catalog, query string, limit int) []models.ProductResult {
	tokens := tokenize(query)
	if len(tokens) == 0 || limit <= 0 {
		return nil
	}

	type scored struct {
		p     models.Product
		score int
	}
	var results []scored
	for _, p := range catalog.ProductsSnapshot() {
		if score := scoreProduct(p, tokens); score > 0 {
			results = append(results, scored{p: p, score: score})
		}
	}

	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].score > results[j-1].score; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}

	out := make([]models.ProductResult, 0, limit)
	for i, result := range results {
		if i >= limit {
			break
		}
		out = append(out, models.ProductResult{
			ID:                 result.p.ID,
			Name:               result.p.Name,
			Article:            result.p.Article,
			Price:              result.p.Price,
			Image:              result.p.Image,
			URL:                result.p.URL,
			Description:        result.p.Description,
			Properties:         result.p.Properties,
			Source:             result.p.Source,
			Availability:       result.p.Availability,
			StockQuantity:      result.p.StockQuantity,
			StockLocation:      result.p.StockLocation,
			TotalStockQuantity: result.p.TotalStockQuantity,
			Stores:             result.p.Stores,
			Certificates:       result.p.Certificates,
		})
	}
	return out
}

func normalizeArticleReference(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Trim(value, "\"'.,;:()[]{}")
	return strings.TrimRight(value, "_- ")
}

func findProductByReference(catalog *models.Catalog, reference string) *models.Product {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return nil
	}
	normalizedReference := normalizeArticleReference(reference)
	products := catalog.ProductsSnapshot()
	for i := range products {
		product := &products[i]
		if normalizeArticleReference(product.Article) == normalizedReference || strconv.Itoa(product.ID) == reference {
			return product
		}
	}
	for i := range products {
		if strings.Contains(strings.ToLower(products[i].Name), strings.ToLower(reference)) {
			return &products[i]
		}
	}
	return nil
}

func hasAddIntent(message string) bool {
	message = strings.TrimSpace(message)
	return addIntentPattern.MatchString(message) || quantityFollowupPurchasePattern.MatchString(message)
}

func isExplicitConfirmation(message string) bool {
	return confirmationPattern.MatchString(strings.TrimSpace(message))
}

func isCancellation(message string) bool {
	return cancellationPattern.MatchString(strings.TrimSpace(message))
}

func requestedQuantity(message string) int {
	for _, pattern := range []*regexp.Regexp{quantityAfterLabelPattern, quantityBeforeUnitPattern} {
		match := pattern.FindStringSubmatch(message)
		if len(match) == 2 {
			if quantity, err := strconv.Atoi(match[1]); err == nil {
				return quantity
			}
		}
	}
	return 1
}

func inferProductForAdd(catalog *models.Catalog, message string, contextProducts ...[]models.Product) *models.Product {
	if len(contextProducts) > 0 {
		if selected := selectedProductFromContext(message, contextProducts[0]); selected != nil {
			return selected
		}
		bestScore, bestIndex := 0, -1
		for index, product := range contextProducts[0] {
			if score := scoreProduct(product, tokenize(message)); score > bestScore {
				bestScore, bestIndex = score, index
			}
		}
		if bestIndex >= 0 {
			return &contextProducts[0][bestIndex]
		}
		if quantityFollowupPurchasePattern.MatchString(strings.TrimSpace(message)) && len(contextProducts[0]) > 0 {
			return &contextProducts[0][0]
		}
		if len(contextProducts[0]) == 1 && regexp.MustCompile(`(?i)(этот|это|его|this|that)`).MatchString(message) {
			return &contextProducts[0][0]
		}
	}
	for _, token := range tokenize(message) {
		if len(token) >= 3 {
			if product := findProductByReference(catalog, token); product != nil {
				return product
			}
		}
	}
	matches := searchProducts(catalog, message, 2)
	if len(matches) == 1 {
		return findProductByReference(catalog, matches[0].Article)
	}
	return nil
}

func selectedProductFromContext(message string, products []models.Product) *models.Product {
	if len(products) == 0 || !productChoicePattern.MatchString(message) {
		return nil
	}
	index := 0
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "втор") || strings.Contains(lower, "second"):
		index = 1
	case strings.Contains(lower, "трет") || strings.Contains(lower, "third"):
		index = 2
	case strings.Contains(lower, "четверт") || strings.Contains(lower, "fourth"):
		index = 3
	default:
		matches := regexp.MustCompile(`(?i)(?:номер|№)\s*(\d+)`).FindStringSubmatch(message)
		if len(matches) == 2 {
			index, _ = strconv.Atoi(matches[1])
			index--
		}
	}
	if index >= 0 && index < len(products) {
		return &products[index]
	}
	return nil
}

func cartAction(product *models.Product, quantity int) *models.CartAction {
	return &models.CartAction{
		Article:  product.Article,
		Name:     product.Name,
		Price:    product.Price,
		Quantity: quantity,
	}
}

func createCompletion(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage) (openai.ChatCompletionResponse, error) {
	request := openai.ChatCompletionRequest{
		Model:       "gpt-4o-mini",
		Messages:    messages,
		MaxTokens:   500,
		Temperature: 0.4,
	}
	var response openai.ChatCompletionResponse
	var err error
	for attempt := 1; attempt <= 3; attempt++ {
		response, err = client.CreateChatCompletion(ctx, request)
		if err == nil || attempt == 3 {
			return response, err
		}
		message := err.Error()
		if !strings.Contains(message, "500") && !strings.Contains(message, "502") && !strings.Contains(message, "503") {
			return response, err
		}
	}
	return response, err
}

func uniqueProducts(groups ...[]models.Product) []models.Product {
	result := make([]models.Product, 0, 8)
	seen := make(map[string]struct{})
	for _, group := range groups {
		for _, product := range group {
			key := strings.ToLower(product.Article) + "|" + strings.ToLower(product.URL)
			if product.Name == "" || key == "|" {
				continue
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result = append(result, product)
			if len(result) == 4 {
				return result
			}
		}
	}
	return result
}

func productsForResults(catalog *models.Catalog, results []models.ProductResult) []models.Product {
	products := catalog.ProductsSnapshot()
	result := make([]models.Product, 0, len(results))
	for _, match := range results {
		for _, product := range products {
			if strings.EqualFold(product.Article, match.Article) {
				result = append(result, product)
				break
			}
		}
	}
	return result
}

func productResultsFromProducts(products []models.Product) []models.ProductResult {
	results := make([]models.ProductResult, 0, len(products))
	for _, product := range products {
		results = append(results, models.ProductResult{
			ID:                 product.ID,
			Name:               product.Name,
			Article:            product.Article,
			Price:              product.Price,
			Image:              product.Image,
			URL:                product.URL,
			Description:        product.Description,
			Properties:         product.Properties,
			Source:             product.Source,
			Availability:       product.Availability,
			StockQuantity:      product.StockQuantity,
			StockLocation:      product.StockLocation,
			TotalStockQuantity: product.TotalStockQuantity,
			Stores:             product.Stores,
			Certificates:       product.Certificates,
		})
	}
	return results
}

func purchaseTermsReply() string {
	return "Условия покупки: оплата — онлайн через AirbaPay/Cloudpayments, также доступна рассрочка; доставка — по Казахстану. Для прототипа минимальная партия — 1 шт., если у конкретного товара не указано иное."
}

func productUnavailable(product models.Product) bool {
	status := strings.ToLower(strings.TrimSpace(product.Availability))
	return status == "под заказ" ||
		status == "нет в наличии" ||
		(status != "" && product.StockQuantity == 0 && product.TotalStockQuantity == 0 && status != "в наличии")
}

func analogSimilarity(target, candidate models.Product) int {
	if strings.EqualFold(target.Article, candidate.Article) || target.Article == "" || candidate.Article == "" {
		return -1
	}
	score := 0
	candidateName := strings.ToLower(candidate.Name)
	for _, token := range tokenize(target.Name) {
		if len([]rune(token)) >= 4 && strings.Contains(candidateName, token) {
			score += 2
		}
	}
	for key, value := range target.Properties {
		if value != "" && candidate.Properties[key] == value {
			score += 3
		}
	}
	if strings.EqualFold(candidate.Availability, "В наличии") {
		score += 8
	}
	if candidate.StockQuantity > 0 || candidate.TotalStockQuantity > 0 {
		score += 6
	}
	return score
}

func findAnalogs(catalog *models.Catalog, target models.Product, limit int) []models.Product {
	type scored struct {
		product models.Product
		score   int
	}
	all := catalog.ProductsSnapshot()
	candidates := make([]scored, 0, len(all))
	for _, candidate := range all {
		if productUnavailable(candidate) {
			continue
		}
		score := analogSimilarity(target, candidate)
		if score > 0 {
			candidates = append(candidates, scored{product: candidate, score: score})
		}
	}
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0 && candidates[j].score > candidates[j-1].score; j-- {
			candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
		}
	}
	if limit > len(candidates) {
		limit = len(candidates)
	}
	result := make([]models.Product, 0, limit)
	for i := 0; i < limit; i++ {
		result = append(result, candidates[i].product)
	}
	return result
}

func analogReason(target, analog models.Product) string {
	common := make([]string, 0, 2)
	for key, value := range target.Properties {
		if value != "" && analog.Properties[key] == value {
			common = append(common, key+"="+value)
			if len(common) == 2 {
				break
			}
		}
	}
	if len(common) > 0 {
		return "совпадают ключевые характеристики: " + strings.Join(common, ", ")
	}
	return "похож по названию/назначению и доступнее по текущим данным каталога"
}

func appendAnalogFacts(reply string, target *models.Product, analogs []models.Product) string {
	if target == nil || len(analogs) == 0 {
		return reply
	}
	lines := make([]string, 0, len(analogs))
	for _, analog := range analogs {
		lines = append(lines, fmt.Sprintf("«%s» (арт. %s) — %s", analog.Name, analog.Article, analogReason(*target, analog)))
	}
	return strings.TrimSpace(reply) + "\n\nПодходящие аналоги: " + strings.Join(lines, "; ") + "."
}

// ChatHandler handles POST /api/chat.
func ChatHandler(catalog *models.Catalog, store *CartStore, live *LiveCatalog, conversations *ConversationStore, ektAPIs ...*EKTAPI) http.HandlerFunc {
	return ChatHandlerWithAuth(catalog, store, live, conversations, nil, ektAPIs...)
}

func ChatHandlerWithAuth(catalog *models.Catalog, store *CartStore, live *LiveCatalog, conversations *ConversationStore, auth *AuthStore, ektAPIs ...*EKTAPI) http.HandlerFunc {
	apiKey := os.Getenv("OPENAI_API_KEY")
	var ektAPI *EKTAPI
	if len(ektAPIs) > 0 {
		ektAPI = ektAPIs[0]
	}
	if conversations == nil {
		conversations = NewConversationStore()
	}

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		var req models.ChatRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
		if err := decoder.Decode(&req); err != nil || strings.TrimSpace(req.Message) == "" {
			writeJSONError(w, http.StatusBadRequest, "Invalid or empty message")
			return
		}

		sessionID, err := store.keyForRequest(w, r, auth)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Could not create a chat session")
			return
		}

		if isCancellation(req.Message) {
			conversations.clearPending(sessionID)
			reply := "Хорошо, добавление в корзину отменено."
			conversations.append(sessionID, req.Message, reply, nil)
			writeJSON(w, models.ChatResponse{Reply: reply})
			return
		}

		if isExplicitConfirmation(req.Message) {
			pending := conversations.getPending(sessionID)
			if pending == nil {
				writeJSON(w, models.ChatResponse{Reply: "Сначала выберите товар и количество, затем я попрошу подтверждение."})
				return
			}
			product := pending.Product
			if latest := findProductByReference(catalog, product.Article); latest != nil {
				product = *latest
			}
			if message := validateCartQuantity(store, sessionID, product, pending.Quantity); message != "" {
				conversations.clearPending(sessionID)
				analogs := findAnalogs(catalog, product, 3)
				reply := appendAnalogFacts(message, &product, analogs)
				writeJSON(w, models.ChatResponse{Reply: reply, Analogs: productResultsFromProducts(analogs)})
				return
			}
			cart := store.add(sessionID, product, pending.Quantity)
			action := cartAction(&product, pending.Quantity)
			conversations.clearPending(sessionID)
			reply := fmt.Sprintf("✅ Добавлено: «%s» — %d шт. Открыть актуальную корзину: /#cart", product.Name, pending.Quantity)
			conversations.append(sessionID, req.Message, reply, []models.Product{product})
			writeJSON(w, models.ChatResponse{Reply: reply, CartAction: action, Cart: &cart, CartURL: "/#cart"})
			return
		}

		if purchaseTermsPattern.MatchString(req.Message) {
			reply := purchaseTermsReply()
			conversations.append(sessionID, req.Message, reply, nil)
			writeJSON(w, models.ChatResponse{Reply: reply})
			return
		}

		_, recentProducts := conversations.get(sessionID)
		liveProducts := make([]models.Product, 0)
		if live != nil {
			liveContext, cancel := context.WithTimeout(r.Context(), 7*time.Second)
			liveProducts, _ = live.Search(liveContext, liveSearchQuery(req.Message))
			liveProducts = live.Enrich(liveContext, liveProducts)
			cancel()
			if ektAPI != nil && ektAPI.Enabled() && len(liveProducts) > 0 {
				apiContext, apiCancel := context.WithTimeout(r.Context(), 6*time.Second)
				liveProducts = ektAPI.Enrich(apiContext, liveProducts)
				apiCancel()
			}
			catalog.AddProducts(liveProducts)
		}

		contextProducts := uniqueProducts(liveProducts, recentProducts)
		matchedProducts := searchProducts(catalog, req.Message, 4)
		if len(liveProducts) > 0 {
			matchedProducts = productResultsFromProducts(liveProducts)
		}
		if len(matchedProducts) == 0 && len(recentProducts) > 0 {
			matchedProducts = productResultsFromProducts(recentProducts)
		}
		if len(contextProducts) == 0 {
			contextProducts = productsForResults(catalog, matchedProducts)
		}
		if ektAPI != nil && ektAPI.Enabled() && len(liveProducts) == 0 && len(contextProducts) > 0 {
			apiContext, apiCancel := context.WithTimeout(r.Context(), 6*time.Second)
			contextProducts = ektAPI.Enrich(apiContext, contextProducts)
			apiCancel()
			catalog.AddProducts(contextProducts)
			matchedProducts = productResultsFromProducts(contextProducts)
		}

		var unavailable *models.Product
		for i := range contextProducts {
			if productUnavailable(contextProducts[i]) {
				copyProduct := contextProducts[i]
				unavailable = &copyProduct
				break
			}
		}
		analogs := make([]models.Product, 0)
		if unavailable != nil {
			analogs = findAnalogs(catalog, *unavailable, 3)
		}

		if hasAddIntent(req.Message) {
			quantity := requestedQuantity(req.Message)
			product := inferProductForAdd(catalog, req.Message, contextProducts)
			if product == nil {
				reply := "Уточните артикул или выберите один товар из карточек."
				conversations.append(sessionID, req.Message, reply, contextProducts)
				writeJSON(w, models.ChatResponse{Reply: reply, Products: matchedProducts, Analogs: productResultsFromProducts(analogs)})
				return
			}
			if message := validateCartQuantity(store, sessionID, *product, quantity); message != "" {
				productAnalogs := findAnalogs(catalog, *product, 3)
				reply := appendAnalogFacts(message, product, productAnalogs)
				conversations.append(sessionID, req.Message, reply, contextProducts)
				writeJSON(w, models.ChatResponse{Reply: reply, Products: matchedProducts, Analogs: productResultsFromProducts(productAnalogs)})
				return
			}
			conversations.setPending(sessionID, *product, quantity)
			reply := fmt.Sprintf("Подтвердите: добавить «%s» — %d шт. в корзину? Напишите «да, добавь».", product.Name, quantity)
			conversations.append(sessionID, req.Message, reply, []models.Product{*product})
			writeJSON(w, models.ChatResponse{Reply: reply, Products: matchedProducts})
			return
		}

		if apiKey == "" {
			reply := "Здравствуйте! 👋 Я AI-ассистент ГК Электрокомплект.\n\n" +
				"(Демо-режим — настройте OPENAI_API_KEY для полноценного ИИ)\n\n" +
				"Звоните: 📞 +7 (727) 346-88-88"
			reply = appendAvailabilityFacts(reply, req.Message, contextProducts)
			reply = appendAnalogFacts(reply, unavailable, analogs)
			conversations.append(sessionID, req.Message, reply, contextProducts)
			writeJSON(w, models.ChatResponse{Reply: reply, Products: matchedProducts, Analogs: productResultsFromProducts(analogs)})
			return
		}

		client := openai.NewClient(apiKey)
		history, _ := conversations.get(sessionID)
		messages := []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: buildSystemPrompt(catalog, contextProducts)}}
		messages = append(messages, history...)
		messages = append(messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: req.Message})
		first, err := createCompletion(r.Context(), client, messages)
		if err != nil || len(first.Choices) == 0 {
			reply := appendAnalogFacts("😔 AI-ассистент временно недоступен. Попробуйте через несколько секунд или позвоните нам: 📞 +7 (727) 346-88-88", unavailable, analogs)
			writeJSON(w, models.ChatResponse{Reply: reply, Products: matchedProducts, Analogs: productResultsFromProducts(analogs)})
			return
		}

		reply := first.Choices[0].Message.Content
		reply = appendAvailabilityFacts(reply, req.Message, contextProducts)
		reply = appendAnalogFacts(reply, unavailable, analogs)
		conversations.append(sessionID, req.Message, reply, contextProducts)
		writeJSON(w, models.ChatResponse{Reply: reply, Products: matchedProducts, Analogs: productResultsFromProducts(analogs)})
	}
}

func liveSearchQuery(message string) string {
	bestToken, bestScore := "", 0
	for _, token := range catalogReferencePattern.FindAllString(message, -1) {
		lower := strings.ToLower(token)
		score := len([]rune(token))
		if strings.IndexFunc(lower, unicode.IsLetter) >= 0 && strings.IndexFunc(lower, unicode.IsDigit) >= 0 {
			score += 100
		}
		if strings.ContainsAny(token, "-_/\\") {
			score += 20
		}
		if score > bestScore {
			bestToken, bestScore = token, score
		}
	}
	if bestToken != "" {
		return bestToken
	}
	return message
}

func appendAvailabilityFacts(reply, message string, products []models.Product) string {
	if !availabilityQuestionPattern.MatchString(message) || len(products) == 0 {
		return reply
	}

	lines := make([]string, 0, 1)
	missingFact := false
	lowerReply := strings.ToLower(reply)
	for _, product := range products {
		if product.Availability == "" {
			continue
		}
		line := fmt.Sprintf("«%s»: %s", product.Name, product.Availability)
		if product.StockQuantity > 0 {
			line += fmt.Sprintf(", до %d шт.", product.StockQuantity)
		}
		if product.StockLocation != "" {
			line += " (" + product.StockLocation + ")"
		}
		lines = append(lines, line)
		if !strings.Contains(lowerReply, strings.ToLower(product.Availability)) ||
			(product.StockQuantity > 0 && !strings.Contains(lowerReply, strconv.Itoa(product.StockQuantity))) ||
			(product.StockLocation != "" && !strings.Contains(lowerReply, strings.ToLower(product.StockLocation))) {
			missingFact = true
		}
		// The cards contain the full result set. Keep the guaranteed textual
		// fallback focused on the first, most relevant live match.
		break
	}
	if !missingFact || len(lines) == 0 {
		return reply
	}
	return strings.TrimSpace(reply) + "\n\nНаличие по данным региональной страницы EKT: " + strings.Join(lines, "; ") + "."
}
