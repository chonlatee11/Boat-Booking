// Command gateway is the thin BFF behind Kong: it re-verifies the access
// JWT and, from Phase 2 onward, forwards trusted claim headers to downstream
// services (D-29, D-37).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/chonlatee11/boat-booking/pkg/auth"
	"github.com/chonlatee11/boat-booking/pkg/httpx"
	bffhttp "github.com/chonlatee11/boat-booking/services/gateway/internal/adapters/http"
)

func main() {
	addr := httpx.EnvOr("HTTP_ADDR", ":8080")

	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		os.Exit(runHealthcheck(addr))
	}

	pubB64 := httpx.MustEnv("JWT_PUBLIC_KEY_B64")
	issuer := httpx.EnvOr("JWT_ISSUER", "boatbooking-dev")

	pub, err := auth.ParsePublicKeyB64(pubB64)
	if err != nil {
		fatalf("parse JWT_PUBLIC_KEY_B64: %v", err)
	}
	verifier := auth.NewVerifier(pub, issuer)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeStatusOK(w)
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		writeStatusOK(w)
	})

	bffhttp.Routes(r, verifier)

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		fatalf("server error: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		fatalf("shutdown error: %v", err)
	}
}

func writeStatusOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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

func fatalf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}
