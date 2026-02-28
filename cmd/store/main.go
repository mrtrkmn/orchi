package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	listenAddr := envOr("STORE_LISTEN_ADDR", ":5454")
	metricsAddr := envOr("STORE_METRICS_ADDR", ":9090")
	dataDir := envOr("STORE_DATA_DIR", "/data")

	log.Printf("orchi-store starting (listen=%s, metrics=%s, data=%s)", listenAddr, metricsAddr, dataDir)

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Printf("Warning: could not create data dir %s: %v", dataDir, err)
	}

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", listenAddr, err)
	}

	go func() {
		log.Printf("Store gRPC listening on %s", listenAddr)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "# HELP orchi_store_up Store is running")
		fmt.Fprintln(w, "# TYPE orchi_store_up gauge")
		fmt.Fprintln(w, "orchi_store_up 1")
	})
	metricsServer := &http.Server{Addr: metricsAddr, Handler: metricsMux}

	go func() {
		log.Printf("Metrics listening on %s", metricsAddr)
		if err := metricsServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("metrics error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop

	log.Println("Shutting down store...")
	ln.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	metricsServer.Shutdown(ctx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
