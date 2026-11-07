package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gogate/internal/proxy"
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	p, err := proxy.New([]proxy.Route{
		{Prefix: "/api/users", Target: envOr("GOGATE_USER_TARGET", "http://localhost:8081"), Strip: "/api"},
		{Prefix: "/api/products", Target: envOr("GOGATE_PRODUCT_TARGET", "http://localhost:8082"), Strip: "/api"},
		{Prefix: "/api/orders", Target: envOr("GOGATE_ORDER_TARGET", "http://localhost:8083"), Strip: "/api"},
	}, 10*time.Second)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.Handle("/", p)

	addr := envOr("GOGATE_ADDR", ":8080")
	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		fmt.Printf("GoGate running on %s\n", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
