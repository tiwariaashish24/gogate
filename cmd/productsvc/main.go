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

var products = []map[string]any{
	{"id": 1, "name": "Laptop", "price": 999.99},
	{"id": 2, "name": "Mouse", "price": 29.99},
	{"id": 3, "name": "Keyboard", "price": 79.99},
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
	go func() {
		regclient.RegisterWithRetry(context.Background(), envOr("GOGATE_REGISTRY_URL", "http://localhost:8080"), service, registerAddr, 2*time.Second)
		log.Printf("registered with gateway as %s@%s", service, registerAddr)
	}()
}

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"service": "products", "status": "ok"})
	})
	mux.HandleFunc("/products", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		writeJSON(w, http.StatusOK, products)
	})
	mux.HandleFunc("/products/", func(w http.ResponseWriter, r *http.Request) {
		for _, p := range products {
			if p["id"] == 1 && r.URL.Path == "/products/1" || p["id"] == 2 && r.URL.Path == "/products/2" || p["id"] == 3 && r.URL.Path == "/products/3" {
				writeJSON(w, http.StatusOK, p)
				return
			}
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "product not found"})
	})

	addr := envOr("PRODUCT_SERVICE_ADDR", ":8082")
	registerWithGateway("products", addr)
	log.Printf("product service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
