## Deferred Items

- `make lint` fails on 3 pre-existing gosec G124 findings in
  `services/gateway/internal/adapters/http/{auth_test,proxy_test}.go`
  (missing Secure/HttpOnly/SameSite on test-only `http.Cookie` literals).
  status: open
  **What:** Introduced in plan 02-05 (`aa5128a`), unrelated to 02-06's
  catalog-only scope (routes/prices/archive rules). `services/catalog`'s own
  `golangci-lint run ./...` is clean (0 issues); this only surfaces when
  running the whole-workspace `make lint` target.
  **Found during:** 02-06 Task 3 plan-level `<verification>` (`make lint`).
