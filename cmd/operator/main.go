package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	leaderElect := flag.Bool("leader-elect", false, "Enable leader election")
	metricsAddr := flag.String("metrics-bind-address", ":8080", "Metrics bind address")
	healthAddr := flag.String("health-probe-bind-address", ":8081", "Health probe bind address")
	flag.Parse()

	log.Printf("orchi-operator starting (leader-elect=%v, metrics=%s, health=%s)",
		*leaderElect, *metricsAddr, *healthAddr)

	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	healthMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	healthServer := &http.Server{Addr: *healthAddr, Handler: healthMux}

	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "# HELP orchi_operator_up Operator is running")
		fmt.Fprintln(w, "# TYPE orchi_operator_up gauge")
		fmt.Fprintln(w, "orchi_operator_up 1")
	})
	metricsServer := &http.Server{Addr: *metricsAddr, Handler: metricsMux}

	go func() {
		log.Printf("Health probes listening on %s", *healthAddr)
		if err := healthServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("health server error: %v", err)
		}
	}()
	go func() {
		log.Printf("Metrics listening on %s", *metricsAddr)
		if err := metricsServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("metrics server error: %v", err)
		}
	}()

	log.Println("orchi-operator ready")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop

	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	healthServer.Shutdown(ctx)
	metricsServer.Shutdown(ctx)
}
