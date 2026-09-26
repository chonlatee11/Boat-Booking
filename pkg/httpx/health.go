package httpx

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/chonlatee11/boat-booking/pkg/clock"
)

const (
	readinessCacheTTL = time.Second
	readinessTimeout  = 2 * time.Second
)

// Readiness aggregates named dependency checks behind /readyz (D-39):
// results are cached for readinessCacheTTL, each poll of all checks is
// bounded by readinessTimeout, and once SetShuttingDown is called every
// request gets 503 regardless of cache state (D-40).
type Readiness struct {
	checks map[string]func(context.Context) error

	mu           sync.Mutex
	lastResults  map[string]error
	lastAt       time.Time
	shuttingDown bool
}

// NewReadiness builds a Readiness polling checks (name -> check function).
func NewReadiness(checks map[string]func(context.Context) error) *Readiness {
	return &Readiness{checks: checks}
}

// SetShuttingDown makes every subsequent Handler call return 503
// immediately, regardless of cache freshness (D-40).
func (r *Readiness) SetShuttingDown() {
	r.mu.Lock()
	r.shuttingDown = true
	r.mu.Unlock()
}

// Handler serves /readyz.
func (r *Readiness) Handler(w http.ResponseWriter, req *http.Request) {
	r.mu.Lock()
	shuttingDown := r.shuttingDown
	fresh := !r.lastAt.IsZero() && clock.Now().Sub(r.lastAt) < readinessCacheTTL
	var cached map[string]error
	if fresh {
		cached = r.lastResults
	}
	r.mu.Unlock()

	if shuttingDown {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "shutting_down"})
		return
	}
	if fresh {
		writeReadinessResult(w, cached)
		return
	}

	results := r.runChecks(req.Context())

	r.mu.Lock()
	r.lastResults = results
	r.lastAt = clock.Now()
	r.mu.Unlock()

	writeReadinessResult(w, results)
}

func (r *Readiness) runChecks(ctx context.Context) map[string]error {
	results := make(map[string]error, len(r.checks))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for name, check := range r.checks {
		wg.Add(1)
		go func(name string, check func(context.Context) error) {
			defer wg.Done()
			checkCtx, cancel := context.WithTimeout(ctx, readinessTimeout)
			defer cancel()
			err := check(checkCtx)
			mu.Lock()
			results[name] = err
			mu.Unlock()
		}(name, check)
	}
	wg.Wait()
	return results
}

func writeReadinessResult(w http.ResponseWriter, results map[string]error) {
	body := make(map[string]string, len(results))
	ok := true
	for name, err := range results {
		if err != nil {
			ok = false
			body[name] = "error: " + err.Error()
		} else {
			body[name] = "ok"
		}
	}
	status := http.StatusOK
	if !ok {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, body)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// Healthz always returns 200 while the process is alive (D-39).
func Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
