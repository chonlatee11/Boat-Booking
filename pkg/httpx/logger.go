package httpx

import (
	"context"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/trace"
)

// NewLogger builds service's slog.Logger (D-52, D-53): stdout JSON (or text
// when LOG_FORMAT=text) carrying trace_id/span_id whenever the logging call
// site's ctx holds an active span, fanned out to the OTel logs bridge
// (otelslog), which gets its trace/span ids natively from ctx via the OTel
// SDK itself. Every line also carries base attrs service and env.
func NewLogger(service string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(EnvOr("LOG_LEVEL", "info"))}

	var stdout slog.Handler
	if EnvOr("LOG_FORMAT", "json") == "text" {
		stdout = slog.NewTextHandler(os.Stdout, opts)
	} else {
		stdout = slog.NewJSONHandler(os.Stdout, opts)
	}

	handler := &fanoutHandler{handlers: []slog.Handler{
		&traceHandler{next: stdout},
		otelslog.NewHandler(service),
	}}

	return slog.New(handler).With("service", service, "env", EnvOr("ENV", "dev"))
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// traceHandler wraps another handler, adding trace_id/span_id attributes
// from ctx's active span (D-52) when one is present.
type traceHandler struct {
	next slog.Handler
}

func (h *traceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r = r.Clone()
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.next.Handle(ctx, r)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{next: h.next.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{next: h.next.WithGroup(name)}
}

// fanoutHandler sends every record to all handlers. Hand-rolled rather than
// imported — stdlib has no multi-handler, and 2 known sinks with 4 trivial
// methods each is below the threshold where a dependency is justified
// (RESEARCH "Don't Hand-Roll" table).
type fanoutHandler struct{ handlers []slog.Handler }

func (f *fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (f *fanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range f.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &fanoutHandler{handlers: next}
}

func (f *fanoutHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithGroup(name)
	}
	return &fanoutHandler{handlers: next}
}
