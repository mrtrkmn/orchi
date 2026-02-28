package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	listenAddr := envOr("LISTEN_ADDR", ":8443")
	metricsAddr := envOr("METRICS_ADDR", ":8081")

	log.Printf("orchi-vnc-proxy starting (listen=%s, metrics=%s)", listenAddr, metricsAddr)

	mux := http.NewServeMux()
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/vnc", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "VNC proxy not yet implemented", http.StatusNotImplemented)
	})
	server := &http.Server{Addr: listenAddr, Handler: mux}

	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "# HELP orchi_vnc_active_connections Active VNC connections")
		fmt.Fprintln(w, "# TYPE orchi_vnc_active_connections gauge")
		fmt.Fprintln(w, "orchi_vnc_active_connections 0")
	})
	metricsServer := &http.Server{Addr: metricsAddr, Handler: metricsMux}

	go func() {
		log.Printf("VNC proxy listening on %s", listenAddr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("vnc-proxy error: %v", err)
		}
	}()
	go func() {
		log.Printf("Metrics listening on %s", metricsAddr)
		if err := metricsServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("metrics error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop

	log.Println("Shutting down vnc-proxy...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(ctx)
	metricsServer.Shutdown(ctx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
