package handlers

import (
	"sync"
	"time"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
	"github.com/sashabaranov/go-openai"
)

const maxConversationMessages = 12

type pendingPurchase struct {
	Token    string
	Expires  time.Time
	Product  models.Product
	Quantity int
	Items    []pendingLine
}

type pendingLine struct {
	Product  models.Product
	Quantity int
}

type conversation struct {
	expires        time.Time
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
	if time.Now().After(entry.expires) {
		return nil, nil
	}
	messages := append([]openai.ChatCompletionMessage(nil), entry.messages...)
	products := append([]models.Product(nil), entry.recentProducts...)
	return messages, products
}

func (s *ConversationStore) append(sessionID string, userMessage, assistantMessage string, products []models.Product) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.conversations[sessionID]
	if time.Now().After(entry.expires) {
		entry = conversation{}
	}
	entry.expires = time.Now().Add(30 * time.Minute)
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
	s.setPendingOrder(sessionID, []pendingLine{{Product: product, Quantity: quantity}})
}

func (s *ConversationStore) setPendingOrder(sessionID string, lines []pendingLine) string {
	token, err := randomToken(24)
	if err != nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.conversations[sessionID]
	entry.expires = time.Now().Add(30 * time.Minute)
	if len(lines) == 0 {
		entry.pending = nil
	} else {
		entry.pending = &pendingPurchase{
			Token: token, Expires: time.Now().Add(10 * time.Minute),
			Product:  lines[0].Product,
			Quantity: lines[0].Quantity,
			Items:    append([]pendingLine(nil), lines...),
		}
	}
	s.conversations[sessionID] = entry
	return token
}

func (s *ConversationStore) getPending(sessionID string) *pendingPurchase {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry := s.conversations[sessionID]
	if entry.pending == nil || time.Now().After(entry.pending.Expires) {
		return nil
	}
	copyPending := *entry.pending
	copyPending.Items = append([]pendingLine(nil), entry.pending.Items...)
	return &copyPending
}

// takePending consumes the exact server-side selection under one lock. Model
// prose and conversation history never grant authority to mutate a cart.
func (s *ConversationStore) takePending(sessionID, token string) *pendingPurchase {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.conversations[sessionID]
	pending := entry.pending
	if pending == nil || time.Now().After(pending.Expires) || token != "" && token != pending.Token {
		return nil
	}
	entry.pending = nil
	s.conversations[sessionID] = entry
	return pending
}

func (s *ConversationStore) forget(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conversations, sessionID)
}

// Cleanup runs in the server, so expired history is erased even without another request.
func Cleanup(carts *CartStore, conversations *ConversationStore, auth *AuthStore) {
	carts.pruneExpired()
	conversations.mu.Lock()
	for id, entry := range conversations.conversations {
		if time.Now().After(entry.expires) {
			delete(conversations.conversations, id)
		}
	}
	conversations.mu.Unlock()
	auth.mu.Lock()
	for id, expires := range auth.expires {
		if time.Now().After(expires) {
			delete(auth.expires, id)
			delete(auth.sessions, id)
		}
	}
	auth.mu.Unlock()
}

func (s *ConversationStore) clearPending(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := s.conversations[sessionID]
	entry.pending = nil
	s.conversations[sessionID] = entry
}
