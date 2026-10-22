package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type user struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`
	Salt         string `json:"-"`
}

func hashPassword(password, salt string) string {
	sum := sha256.Sum256([]byte(salt + password))
	return hex.EncodeToString(sum[:])
}

func newPasswordHash(password string) (hash, salt string, err error) {
	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
		return "", "", err
	}
	salt = hex.EncodeToString(saltBytes)
	return hashPassword(password, salt), salt, nil
}

func (u user) passwordMatches(password string) bool {
	got := hashPassword(password, u.Salt)
	return subtle.ConstantTimeCompare([]byte(got), []byte(u.PasswordHash)) == 1
}

type userStore struct {
	mu    sync.RWMutex
	users map[string]user
}

func newUserStore() *userStore {
	return &userStore{users: make(map[string]user)}
}

func (s *userStore) list() []user {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]user, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u)
	}
	return users
}

func (s *userStore) create(u user) user {
	s.mu.Lock()
	defer s.mu.Unlock()

	u.ID = uuid.NewString()
	s.users[u.ID] = u
	return u
}

func (s *userStore) get(id string) (user, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.users[id]
	return u, ok
}

func (s *userStore) findByEmail(email string) (user, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, u := range s.users {
		if strings.EqualFold(u.Email, email) {
			return u, true
		}
	}
	return user{}, false
}

func (s *userStore) delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[id]; !ok {
		return false
	}
	delete(s.users, id)
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func apiKeyAuth(next http.Handler) http.Handler {
	key := os.Getenv("API_KEY")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if key == "" {
			writeError(w, http.StatusInternalServerError, "server authentication is not configured")
			return
		}
		if r.Header.Get("X-API-Key") != key {
			writeError(w, http.StatusUnauthorized, "missing or invalid API key")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newRouter() http.Handler {
	r := chi.NewRouter()
	store := newUserStore()

	r.Get("/hello", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Hello from Go (chi)"})
	})
	r.Get("/id", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(uuid.NewString()))
	})

	auth := r.With(apiKeyAuth)
	auth.Get("/users", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, store.list())
	})
	auth.Post("/users", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Name     string `json:"name"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		input.Name = strings.TrimSpace(input.Name)
		input.Email = strings.TrimSpace(input.Email)
		if input.Name == "" || input.Email == "" || input.Password == "" {
			writeError(w, http.StatusBadRequest, "name, email and password are required")
			return
		}
		if _, ok := store.findByEmail(input.Email); ok {
			writeError(w, http.StatusConflict, "a user with that email already exists")
			return
		}

		hash, salt, err := newPasswordHash(input.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not secure password")
			return
		}

		writeJSON(w, http.StatusCreated, store.create(user{
			Name:         input.Name,
			Email:        input.Email,
			PasswordHash: hash,
			Salt:         salt,
		}))
	})
	auth.Get("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		u, ok := store.get(chi.URLParam(r, "id"))
		if !ok {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeJSON(w, http.StatusOK, u)
	})
	auth.Delete("/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !store.delete(chi.URLParam(r, "id")) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	r.Post("/login", func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		input.Email = strings.TrimSpace(input.Email)
		if input.Email == "" || input.Password == "" {
			writeError(w, http.StatusBadRequest, "email and password are required")
			return
		}

		u, ok := store.findByEmail(input.Email)
		if !ok || !u.passwordMatches(input.Password) {
			writeError(w, http.StatusUnauthorized, "invalid email or password")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"message": "login successful",
			"user":    u,
		})
	})

	return r
}

func main() {
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", newRouter()))
}
