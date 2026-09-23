package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
	openai "github.com/sashabaranov/go-openai"
)

// buildSystemPrompt creates the AI system prompt using the menu data
func buildSystemPrompt(menu *models.Menu) string {
	prompt := fmt.Sprintf(`Ты — дружелюбный AI-ассистент ресторана "%s".

О ресторане:
- Адрес: %s
- Телефон: %s
- Режим работы: %s
- Описание: %s

Твоя задача — помогать клиентам:
1. Отвечать на вопросы о меню (состав, цены, вес блюд)
2. Рекомендовать блюда по предпочтениям
3. Информировать об акциях и комбо-предложениях
4. Отвечать на вопросы о режиме работы и контактах

Полное меню:
`, menu.Restaurant.Name, menu.Restaurant.Address,
		menu.Restaurant.Phone, menu.Restaurant.WorkingHours,
		menu.Restaurant.Description)

	for _, cat := range menu.Categories {
		prompt += fmt.Sprintf("\n## %s\n", cat.Name)
		for _, item := range cat.Items {
			spicyTag := ""
			if item.Spicy {
				spicyTag = " 🌶️ (острое)"
			}
			prompt += fmt.Sprintf("- %s%s — %g тг (%s): %s\n",
				item.Name, spicyTag, item.Price, item.Weight, item.Description)
		}
	}

	prompt += `
Правила общения:
- Общайся вежливо и дружелюбно
- Отвечай кратко и по делу
- Используй эмодзи умеренно 🌯
- Если не знаешь ответа — предложи позвонить в ресторан
- Отвечай на русском языке (если пользователь не пишет на другом языке)
`
	return prompt
}

// ChatHandler handles POST /api/chat requests
func ChatHandler(menu *models.Menu) http.HandlerFunc {
	apiKey := os.Getenv("OPENAI_API_KEY")

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Method not allowed"})
			return
		}

		var req models.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Invalid request body"})
			return
		}

		if req.Message == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "Message cannot be empty"})
			return
		}

		// If no API key, return a demo response
		if apiKey == "" {
			demoReply := demoResponse(req.Message, menu)
			json.NewEncoder(w).Encode(models.ChatResponse{Reply: demoReply})
			return
		}

		client := openai.NewClient(apiKey)
		systemPrompt := buildSystemPrompt(menu)

		resp, err := client.CreateChatCompletion(r.Context(), openai.ChatCompletionRequest{
			Model: "gpt-4o-mini",
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleSystem,
					Content: systemPrompt,
				},
				{
					Role:    openai.ChatMessageRoleUser,
					Content: req.Message,
				},
			},
			MaxTokens:   500,
			Temperature: 0.7,
		})

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(models.ErrorResponse{Error: "AI service error: " + err.Error()})
			return
		}

		reply := resp.Choices[0].Message.Content
		json.NewEncoder(w).Encode(models.ChatResponse{Reply: reply})
	}
}

// demoResponse returns a simple rule-based response when no API key is set
func demoResponse(message string, menu *models.Menu) string {
	// Simple keyword matching for demo mode
	msg := message

	// Check for common keywords
	for _, cat := range menu.Categories {
		for _, item := range cat.Items {
			_ = item
		}
	}

	_ = msg

	return fmt.Sprintf(
		"👋 Привет! Я AI-ассистент %s. "+
			"(Демо-режим: добавьте OPENAI_API_KEY для полноценного ИИ)\n\n"+
			"Наше меню доступно на сайте. Звоните: %s",
		menu.Restaurant.Name, menu.Restaurant.Phone,
	)
}
