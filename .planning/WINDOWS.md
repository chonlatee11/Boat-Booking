---
schema_version: 1
open_count: 1
waived_count: 0
fixed_count: 0
total_count: 1
last_updated: 2026-09-26T18:48:07.301Z
---

# Broken Windows Ledger

> Cross-phase defect register. With `workflow.windows_enforce` enabled, `/gsd-ship` blocks while `open_count > 0`.
> Waive with `gsd-tools windows waive <id> "<reason>"` (reason required).
> Mark fixed with `gsd-tools windows fixed <id>`.

| id | phase | kind | file | line | description | status | reason | recorded_at | resolved_at |
|----|-------|------|------|------|-------------|--------|--------|-------------|-------------|
| 1 | 02 | lint-warning | services/gateway/internal/adapters/http/auth_test.go |  | gosec G124: missing Secure/HttpOnly/SameSite on test-only http.Cookie literals (pre-existing from 02-05, out of scope for 02-06) | open |  | 2026-09-26T18:48:07.301Z |  |

````json
[
  {
    "id": 1,
    "kind": "lint-warning",
    "phase": "02",
    "file": "services/gateway/internal/adapters/http/auth_test.go",
    "line": null,
    "description": "gosec G124: missing Secure/HttpOnly/SameSite on test-only http.Cookie literals (pre-existing from 02-05, out of scope for 02-06)",
    "status": "open",
    "reason": "",
    "recorded_at": "2026-09-26T18:48:07.301Z",
    "resolved_at": null,
    "milestone": null
  }
]
````
