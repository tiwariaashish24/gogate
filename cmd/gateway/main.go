package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"gogate/internal/gateway"
	"gogate/internal/proxy"
	"gogate/internal/registry"
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	reg := registry.New(envDuration("GOGATE_HEALTH_INTERVAL", 5*time.Second), envInt("GOGATE_HEALTH_MAX_FAILURES", 3))
	go reg.Start(ctx)

	p, err := proxy.New([]proxy.Route{
		{Prefix: "/api/users", Service: "users", Strip: "/api"},
		{Prefix: "/api/products", Service: "products", Strip: "/api"},
		{Prefix: "/api/orders", Service: "orders", Strip: "/api"},
	}, reg, envDuration("GOGATE_PROXY_TIMEOUT", 10*time.Second))
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.Handle("/registry/", gateway.RegistryAPI(reg))
	mux.Handle("/", p)

	addr := envOr("GOGATE_ADDR", ":8080")
	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		fmt.Printf("GoGate running on %s\n", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
