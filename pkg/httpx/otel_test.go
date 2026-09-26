package httpx

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestAllowlistExporterDropsNonAllowlistedAttributes(t *testing.T) {
	inMemory := tracetest.NewInMemoryExporter()
	exp := &allowlistExporter{next: inMemory}
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))

	_, span := tp.Tracer("test").Start(context.Background(), "test-span")
	span.SetAttributes(
		attribute.String("client.address", "1.2.3.4"),
		attribute.String("user_agent.original", "curl/8.0"),
		attribute.String("url.query", "a=b"),
		attribute.String("http.route", "/v1/pings"),
	)
	span.End()

	// WithSyncer exports synchronously in End(), so spans are already
	// visible here. Read them before Shutdown, which clears the recorder.
	spans := inMemory.GetSpans()
	if err := tp.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}

	attrs := spans[0].Attributes
	if len(attrs) != 1 {
		t.Fatalf("got %d attributes, want 1: %v", len(attrs), attrs)
	}
	if string(attrs[0].Key) != "http.route" {
		t.Fatalf("surviving attribute = %q, want %q", attrs[0].Key, "http.route")
	}
	if attrs[0].Value.AsString() != "/v1/pings" {
		t.Fatalf("http.route value = %q, want %q", attrs[0].Value.AsString(), "/v1/pings")
	}
}
