package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gogate/internal/registryclient"
)

var orders = []map[string]any{
	{"id": 1, "user_id": 1, "product_id": 1, "total": 999.99, "status": "shipped"},
	{"id": 2, "user_id": 2, "product_id": 2, "total": 29.99, "status": "pending"},
	{"id": 3, "user_id": 1, "product_id": 3, "total": 79.99, "status": "delivered"},
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

func registerWithGateway(service, addr string) {
	registerAddr := envOr("REGISTER_ADDRESS", "")
	if registerAddr == "" {
		if strings.HasPrefix(addr, ":") {
			registerAddr = "localhost" + addr
		} else {
			registerAddr = addr
		}
	}
	go regclient.RegisterWithRetry(context.Background(), envOr("GOGATE_REGISTRY_URL", "http://localhost:8080"), service, registerAddr, 5*time.Second)
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"service": "orders", "status": "ok"})
	})
	mux.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		writeJSON(w, http.StatusOK, orders)
	})
	mux.HandleFunc("/orders/", func(w http.ResponseWriter, r *http.Request) {
		for _, o := range orders {
			if o["id"] == 1 && r.URL.Path == "/orders/1" || o["id"] == 2 && r.URL.Path == "/orders/2" || o["id"] == 3 && r.URL.Path == "/orders/3" {
				writeJSON(w, http.StatusOK, o)
				return
			}
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "order not found"})
	})

	addr := envOr("ORDER_SERVICE_ADDR", ":8083")
	registerWithGateway("orders", addr)
	log.Printf("order service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
