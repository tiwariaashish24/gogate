package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

var users = []map[string]any{
	{"id": 1, "name": "Alice", "email": "alice@example.com"},
	{"id": 2, "name": "Bob", "email": "bob@example.com"},
	{"id": 3, "name": "Carol", "email": "carol@example.com"},
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"service": "users", "status": "ok"})
	})
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		writeJSON(w, http.StatusOK, users)
	})
	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		for _, u := range users {
			if u["id"] == 1 && r.URL.Path == "/users/1" || u["id"] == 2 && r.URL.Path == "/users/2" || u["id"] == 3 && r.URL.Path == "/users/3" {
				writeJSON(w, http.StatusOK, u)
				return
			}
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
	})

	addr := envOr("USER_SERVICE_ADDR", ":8081")
	log.Printf("user service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
