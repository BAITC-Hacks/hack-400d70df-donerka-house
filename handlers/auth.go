package handlers

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/BAITC-Hacks/hack-400d70df-donerka-house/models"
)

const (
	authCookieName       = "ekt_auth"
	accountMinPassword   = 8
	passwordHashRounds   = 120000
	maxAccountNameLength = 80
)

type accountRecord struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	Salt         string    `json:"salt"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

// AuthStore provides local accounts for the demo application. The account
// file contains salted password hashes only; it never contains plaintext
// passwords or the EKT service Basic Auth credentials.
type AuthStore struct {
	mu       sync.RWMutex
	users    map[string]accountRecord
	sessions map[string]string
	path     string
}

// NewAuthStore creates a file-backed account store. Set ACCOUNT_STORE_PATH to
// change the location; an empty path keeps accounts in memory, which is useful
// for tests.
func NewAuthStore(path string) (*AuthStore, error) {
	store := &AuthStore{
		users:    make(map[string]accountRecord),
		sessions: make(map[string]string),
		path:     strings.TrimSpace(path),
	}
	if store.path == "" {
		store.path = "data/accounts.json"
	}

	data, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read account store: %w", err)
	}
	var records []accountRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("parse account store: %w", err)
	}
	for _, record := range records {
		if record.ID != "" && record.Email != "" && record.PasswordHash != "" && record.Salt != "" {
			store.users[record.Email] = record
		}
	}
	return store, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validEmail(email string) bool {
	if len(email) < 5 || len(email) > 254 || strings.Count(email, "@") != 1 {
		return false
	}
	parts := strings.SplitN(email, "@", 2)
	return parts[0] != "" && parts[1] != "" && !strings.ContainsAny(email, " \t\r\n")
}

func randomToken(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func hashPassword(password, salt string) string {
	key := []byte(password)
	message := []byte(salt)
	var digest []byte
	for round := 0; round < passwordHashRounds; round++ {
		mac := hmac.New(sha256.New, key)
		mac.Write(message)
		digest = mac.Sum(nil)
		key = digest
		message = []byte(salt)
	}
	return hex.EncodeToString(digest)
}

func publicUser(record accountRecord) *models.User {
	return &models.User{ID: record.ID, Email: record.Email, Name: record.Name}
}

func (s *AuthStore) saveLocked() error {
	if s.path == "" {
		return nil
	}
	records := make([]accountRecord, 0, len(s.users))
	for _, record := range s.users {
		records = append(records, record)
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return err
	}
	temporary := s.path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, s.path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func (s *AuthStore) setSession(w http.ResponseWriter, r *http.Request, userID string) error {
	token, err := randomToken(32)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.sessions[token] = userID
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		MaxAge:   60 * 60 * 24 * 30,
	})
	return nil
}

func (s *AuthStore) clearSession(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(authCookieName); err == nil {
		s.mu.Lock()
		delete(s.sessions, cookie.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     authCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		MaxAge:   -1,
	})
}

// User returns the account associated with the current request, if any.
func (s *AuthStore) User(r *http.Request) *models.User {
	cookie, err := r.Cookie(authCookieName)
	if err != nil || cookie.Value == "" {
		return nil
	}
	s.mu.RLock()
	userID := s.sessions[cookie.Value]
	for _, record := range s.users {
		if record.ID == userID {
			user := publicUser(record)
			s.mu.RUnlock()
			return user
		}
	}
	s.mu.RUnlock()
	return nil
}

// Register creates a local account and logs the browser into it.
func (s *AuthStore) Register(email, name, password string) (*models.User, error) {
	email = normalizeEmail(email)
	name = strings.TrimSpace(name)
	if !validEmail(email) {
		return nil, errors.New("Введите корректный email")
	}
	if len([]rune(password)) < accountMinPassword {
		return nil, fmt.Errorf("Пароль должен содержать минимум %d символов", accountMinPassword)
	}
	if name == "" {
		name = strings.Split(email, "@")[0]
	}
	if len([]rune(name)) > maxAccountNameLength {
		return nil, errors.New("Имя слишком длинное")
	}
	for _, character := range name {
		if unicode.IsControl(character) {
			return nil, errors.New("Имя содержит недопустимые символы")
		}
	}
	salt, err := randomToken(16)
	if err != nil {
		return nil, errors.New("Не удалось создать аккаунт")
	}
	id, err := randomToken(18)
	if err != nil {
		return nil, errors.New("Не удалось создать аккаунт")
	}
	record := accountRecord{
		ID:           id,
		Email:        email,
		Name:         name,
		Salt:         salt,
		PasswordHash: hashPassword(password, salt),
		CreatedAt:    time.Now().UTC(),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[email]; exists {
		return nil, errors.New("Аккаунт с таким email уже существует")
	}
	s.users[email] = record
	if err := s.saveLocked(); err != nil {
		delete(s.users, email)
		return nil, errors.New("Не удалось сохранить аккаунт")
	}
	return publicUser(record), nil
}

// Login validates a local account password.
func (s *AuthStore) Login(email, password string) (*models.User, error) {
	email = normalizeEmail(email)
	s.mu.RLock()
	record, exists := s.users[email]
	s.mu.RUnlock()
	if !exists || !hmac.Equal([]byte(record.PasswordHash), []byte(hashPassword(password, record.Salt))) {
		return nil, errors.New("Неверный email или пароль")
	}
	return publicUser(record), nil
}

// AuthHandler exposes GET/POST/DELETE /api/auth for the localhost account.
func AuthHandler(auth *AuthStore, carts *CartStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			user := auth.User(r)
			writeJSON(w, models.AuthResponse{Authenticated: user != nil, User: user})
		case http.MethodPost:
			var request models.AuthRequest
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
			if err := decoder.Decode(&request); err != nil {
				writeJSONError(w, http.StatusBadRequest, "Некорректные данные аккаунта")
				return
			}
			var user *models.User
			var err error
			switch strings.ToLower(strings.TrimSpace(request.Action)) {
			case "register":
				user, err = auth.Register(request.Email, request.Name, request.Password)
			case "login":
				user, err = auth.Login(request.Email, request.Password)
			default:
				writeJSONError(w, http.StatusBadRequest, "Неизвестное действие аккаунта")
				return
			}
			if err != nil {
				status := http.StatusBadRequest
				if strings.Contains(err.Error(), "уже существует") {
					status = http.StatusConflict
				}
				writeJSONError(w, status, err.Error())
				return
			}
			if err := auth.setSession(w, r, user.ID); err != nil {
				writeJSONError(w, http.StatusInternalServerError, "Не удалось создать сессию аккаунта")
				return
			}
			if carts != nil {
				if err := carts.MergeSessionIntoUser(w, r, user.ID); err != nil {
					writeJSONError(w, http.StatusInternalServerError, "Не удалось перенести корзину в аккаунт")
					return
				}
			}
			writeJSON(w, models.AuthResponse{Authenticated: true, User: user})
		case http.MethodDelete:
			auth.clearSession(w, r)
			writeJSON(w, models.AuthResponse{Authenticated: false})
		default:
			writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
	}
}
