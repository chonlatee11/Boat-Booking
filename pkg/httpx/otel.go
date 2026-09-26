package httpx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	logglobal "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// SetupOTel always installs the global TraceContext+Baggage propagator, so
// trace context keeps flowing across HTTP and Kafka hops regardless of
// whether telemetry is exported anywhere. When OTEL_EXPORTER_OTLP_ENDPOINT
// is set, it also wires trace/metric/log providers exporting to it over
// OTLP/HTTP (D-45, D-50, D-52, D-53); otherwise it returns a no-op shutdown.
func SetupOTel(ctx context.Context, service string) (shutdown func(context.Context) error, err error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.New(ctx, resource.WithAttributes(
		attribute.String("service.name", service),
		attribute.String("deployment.environment", EnvOr("ENV", "dev")),
	))
	if err != nil {
		return nil, fmt.Errorf("httpx: build otel resource: %w", err)
	}

	traceExp, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("httpx: new trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
		sdktrace.WithBatcher(&allowlistExporter{next: traceExp}),
	)
	otel.SetTracerProvider(tp)

	metricExp, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("httpx: new metric exporter: %w", err)
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(15*time.Second))),
	)
	otel.SetMeterProvider(mp)

	logExp, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("httpx: new log exporter: %w", err)
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
	)
	logglobal.SetLoggerProvider(lp)

	return func(shutdownCtx context.Context) error {
		return errors.Join(
			tp.Shutdown(shutdownCtx),
			mp.Shutdown(shutdownCtx),
			lp.Shutdown(shutdownCtx),
		)
	}, nil
}

// spanAttrAllowlist are the exact keys or key prefixes exported span
// attributes may carry (D-45) — everything else (client IPs, user agents,
// query strings, and any other attribute a future instrumentation adds) is
// dropped before export.
//
// IN-02: this list has no compile-time link to what instrumentation
// libraries or manual span.SetAttributes calls actually emit. A new
// attribute added anywhere without a matching entry here is silently
// dropped (the safe failure direction for PII) rather than erroring — if a
// new span attribute isn't showing up in traces, check here first.
var spanAttrAllowlist = []string{
	"http.request.method",
	"http.response.status_code",
	"http.route",
	"url.path",
	"url.scheme",
	"server.",
	"network.protocol.",
	"rpc.",
	"messaging.",
	"error.type",
	"event_type",
	"event_id",
	"aggregate_id",
	"otel.",
}

func allowedSpanAttr(key string) bool {
	for _, prefix := range spanAttrAllowlist {
		if key == prefix || strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// allowlistExporter wraps a sdktrace.SpanExporter, dropping every span
// attribute not in spanAttrAllowlist before delegating the export (D-45).
type allowlistExporter struct {
	next sdktrace.SpanExporter
}

func (e *allowlistExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	filtered := make([]sdktrace.ReadOnlySpan, len(spans))
	for i, s := range spans {
		stub := tracetest.SpanStubFromReadOnlySpan(s)
		kept := make([]attribute.KeyValue, 0, len(stub.Attributes))
		for _, a := range stub.Attributes {
			if allowedSpanAttr(string(a.Key)) {
				kept = append(kept, a)
			}
		}
		stub.Attributes = kept
		filtered[i] = stub.Snapshot()
	}
	return e.next.ExportSpans(ctx, filtered)
}

func (e *allowlistExporter) Shutdown(ctx context.Context) error {
	return e.next.Shutdown(ctx)
}
