// Command catalog is the platform service template: one binary running
// HTTP (chi), the outbox relay, and the Kafka consumer as errgroup
// goroutines with ordered graceful shutdown (D-04, D-39, D-40). Every real
// service starts life as a copy of this file — see plan 10's CLAUDE.md for
// exactly which files to rename/keep/delete.
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/chonlatee11/boat-booking/pkg/httpx"
	"github.com/chonlatee11/boat-booking/pkg/kafka"
	"github.com/chonlatee11/boat-booking/pkg/outbox"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
	httpadapter "github.com/chonlatee11/boat-booking/services/catalog/internal/adapters/http"
	kafkaadapter "github.com/chonlatee11/boat-booking/services/catalog/internal/adapters/kafka"
)

const serviceName = "catalog"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		os.Exit(runHealthcheck(httpx.EnvOr("HTTP_ADDR", ":8080")))
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// run boots the service and blocks until ctx is cancelled, then shuts down
// in the D-40 order: readyz -> 503, stop accepting HTTP, let the consumer
// finish its in-flight record, flush the relay only after HTTP has drained,
// then close Kafka and the DB pool.
func run(ctx context.Context) error {
	addr := httpx.EnvOr("HTTP_ADDR", ":8080")
	databaseURL := os.Getenv("DATABASE_URL")
	brokersEnv := os.Getenv("KAFKA_BROKERS")
	token := httpx.MustEnv("INTERNAL_TOKEN")

	relayEnabled, err := parseBoolEnv("RELAY_ENABLED", databaseURL != "")
	if err != nil {
		return fmt.Errorf("%s: %w", serviceName, err)
	}
	consumerEnabled, err := parseBoolEnv("CONSUMER_ENABLED", databaseURL != "")
	if err != nil {
		return fmt.Errorf("%s: %w", serviceName, err)
	}
	pollInterval, err := time.ParseDuration(httpx.EnvOr("OUTBOX_POLL_INTERVAL", "500ms"))
	if err != nil {
		return fmt.Errorf("%s: parse OUTBOX_POLL_INTERVAL: %w", serviceName, err)
	}

	if relayEnabled && databaseURL == "" {
		return fmt.Errorf("%s: RELAY_ENABLED requires DATABASE_URL", serviceName)
	}
	if consumerEnabled && (databaseURL == "" || brokersEnv == "") {
		return fmt.Errorf("%s: CONSUMER_ENABLED requires DATABASE_URL and KAFKA_BROKERS", serviceName)
	}

	log := httpx.NewLogger(serviceName)
	shutdownOTel, err := httpx.SetupOTel(ctx, serviceName)
	if err != nil {
		return fmt.Errorf("%s: setup otel: %w", serviceName, err)
	}

	var pool *pgxpool.Pool
	if databaseURL != "" {
		pool, err = bbpgx.NewPool(ctx, databaseURL)
		if err != nil {
			return fmt.Errorf("%s: new pool: %w", serviceName, err)
		}
	}

	var brokers []string
	if brokersEnv != "" {
		brokers = strings.Split(brokersEnv, ",")
	}

	var producer *kafka.Producer
	if len(brokers) > 0 {
		producer, err = kafka.NewProducer(brokers)
		if err != nil {
			return fmt.Errorf("%s: new producer: %w", serviceName, err)
		}
	}

	nudge := func() {}
	var relay *outbox.Relay
	if relayEnabled {
		relay = outbox.NewRelay(pool, producer, log)
		relay.PollInterval = pollInterval
		nudge = relay.Nudge
	}

	var consumer *kafka.Consumer
	if consumerEnabled {
		consumer = kafka.NewConsumer(brokers, serviceName, pool, producer, log)
		kafkaadapter.Register(consumer)
	}

	checks := map[string]func(context.Context) error{}
	if pool != nil {
		checks["db"] = pool.Ping
	}
	if producer != nil {
		checks["kafka"] = producer.Ping
	}
	ready := httpx.NewReadiness(checks)

	r := chi.NewRouter()
	r.Get("/healthz", httpx.Healthz)
	r.Get("/readyz", ready.Handler)
	r.Group(func(pr chi.Router) {
		pr.Use(httpx.RequireInternal(token))
		httpadapter.Routes(pr, pool, nudge)
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           httpx.Instrument(serviceName, r),
		ReadHeaderTimeout: 5 * time.Second,
	}

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("%s: http server: %w", serviceName, err)
		}
		return nil
	})

	relayCtx, cancelRelay := context.WithCancel(context.Background())
	defer cancelRelay()
	if relay != nil {
		g.Go(func() error { return relay.Run(relayCtx) })
	}
	if consumer != nil {
		g.Go(func() error { return consumer.Run(gctx) })
	}

	g.Go(func() error {
		<-gctx.Done()
		ready.SetShuttingDown()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(gctx), 15*time.Second)
		defer cancel()
		shutdownErr := srv.Shutdown(shutdownCtx)
		cancelRelay()
		return shutdownErr
	})

	runErr := g.Wait()

	if producer != nil {
		producer.Close()
	}
	if pool != nil {
		pool.Close()
	}
	if err := shutdownOTel(context.Background()); err != nil {
		log.Error("shutdown otel failed", "error", err)
	}

	return runErr
}

// parseBoolEnv reads key as a strconv.ParseBool value, defaulting to def
// when unset.
func parseBoolEnv(key string, def bool) (bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}
	switch v {
	case "true", "1":
		return true, nil
	case "false", "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid %s %q: must be true/false", key, v)
	}
}

func runHealthcheck(addr string) int {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 1
	}
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/readyz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close() //nolint:errcheck // healthcheck path, nothing actionable
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
