package handlers

import (
	"sync"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
	"github.com/sashabaranov/go-openai"
)

const maxConversationMessages = 12

type pendingPurchase struct {
	Product  models.Product
	Quantity int
}

type conversation struct {
	messages       []openai.ChatCompletionMessage
	recentProducts []models.Product
	pending        *pendingPurchase
}

// ConversationStore keeps a small amount of context per browser session so
// follow-up questions such as "what is this used for?" retain the selected item.
type ConversationStore struct {
	mu            sync.RWMutex
	conversations map[string]conversation
}

func NewConversationStore() *ConversationStore {
	return &ConversationStore{conversations: make(map[string]conversation)}
}

func (s *ConversationStore) get(sessionID string) ([]openai.ChatCompletionMessage, []models.Product) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry := s.conversations[sessionID]
	messages := append([]openai.ChatCompletionMessage(nil), entry.messages...)
	products := append([]models.Product(nil), entry.recentProducts...)
	return messages, products
}

func (s *ConversationStore) append(sessionID string, userMessage, assistantMessage string, products []models.Product) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.conversations[sessionID]
	entry.messages = append(entry.messages,
		openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: userMessage},
		openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: assistantMessage},
	)
	if len(entry.messages) > maxConversationMessages {
		entry.messages = entry.messages[len(entry.messages)-maxConversationMessages:]
	}
	if len(products) > 4 {
		products = products[:4]
	}
	entry.recentProducts = append([]models.Product(nil), products...)
	s.conversations[sessionID] = entry
}


func (s *ConversationStore) setPending(sessionID string, product models.Product, quantity int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.conversations[sessionID]
	entry.pending = &pendingPurchase{Product: product, Quantity: quantity}
	s.conversations[sessionID] = entry
}

func (s *ConversationStore) getPending(sessionID string) *pendingPurchase {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry := s.conversations[sessionID]
	if entry.pending == nil {
		return nil
	}
	copyPending := *entry.pending
	return &copyPending
}

func (s *ConversationStore) clearPending(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.conversations[sessionID]
	entry.pending = nil
	s.conversations[sessionID] = entry
}
