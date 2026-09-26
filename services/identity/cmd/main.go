// Command identity is the passwordless OTP login service (D-01, D-02,
// D-03): HTTP (chi), the outbox relay, and the Kafka consumer running as
// errgroup goroutines with ordered graceful shutdown, following the
// services/_template shape (D-04, D-39, D-40).
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
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"

	"github.com/chonlatee11/boat-booking/gen/go/catalog/v1/catalogv1connect"
	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
	"github.com/chonlatee11/boat-booking/pkg/kafka"
	"github.com/chonlatee11/boat-booking/pkg/outbox"
	bbpgx "github.com/chonlatee11/boat-booking/pkg/pgx"
	httpadapter "github.com/chonlatee11/boat-booking/services/identity/internal/adapters/http"
	kafkaadapter "github.com/chonlatee11/boat-booking/services/identity/internal/adapters/kafka"
	"github.com/chonlatee11/boat-booking/services/identity/internal/adapters/notify"
	"github.com/chonlatee11/boat-booking/services/identity/internal/app"
)

const serviceName = "identity"

// minOTPHashSecretLen guards against a trivially short OTP_HASH_SECRET
// pepper (D-02, Pitfall 4).
const minOTPHashSecretLen = 32

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
// then close Kafka, Valkey, and the DB pool.
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

	rdb := redis.NewClient(&redis.Options{Addr: httpx.MustEnv("VALKEY_ADDR")})

	privKey, err := auth.ParsePrivateKeyB64(httpx.MustEnv("JWT_PRIVATE_KEY_B64"))
	if err != nil {
		return fmt.Errorf("%s: %w", serviceName, err)
	}
	issuer := auth.NewIssuer(privKey, httpx.EnvOr("JWT_ISSUER", "boatbooking-dev"))

	pepper := httpx.MustEnv("OTP_HASH_SECRET")
	if len(pepper) < minOTPHashSecretLen {
		return fmt.Errorf("%s: OTP_HASH_SECRET must be at least %d bytes", serviceName, minOTPHashSecretLen)
	}

	emailSender, smsSender, err := buildSenders()
	if err != nil {
		return fmt.Errorf("%s: %w", serviceName, err)
	}

	authSvc := &app.Auth{
		Pool:   pool,
		RDB:    rdb,
		Pepper: []byte(pepper),
		Email:  emailSender,
		SMS:    smsSender,
		Issuer: issuer,
		Nudge:  nudge,
	}

	otelOpt, err := httpx.ConnectOtel()
	if err != nil {
		return fmt.Errorf("%s: %w", serviceName, err)
	}
	catalogURL := httpx.EnvOr("CATALOG_URL", "http://catalog:8080")
	usersSvc := &app.Users{
		Pool:          pool,
		Catalog:       catalogv1connect.NewCatalogServiceClient(httpx.NewHTTPClient(5*time.Second), catalogURL, otelOpt),
		InternalToken: token,
		Nudge:         nudge,
	}

	// D-09: the operator-controlled SUPER_ADMIN_EMAIL is the only way a
	// super_admin ever comes to exist — no API or CLI path creates one.
	// Runs before the HTTP server starts accepting traffic; a failure here
	// aborts startup rather than serving without the guaranteed super_admin.
	if err := app.EnsureSuperAdmin(ctx, pool, httpx.MustEnv("SUPER_ADMIN_EMAIL")); err != nil {
		return fmt.Errorf("%s: ensure super admin: %w", serviceName, err)
	}

	checks := map[string]func(context.Context) error{}
	if pool != nil {
		checks["db"] = pool.Ping
	}
	if producer != nil {
		checks["kafka"] = producer.Ping
	}
	checks["valkey"] = func(ctx context.Context) error { return rdb.Ping(ctx).Err() }
	ready := httpx.NewReadiness(checks)

	r := chi.NewRouter()
	r.Get("/healthz", httpx.Healthz)
	r.Get("/readyz", ready.Handler)
	r.Group(func(pr chi.Router) {
		pr.Use(httpx.RequireInternal(token))
		httpadapter.Routes(pr, authSvc, usersSvc)
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

	_ = rdb.Close()
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

// buildSenders picks the OTP email/SMS senders from env: SMTP_ADDR set means
// the dev/CI transport (email via SMTP to Mailpit, phone via the dev SMS
// sender to the same Mailpit); otherwise RESEND_API_KEY selects the prod
// email transport with no SMS sender (phone login unavailable in prod until
// a real provider is chosen, deferred per CONTEXT). Neither set is a startup
// error — there is no silent no-op delivery configuration.
func buildSenders() (email, sms notify.Sender, err error) {
	otpFrom := httpx.EnvOr("OTP_EMAIL_FROM", "Boat Booking <no-reply@boatbooking.local>")

	if smtpAddr := httpx.EnvOr("SMTP_ADDR", ""); smtpAddr != "" {
		smtpSender := notify.SMTPSender{Addr: smtpAddr, From: otpFrom}
		return smtpSender, notify.DevSMSSender{SMTP: smtpSender}, nil
	}
	if apiKey := httpx.EnvOr("RESEND_API_KEY", ""); apiKey != "" {
		return notify.ResendSender{APIKey: apiKey, From: otpFrom}, nil, nil
	}
	return nil, nil, errors.New("identity: no OTP email sender configured (set SMTP_ADDR or RESEND_API_KEY)")
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
