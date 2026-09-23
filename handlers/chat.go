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
- Используй add_to_cart только после явного согласия клиента, а не просто при рекомендации товара.
- Все цены в тенге (тг / KZT)
- Если клиент спрашивает конкретный товар — он будет показан отдельно карточками с фото, просто дай краткое текстовое описание
- Отвечай на русском (или на языке клиента)
- Будь лаконичным, профессиональным и дружелюбным
- НЕ перечисляй товары списком в тексте — они будут показаны карточками автоматически

`

var addIntentPattern = regexp.MustCompile(`(?i)(добав(?:ь|ить|ьте)|хочу\s+куп|покупаю|купить|беру|add\s+(?:this\s+)?(?:to\s+)?cart|buy|purchase)`)
var quantityAfterLabelPattern = regexp.MustCompile(`(?i)(?:x|×|колич(?:ество|\-во)?|шт\.?|штук(?:и)?)\s*[:=]?\s*(\d+)`)
var quantityBeforeUnitPattern = regexp.MustCompile(`(?i)\b(\d+)\s*(?:шт\.?|штук(?:и)?|pcs)`)

// buildSystemPrompt creates the AI prompt with embedded product data.
func buildSystemPrompt(catalog *models.Catalog) string {
	prompt := systemPromptBase

	if len(catalog.Products) > 0 {
		prompt += "## Товары в каталоге (используй для ответов):\n"
		limit := len(catalog.Products)
		if limit > 50 {
			limit = 50
		}
		for _, p := range catalog.Products[:limit] {
			prompt += fmt.Sprintf("- [Арт: %s | ID: %d] %s — %.0f тг\n", p.Article, p.ID, p.Name, p.Price)
		}
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
	for _, p := range catalog.Products {
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
			ID:      result.p.ID,
			Name:    result.p.Name,
			Article: result.p.Article,
			Price:   result.p.Price,
			Image:   result.p.Image,
			URL:     result.p.URL,
		})
	}
	return out
}

func findProductByReference(catalog *models.Catalog, reference string) *models.Product {
	reference = strings.ToLower(strings.TrimSpace(reference))
	if reference == "" {
		return nil
	}
	for i := range catalog.Products {
		product := &catalog.Products[i]
		if strings.ToLower(product.Article) == reference || strconv.Itoa(product.ID) == reference {
			return product
		}
	}
	// Supplier/article codes such as 027228 are present in the product name
	// while the API article may be a different internal code.
	for i := range catalog.Products {
		if strings.Contains(strings.ToLower(catalog.Products[i].Name), reference) {
			return &catalog.Products[i]
		}
	}
	return nil
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

func inferProductForAdd(catalog *models.Catalog, message string) *models.Product {
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

func cartAction(product *models.Product, quantity int) *models.CartAction {
	return &models.CartAction{
		Article:  product.Article,
		Name:     product.Name,
		Price:    product.Price,
		Quantity: quantity,
	}
}

func addToCartFromMessage(catalog *models.Catalog, store *CartStore, sessionID, message string) (*models.CartAction, *models.CartResponse, string) {
	if !hasAddIntent(message) {
		return nil, nil, ""
	}
	quantity := requestedQuantity(message)
	if quantity < 1 || quantity > maxCartQuantity {
		return nil, nil, "Количество должно быть от 1 до 999."
	}
	product := inferProductForAdd(catalog, message)
	if product == nil {
		return nil, nil, "Уточните артикул или выберите один товар из карточек, чтобы я добавил его в корзину."
	}
	cart := store.add(sessionID, *product, quantity)
	return cartAction(product, quantity), &cart, fmt.Sprintf("✅ Добавил «%s» (%d шт.) в корзину.", product.Name, quantity)
}

func addToCartFromTool(catalog *models.Catalog, store *CartStore, sessionID, userMessage string, arguments string) (*models.CartAction, *models.CartResponse, map[string]any) {
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
			Description: "Adds one catalog product to the current user's cart after explicit customer consent. The server validates the article and quantity.",
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

// ChatHandler handles POST /api/chat.
func ChatHandler(catalog *models.Catalog, store *CartStore) http.HandlerFunc {
	apiKey := os.Getenv("OPENAI_API_KEY")

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

		sessionID, err := store.sessionID(w, r)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Could not create a chat session")
			return
		}
		matchedProducts := searchProducts(catalog, req.Message, 4)

		if apiKey == "" {
			action, cart, addReply := addToCartFromMessage(catalog, store, sessionID, req.Message)
			reply := addReply
			if reply == "" {
				reply = "Здравствуйте! 👋 Я AI-ассистент ГК Электрокомплект.\n\n" +
					"(Демо-режим — настройте OPENAI_API_KEY для полноценного ИИ)\n\n" +
					"Звоните: 📞 +7 (727) 346-88-88"
			}
			writeJSON(w, models.ChatResponse{Reply: reply, Products: matchedProducts, CartAction: action, Cart: cart})
			return
		}

		client := openai.NewClient(apiKey)
		tools := []openai.Tool{addToCartTool()}
		messages := []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: buildSystemPrompt(catalog)},
			{Role: openai.ChatMessageRoleUser, Content: req.Message},
		}
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
			action, cart, result = addToCartFromTool(catalog, store, sessionID, req.Message, toolCall.Function.Arguments)
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
		writeJSON(w, models.ChatResponse{Reply: reply, Products: matchedProducts, CartAction: action, Cart: cart})
	}
}
