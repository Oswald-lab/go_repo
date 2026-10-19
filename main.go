package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func newRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/hello", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Hello from Go (chi)"})
	})
	r.Get("/id", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(uuid.NewString()))
	})
	return r
}

func main() {
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", newRouter()))
}
