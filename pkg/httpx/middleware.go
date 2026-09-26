package httpx

import (
	"log/slog"
	"net/http"

	"github.com/chonlatee11/boat-booking/pkg/clock"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Instrument wraps h with OTel HTTP server tracing (D-50) and a
// method/path/status/duration request log line (D-45, D-52). /healthz and
// /readyz are excluded from tracing (WithFilter) and logged at DEBUG instead
// of INFO, so routine liveness/readiness polling doesn't dominate traces or
// INFO-level logs.
func Instrument(service string, h http.Handler) http.Handler {
	log := NewLogger(service)
	return otelhttp.NewHandler(
		requestLog(log, h),
		service,
		otelhttp.WithFilter(func(r *http.Request) bool {
			return r.URL.Path != "/healthz" && r.URL.Path != "/readyz"
		}),
	)
}

func requestLog(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := clock.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		duration := clock.Now().Sub(start)

		level := slog.LevelInfo
		if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			level = slog.LevelDebug
		}
		log.LogAttrs(r.Context(), level, "http request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", sw.status),
			slog.Int64("duration_ms", duration.Milliseconds()),
		)
	})
}

// statusWriter captures the status code written so it can be logged —
// http.ResponseWriter itself has no getter for it.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
