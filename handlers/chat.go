package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

Важно:
- Все цены в тенге (тг / KZT)
- Если клиент спрашивает конкретный товар — он будет показан отдельно карточками с фото, просто дай краткое текстовое описание
- Отвечай на русском (или на языке клиента)
- Будь лаконичным, профессиональным и дружелюбным
- НЕ перечисляй товары списком в тексте — они будут показаны карточками автоматически

`

// buildSystemPrompt creates the AI prompt with embedded product data
func buildSystemPrompt(catalog *models.Catalog) string {
	prompt := systemPromptBase

	if len(catalog.Products) > 0 {
		prompt += "## Товары в каталоге (используй для ответов):\n"
		limit := len(catalog.Products)
		if limit > 50 {
			limit = 50
		}
		for _, p := range catalog.Products[:limit] {
			prompt += fmt.Sprintf("- [Арт: %s] %s — %.0f тг\n",
				p.Article, p.Name, p.Price)
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

// tokenize splits a string into lowercase words, stripping punctuation
func tokenize(s string) []string {
	s = strings.ToLower(s)
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	return fields
}

// scoreProduct returns how well a product matches the query tokens.
// Higher = better match.
func scoreProduct(p models.Product, tokens []string) int {
	name := strings.ToLower(p.Name)
	article := strings.ToLower(p.Article)
	score := 0
	for _, t := range tokens {
		if len(t) < 2 {
			continue
		}
		if strings.Contains(article, t) {
			score += 10 // exact article match = highest priority
		}
		if strings.Contains(name, t) {
			score += 3
		}
	}
	return score
}

// searchProducts finds up to `limit` products matching the query
func searchProducts(catalog *models.Catalog, query string, limit int) []models.ProductResult {
	tokens := tokenize(query)
	if len(tokens) == 0 {
		return nil
	}

	type scored struct {
		p     models.Product
		score int
	}
	var results []scored

	for _, p := range catalog.Products {
		s := scoreProduct(p, tokens)
		if s > 0 {
			results = append(results, scored{p, s})
		}
	}

	// Sort by score descending (simple insertion sort — catalog is small)
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].score > results[j-1].score; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}

	out := make([]models.ProductResult, 0, limit)
	for i, r := range results {
		if i >= limit {
			break
		}
		out = append(out, models.ProductResult{
			ID:      r.p.ID,
			Name:    r.p.Name,
			Article: r.p.Article,
			Price:   r.p.Price,
			Image:   r.p.Image,
			URL:     r.p.URL,
		})
	}
	return out
}

// ChatHandler handles POST /api/chat
func ChatHandler(catalog *models.Catalog) http.HandlerFunc {
	apiKey := os.Getenv("OPENAI_API_KEY")

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Method not allowed"})
			return
		}

		var req models.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Message == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Invalid or empty message"})
			return
		}

		// Always search catalog for matching products
		matchedProducts := searchProducts(catalog, req.Message, 4)

		// Demo mode fallback
		if apiKey == "" {
			resp := models.ChatResponse{
				Reply: "Здравствуйте! 👋 Я AI-ассистент ГК Электрокомплект.\n\n" +
					"(Демо-режим — настройте OPENAI_API_KEY для полноценного ИИ)\n\n" +
					"Звоните: 📞 +7 (727) 346-88-88",
				Products: matchedProducts,
			}
			json.NewEncoder(w).Encode(resp)
			return
		}

		client := openai.NewClient(apiKey)
		systemPrompt := buildSystemPrompt(catalog)

		// Retry up to 3 times on OpenAI server errors
		var reply string
		var lastErr error
		for attempt := 1; attempt <= 3; attempt++ {
			aiResp, err := client.CreateChatCompletion(r.Context(), openai.ChatCompletionRequest{
				Model: "gpt-4o-mini",
				Messages: []openai.ChatCompletionMessage{
					{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
					{Role: openai.ChatMessageRoleUser, Content: req.Message},
				},
				MaxTokens:   500,
				Temperature: 0.6,
			})
			if err == nil {
				reply = aiResp.Choices[0].Message.Content
				lastErr = nil
				break
			}
			lastErr = err
			// Only retry on server-side errors (5xx), not client errors
			if attempt < 3 && (strings.Contains(err.Error(), "500") ||
				strings.Contains(err.Error(), "502") ||
				strings.Contains(err.Error(), "503")) {
				continue
			}
			break
		}

		if lastErr != nil {
			// Return a friendly fallback — still show matched products
			friendlyMsg := "😔 AI-ассистент временно недоступен. Попробуйте через несколько секунд.\n\n" +
				"Нашли подходящие товары по вашему запросу — нажмите на карточку для просмотра на ekt.kz.\n\n" +
				"Или позвоните нам: 📞 +7 (727) 346-88-88"
			json.NewEncoder(w).Encode(models.ChatResponse{
				Reply:    friendlyMsg,
				Products: matchedProducts,
			})
			return
		}

		json.NewEncoder(w).Encode(models.ChatResponse{
			Reply:    reply,
			Products: matchedProducts,
		})
	}
}
