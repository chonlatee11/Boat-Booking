package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chonlatee11/boat-booking/pkg/clock"
)

func TestHealthzAlwaysOK(t *testing.T) {
	rec := httptest.NewRecorder()
	Healthz(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestReadinessHandlerOK(t *testing.T) {
	r := NewReadiness(map[string]func(context.Context) error{
		"db": func(context.Context) error { return nil },
	})

	rec := httptest.NewRecorder()
	r.Handler(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["db"] != "ok" {
		t.Fatalf("db = %q, want ok", body["db"])
	}
}

func TestReadinessHandlerFailingCheck(t *testing.T) {
	r := NewReadiness(map[string]func(context.Context) error{
		"kafka": func(context.Context) error { return errors.New("broker unreachable") },
	})

	rec := httptest.NewRecorder()
	r.Handler(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["kafka"] != "error: broker unreachable" {
		t.Fatalf("kafka = %q, want error message", body["kafka"])
	}
}

func TestReadinessHandlerCachesResultForOneSecond(t *testing.T) {
	var calls int32
	r := NewReadiness(map[string]func(context.Context) error{
		"db": func(context.Context) error {
			atomic.AddInt32(&calls, 1)
			return nil
		},
	})

	rec1 := httptest.NewRecorder()
	r.Handler(rec1, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	rec2 := httptest.NewRecorder()
	r.Handler(rec2, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("check called %d times for two requests within the cache TTL, want 1", got)
	}

	// Advance the clock past the cache TTL and confirm the check runs again.
	original := clock.Now
	clock.Now = func() time.Time { return original().Add(2 * time.Second) }
	defer func() { clock.Now = original }()

	rec3 := httptest.NewRecorder()
	r.Handler(rec3, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("check called %d times after cache TTL elapsed, want 2", got)
	}
}

func TestReadinessHandlerShuttingDown(t *testing.T) {
	r := NewReadiness(map[string]func(context.Context) error{
		"db": func(context.Context) error { return nil },
	})
	r.SetShuttingDown()

	rec := httptest.NewRecorder()
	r.Handler(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "shutting_down" {
		t.Fatalf("status field = %q, want shutting_down", body["status"])
	}
}
