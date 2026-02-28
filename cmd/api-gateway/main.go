package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mrtrkmn/orchi/api"
)

func main() {
	listenAddr := envOr("LISTEN_ADDR", ":8080")
	metricsAddr := envOr("METRICS_ADDR", ":8081")

	log.Printf("orchi-api-gateway starting (listen=%s, metrics=%s)", listenAddr, metricsAddr)

	var origins []string
	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		for _, o := range strings.Split(v, ",") {
			origins = append(origins, strings.TrimSpace(o))
		}
	}

	cfg := api.Config{
		SigningKey:         []byte(os.Getenv("JWT_SIGNING_KEY")),
		AllowedOrigins:     origins,
		RateLimitPerMinute: 120,
	}
	router := api.NewRouter(cfg)

	apiServer := &http.Server{Addr: listenAddr, Handler: router}

	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "# HELP orchi_api_requests_total Total API requests")
		fmt.Fprintln(w, "# TYPE orchi_api_requests_total counter")
		fmt.Fprintln(w, "orchi_api_requests_total 0")
	})
	metricsServer := &http.Server{Addr: metricsAddr, Handler: metricsMux}

	go func() {
		log.Printf("API gateway listening on %s", listenAddr)
		if err := apiServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("api server error: %v", err)
		}
	}()
	go func() {
		log.Printf("Metrics listening on %s", metricsAddr)
		if err := metricsServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("metrics server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop

	log.Println("Shutting down api-gateway...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	apiServer.Shutdown(ctx)
	metricsServer.Shutdown(ctx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
