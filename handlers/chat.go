package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

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
5. Помогать с оформлением заказа

Важно:
- Все цены в тенге (тг / KZT)
- Если клиент спрашивает конкретный товар — ищи в каталоге ниже
- Если товара нет в каталоге — предложи обратиться на сайт ekt.kz или позвонить
- Отвечай на русском (или на языке клиента)
- Будь лаконичным, профессиональным и дружелюбным

`

// buildSystemPrompt creates the AI prompt with embedded product data
func buildSystemPrompt(catalog *models.Catalog) string {
	prompt := systemPromptBase

	if len(catalog.Products) > 0 {
		prompt += "## Примеры товаров из каталога:\n"
		// Include up to 50 products in context
		limit := len(catalog.Products)
		if limit > 50 {
			limit = 50
		}
		for _, p := range catalog.Products[:limit] {
			prompt += fmt.Sprintf("- [Арт: %s] %s — %.0f тг | %s\n",
				p.Article, p.Name, p.Price, p.URL)
		}
	}

	if catalog.Detail != nil {
		d := catalog.Detail
		prompt += fmt.Sprintf(`
## Пример детальной информации о товаре:
Артикул: %s
Название: %s
Цена: %.0f тг
Описание: %s
Наличие (общее): %d шт.
`, d.Article, d.Name, d.Price, d.Description, d.Quantity)

		// Show stores with stock
		var inStock []string
		for _, s := range d.Stores {
			if s.Quantity > 0 {
				inStock = append(inStock, fmt.Sprintf("%s: %d шт.", s.Name, s.Quantity))
			}
		}
		if len(inStock) > 0 {
			prompt += "Наличие по складам: " + strings.Join(inStock, ", ") + "\n"
		}
	}

	return prompt
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

		// Demo mode fallback
		if apiKey == "" {
			json.NewEncoder(w).Encode(models.ChatResponse{
				Reply: "Здравствуйте! 👋 Я AI-ассистент ГК Электрокомплект.\n\n" +
					"(Демо-режим — настройте OPENAI_API_KEY для полноценного ИИ)\n\n" +
					"Вы можете найти нужный товар на сайте ekt.kz или позвоните нам:\n" +
					"📞 +7 (727) 346-88-88",
			})
			return
		}

		client := openai.NewClient(apiKey)
		systemPrompt := buildSystemPrompt(catalog)

		resp, err := client.CreateChatCompletion(r.Context(), openai.ChatCompletionRequest{
			Model: "gpt-4o-mini",
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
				{Role: openai.ChatMessageRoleUser, Content: req.Message},
			},
			MaxTokens:   600,
			Temperature: 0.6,
		})

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "AI error: " + err.Error()})
			return
		}

		json.NewEncoder(w).Encode(models.ChatResponse{
			Reply: resp.Choices[0].Message.Content,
		})
	}
}
