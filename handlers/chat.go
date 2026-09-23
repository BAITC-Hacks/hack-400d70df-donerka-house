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
5. Если клиент явно говорит "добавь в корзину", "хочу купить", "покупаю" или "беру", вызови add_to_cart.

Важно:
- Для add_to_cart передавай настоящий артикул из каталога и положительное количество.
- Используй add_to_cart после явного выбора клиента. Фразы «беру этот», «выбираю первый», «добавь» и аналогичные уже являются согласием — НЕ спрашивай повторное подтверждение.
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
			prompt += "   Ссылка: " + product.URL + "\n"
		}
		prompt += "Объясняй назначение, применение, ограничения и ключевые характеристики только по этим сведениям или явно отмечай, если данных недостаточно. Если пользователь выбрал товар словами «беру», «выбираю этот/первый» или аналогично, считай это согласием и сразу вызывай add_to_cart без дополнительного вопроса-подтверждения.\n"
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
		})
	}
	return out
}

func findProductByReference(catalog *models.Catalog, reference string) *models.Product {
	reference = strings.TrimSpace(reference)
	if normalizeReference(reference) == "" {
		return nil
	}
	products := catalog.ProductsSnapshot()
	for i := range products {
		product := &products[i]
		if referencesEqual(product.Article, reference) || strconv.Itoa(product.ID) == reference {
			return product
		}
	}
	// Supplier/article codes such as 027228 are present in the product name
	// while the API article may be a different internal code.
	for i := range products {
		if strings.Contains(strings.ToLower(products[i].Name), reference) {
			return &products[i]
		}
	}
	return nil
}

// normalizeReference handles the supplier format used by EKT, where many
// catalog articles contain a trailing underscore (for example 010400273_)
// while customers usually type the visible number without it.
func normalizeReference(reference string) string {
	value := strings.ToLower(strings.TrimSpace(reference))
	value = strings.Trim(value, "\"'`.,;:()[]{}")
	return strings.TrimRight(value, "_")
}

func referencesEqual(left, right string) bool {
	left = normalizeReference(left)
	right = normalizeReference(right)
	return left != "" && left == right
}

func hasAddIntent(message string) bool {
	return addIntentPattern.MatchString(message)
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
		if len(contextProducts[0]) == 1 && regexp.MustCompile(`(?i)(этот|это|this|that)`).MatchString(message) {
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

func addToCartFromMessage(catalog *models.Catalog, store *CartStore, sessionID, message string, contextProducts ...[]models.Product) (*models.CartAction, *models.CartResponse, string) {
	if !hasAddIntent(message) {
		return nil, nil, ""
	}
	quantity := requestedQuantity(message)
	if quantity < 1 || quantity > maxCartQuantity {
		return nil, nil, "Количество должно быть от 1 до 999."
	}
	product := inferProductForAdd(catalog, message, contextProducts...)
	if product == nil {
		return nil, nil, "Уточните артикул или выберите один товар из карточек, чтобы я добавил его в корзину."
	}
	cart := store.add(sessionID, *product, quantity)
	return cartAction(product, quantity), &cart, fmt.Sprintf("✅ Добавил «%s» (%d шт.) в корзину.", product.Name, quantity)
}

func addToCartFromTool(catalog *models.Catalog, store *CartStore, sessionID, userMessage string, arguments string, contextProducts ...[]models.Product) (*models.CartAction, *models.CartResponse, map[string]any) {
	var args models.CartRequest
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return nil, nil, map[string]any{"ok": false, "error": "invalid_arguments"}
	}
	if !hasAddIntent(userMessage) {
		return nil, nil, map[string]any{"ok": false, "error": "explicit_customer_consent_required"}
	}
	if args.Quantity < 1 || args.Quantity > maxCartQuantity {
		return nil, nil, map[string]any{"ok": false, "error": "quantity_must_be_between_1_and_999"}
	}
	product := findProductByReference(catalog, args.Article)
	if product == nil && len(contextProducts) > 0 {
		product = selectedProductFromContext(userMessage, contextProducts[0])
	}
	if product == nil {
		return nil, nil, map[string]any{"ok": false, "error": "product_not_found"}
	}
	cart := store.add(sessionID, *product, args.Quantity)
	action := cartAction(product, args.Quantity)
	return action, &cart, map[string]any{
		"ok":       true,
		"article":  product.Article,
		"name":     product.Name,
		"quantity": args.Quantity,
	}
}

func addToCartTool() openai.Tool {
	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        "add_to_cart",
			Description: "Adds one catalog product to the current user's cart after the customer clearly chooses it. Do not ask for a second confirmation after phrases such as 'I will take this', 'the first one', or 'add it'. The server validates the article and quantity.",
			Parameters: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"article": map[string]any{
						"type":        "string",
						"description": "Exact catalog article from the catalog context, including a trailing underscore when present.",
					},
					"quantity": map[string]any{
						"type":        "integer",
						"description": "Positive quantity from 1 to 999.",
					},
				},
				"required": []string{"article", "quantity"},
			},
		},
	}
}

func createCompletion(ctx context.Context, client *openai.Client, messages []openai.ChatCompletionMessage, tools []openai.Tool, toolChoice any) (openai.ChatCompletionResponse, error) {
	request := openai.ChatCompletionRequest{
		Model:       "gpt-4o-mini",
		Messages:    messages,
		Tools:       tools,
		ToolChoice:  toolChoice,
		MaxTokens:   500,
		Temperature: 0.6,
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
			if referencesEqual(product.Article, match.Article) {
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
		})
	}
	return results
}

// ChatHandler keeps the original anonymous-session behavior for tests and
// deployments that do not configure local accounts.
func ChatHandler(catalog *models.Catalog, store *CartStore, live *LiveCatalog, conversations *ConversationStore, ektAPIs ...*EKTAPI) http.HandlerFunc {
	return ChatHandlerWithAuth(catalog, store, live, conversations, nil, ektAPIs...)
}

// ChatHandlerWithAuth uses the authenticated account as the cart and
// conversation owner when one is available. Guests retain a browser session.
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
			// Prefer the fresh regional HTML results over an older local copy of
			// the same article so stock fields reach both the UI and the model.
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
			matchedProducts = productResultsFromProducts(contextProducts)
		}

		// Explicit purchase language is already customer consent. Resolve and
		// add the validated product before asking the model for prose, so the
		// model cannot turn a direct purchase request into a confirmation step.
		if hasAddIntent(req.Message) {
			action, cart, directReply := addToCartFromMessage(catalog, store, sessionID, req.Message, contextProducts)
			if action != nil {
				conversations.append(sessionID, req.Message, directReply, contextProducts)
				writeJSON(w, models.ChatResponse{Reply: directReply, Products: matchedProducts, CartAction: action, Cart: cart})
				return
			}
		}

		if apiKey == "" {
			action, cart, addReply := addToCartFromMessage(catalog, store, sessionID, req.Message, contextProducts)
			reply := addReply
			if reply == "" {
				reply = "Здравствуйте! 👋 Я AI-ассистент ГК Электрокомплект.\n\n" +
					"(Демо-режим — настройте OPENAI_API_KEY для полноценного ИИ)\n\n" +
					"Звоните: 📞 +7 (727) 346-88-88"
			}
			reply = appendAvailabilityFacts(reply, req.Message, contextProducts)
			conversations.append(sessionID, req.Message, reply, contextProducts)
			writeJSON(w, models.ChatResponse{Reply: reply, Products: matchedProducts, CartAction: action, Cart: cart})
			return
		}

		client := openai.NewClient(apiKey)
		tools := []openai.Tool{addToCartTool()}
		history, _ := conversations.get(sessionID)
		messages := []openai.ChatCompletionMessage{{Role: openai.ChatMessageRoleSystem, Content: buildSystemPrompt(catalog, contextProducts)}}
		messages = append(messages, history...)
		messages = append(messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: req.Message})
		first, err := createCompletion(r.Context(), client, messages, tools, "auto")
		if err != nil || len(first.Choices) == 0 {
			writeJSON(w, models.ChatResponse{
				Reply:    "😔 AI-ассистент временно недоступен. Попробуйте через несколько секунд или позвоните нам: 📞 +7 (727) 346-88-88",
				Products: matchedProducts,
			})
			return
		}

		assistantMessage := first.Choices[0].Message
		if len(assistantMessage.ToolCalls) == 0 {
			assistantMessage.Content = appendAvailabilityFacts(assistantMessage.Content, req.Message, contextProducts)
			conversations.append(sessionID, req.Message, assistantMessage.Content, contextProducts)
			writeJSON(w, models.ChatResponse{Reply: assistantMessage.Content, Products: matchedProducts})
			return
		}

		messages = append(messages, assistantMessage)
		var action *models.CartAction
		var cart *models.CartResponse
		for _, toolCall := range assistantMessage.ToolCalls {
			if toolCall.Function.Name != "add_to_cart" {
				continue
			}
			var result map[string]any
			action, cart, result = addToCartFromTool(catalog, store, sessionID, req.Message, toolCall.Function.Arguments, contextProducts)
			encodedResult, _ := json.Marshal(result)
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				ToolCallID: toolCall.ID,
				Content:    string(encodedResult),
			})
		}

		finalResponse, finalErr := createCompletion(r.Context(), client, messages, tools, "none")
		reply := ""
		if finalErr == nil && len(finalResponse.Choices) > 0 {
			reply = finalResponse.Choices[0].Message.Content
		}
		if reply == "" && action != nil {
			reply = fmt.Sprintf("✅ Добавил «%s» (%d шт.) в корзину.", action.Name, action.Quantity)
		}
		if reply == "" {
			reply = "Не удалось обработать запрос. Уточните артикул товара."
		}
		reply = appendAvailabilityFacts(reply, req.Message, contextProducts)
		conversations.append(sessionID, req.Message, reply, contextProducts)
		writeJSON(w, models.ChatResponse{Reply: reply, Products: matchedProducts, CartAction: action, Cart: cart})
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
