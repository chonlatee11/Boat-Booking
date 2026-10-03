---
status: diagnosed
trigger: "UAT test 9 (G-02-9): route duplicate not rejected — 2 rows 'Proof Pier -> Proof Pier' (60 min, active) shown, no AlreadyExists error"
created: 2026-09-28T00:00:00Z
updated: 2026-09-28T14:40:00Z
---

## Current Focus

reasoning_checkpoint:
  hypothesis: "The two 'duplicate' rows are legitimately distinct one-way routes (Pier A->Pier B and Pier B->Pier A) between two DIFFERENT piers that happen to both be literally named 'Proof Pier' (seed pollution from repeated deploy/proof.sh runs). The backend unique constraint correctly does not fire (different ordered pairs). The visual 'duplicate' the user saw is caused by apps/admin's pierName()/route-label rendering using only nameTh with no disambiguator when two piers share a name, combined with the D-11 return-route button making it trivial to create the reverse pair while believing you're re-testing 'the same pair'."
  confirming_evidence:
    - "SELECT on piers: exactly 2 rows named 'Proof Pier', different ids, different operator_id (Proof Operator 1790451038 / Proof Operator 1790451769), created 2026-09-26 19:30:38 and 19:42:49 (~12min apart, two separate deploy/proof.sh runs)"
    - "SELECT on routes: the 2 active rows have pier_from_id/pier_to_id REVERSED (row1: A->B, operator=A's operator; row2: B->A, operator=B's operator) -- not the same ordered pair, so routes_active_pair_uq (UNIQUE btree (pier_from_id,pier_to_id) WHERE archived_at IS NULL) correctly does not fire"
    - "catalog container logs show exactly 2 UpsertRoute calls in the session (14:14:49, 14:15:04 UTC), both status 200 -- the literal same-pair-twice case was never actually submitted in this session"
    - "routes table has CHECK routes_check (pier_from_id <> pier_to_id) -- self-route is already impossible at the DB level, ruling out that hypothesis"
    - "apps/admin/.../routes/queries.ts pierName() returns found.nameTh with no operator/id disambiguation when a pier is found by id in the list (id-slice fallback only applies when NOT found at all)"
    - "apps/admin/.../routes/page.tsx line 85 builds the list label as `${name(pierFromId)} -> ${name(pierToId)}` -- two distinct pier ids sharing nameTh='Proof Pier' render as identical text"
    - "route-sheet.tsx's returnOf handler (D-11 'สร้างเส้นทางย้อนกลับ') swaps pierFromId/pierToId and is reachable directly from the routes list row actions -- explains the 15s gap between the two UpsertRoute calls and how a reverse pair was created without the user changing any visibly-different dropdown value (since both piers display as 'Proof Pier')"
    - "deploy/proof.sh line 54 hardcodes nameTh/nameEn='Proof Pier' with NO run-unique suffix (unlike line 39's operator name, which does get a timestamp suffix) -- every proof run leaves another identically-named pier behind"
  falsification_test: "If the two route rows' pier_from_id/pier_to_id had been identical (not reversed), or if only one 'Proof Pier' pier row existed in the DB, this hypothesis would be wrong. Both were checked directly via SELECT and refute the alternative."
  fix_rationale: "N/A (find_root_cause_only mode) -- fix would target apps/admin route/pier label rendering (disambiguate by operator name or id suffix on collision) and optionally deploy/proof.sh (unique-suffix the pier name like the operator name already gets), not the backend, which is already correct and covered by a passing automated test (02-06 D1 / UAT test 48)."
  blind_spots: "Did not inspect browser-side devtools network trace from the user's actual UAT session (not available, read-only investigation) -- inferred the return-route-button path from the 15s timing gap + reversed pier ids + code reachability, not from a captured client request. Did not check apps/web (customer-facing) since UAT test 9 is admin-only."
  candidate_causes:
    - "data: deploy/proof.sh (line 54) creates a non-uniquely-named 'Proof Pier' fixture on every run, unlike its own operator-name pattern one line above -- leftover dev seed collision"
    - "code: apps/admin routes page + queries.ts pierName()/label rendering has no disambiguation for two piers sharing nameTh -- a UI defect independent of the data collision"
  and_gate: "yes -- reproducing the exact UAT symptom (list shows what LOOKS like a duplicate, no AlreadyExists ever shown) requires BOTH conditions together: without the seed-data name collision, the two reversed-pair rows would show clearly different pier names and never look like a duplicate; without the UI's missing disambiguation, even colliding names would not visually mislead if the label included operator/id. Backend logic itself (unique constraint, check constraint, D-30 operator derivation) is correct and not a contributing cause."

next_action: none -- root cause confirmed with direct evidence, returning ROOT CAUSE FOUND (goal: find_root_cause_only).

## Symptoms

expected: Creating a route for a (pier_from, pier_to) pair that already has an active route is rejected with "มีเส้นทางนี้อยู่แล้ว"; the list distinguishes same-named piers.
actual: UAT test 9 (user, manual): "ไม่ผ่าน มันสร้างซ้ำได้" — admin routes list shows 2 rows "Proof Pier -> Proof Pier" (60 min, active), no duplicate error shown.
errors: none captured (no error toast observed by user — implies either request succeeded, or user never actually triggered the exact duplicate pair)
reproduction: Admin routes page as pier_admin, create route pier_from -> pier_to for a pair that (per user) already has an active route; observe no rejection, and 2 rows appear in list both labeled "Proof Pier -> Proof Pier".
started: found during 02-UAT.md test 9 (2026-09-28), phase 02-identity-catalog

## Eliminated

- hypothesis: "pier_from_id == pier_to_id is allowed (self-route bypasses uniqueness check)"
  evidence: "routes table has CHECK routes_check (pier_from_id <> pier_to_id); domain/app also has explicit pier_from<>pier_to validation on update (route.go line 103-105). Confirmed no self-route rows exist in the routes table."
  timestamp: 2026-09-28

- hypothesis: "The unique constraint / partial index on routes does not actually cover the reported pair (backend bug in migration or UpsertRoute)"
  evidence: "routes_active_pair_uq UNIQUE btree (pier_from_id, pier_to_id) WHERE archived_at IS NULL exists and is correctly ordered-pair scoped (migrations/00005_routes.sql); createRoute() maps 23505 -> domain.ErrAlreadyExists -> CodeAlreadyExists (route.go, scope.go toConnectErr). The 2 rows are NOT the same ordered pair (see Evidence) so no violation was expected."
  timestamp: 2026-09-28

## Evidence

- timestamp: 2026-09-28T14:30:00Z
  checked: "SELECT id, operator_id, name_th, name_en, created_at FROM piers WHERE name ILIKE 'Proof Pier'"
  found: "2 distinct pier rows, name_th=name_en='Proof Pier' for both, different ids, different operator_id (Proof Operator 1790451038 / Proof Operator 1790451769), created 2026-09-26 19:30:38 and 19:42:49 UTC"
  implication: "Not the same pier -- two separate dev-fixture piers happen to share an identical display name"

- timestamp: 2026-09-28T14:31:00Z
  checked: "SELECT id, operator_id, pier_from_id, pier_to_id, duration_minutes, archived_at, created_at FROM routes ORDER BY created_at"
  found: "The 2 non-archived rows referenced in UAT test 9 have REVERSED pier_from_id/pier_to_id (row1: PierA->PierB, operator=PierA's operator; row2: PierB->PierA, operator=PierB's operator), created 2026-09-28 14:14:49 and 14:15:04 (15s apart). 2 other rows exist but are archived (different pier pair, unrelated)."
  implication: "The 2 rows are legitimately different one-way routes (A->B and its reverse B->A), not a duplicate of the same ordered pair -- routes_active_pair_uq correctly does not reject this"

- timestamp: 2026-09-28T14:32:00Z
  checked: "\\d piers, \\d routes (schema/constraints) in catalog DB"
  found: "routes_active_pair_uq UNIQUE btree (pier_from_id, pier_to_id) WHERE archived_at IS NULL; routes_check CHECK (pier_from_id <> pier_to_id); both FKs to piers present"
  implication: "Schema matches the documented design (D-11/D-30/CAT-03) exactly -- no self-route possible, uniqueness correctly scoped per ordered pair"

- timestamp: 2026-09-28T14:33:00Z
  checked: "docker logs boatbooking-catalog-1 filtered to the UAT session window (14:10-14:20 UTC 2026-09-28)"
  found: "Exactly 2 UpsertRoute POSTs in the window (14:14:49, 14:15:04), both status 200 -- no AlreadyExists / non-200 response for any UpsertRoute call in this session"
  implication: "The literal 'submit the exact same (pier_from,pier_to) pair a second time' case was never actually exercised in this manual session -- both submitted pairs differ (reversed), so the AlreadyExists path was correctly never triggered, matching the DB row data"

- timestamp: 2026-09-28T14:34:00Z
  checked: "services/catalog/internal/app/route.go UpsertRoute/createRoute/updateRoute"
  found: "OperatorID always derived from pierFrom's stored operator (D-30, line 78); isUniqueViolation(err) on pgcode 23505 maps to domain.ErrAlreadyExists (line 129-131); update path explicitly rejects PierFromID==PierToID (line 103-105) in addition to the DB CHECK"
  implication: "Backend UpsertRoute implementation matches the documented and tested behavior; confirmed correct, not the defect"

- timestamp: 2026-09-28T14:35:00Z
  checked: "apps/admin/src/app/(admin)/routes/queries.ts pierName() and page.tsx route list label (line 85) and route-sheet.tsx (create/edit Sheet + returnOf swap handler)"
  found: "pierName(id, ...) returns found.nameTh with NO disambiguation (operator name / id) whenever the pier IS found in a list -- the id.slice(0,8) fallback only fires when the pier is absent from every list. page.tsx builds every list/archive-dialog label as `${name(pierFromId)} -> ${name(pierToId)}`. route-sheet.tsx's returnOf branch (D-11 'สร้างเส้นทางย้อนกลับ' button, reachable per-row from the routes list) sets pierFromId=source.pierToId and pierToId=source.pierFromId and is one click + one save away from creating the exact reverse route."
  implication: "Given 2 piers named identically 'Proof Pier', any route and its D-11 reverse-route counterpart render as textually IDENTICAL rows ('Proof Pier -> Proof Pier' twice) in both the routes list and the pier-select dropdowns -- a human tester cannot visually distinguish a true duplicate attempt from the legitimate reverse-route feature, nor tell the two piers apart to pick 'the same' pier_to twice on purpose. This is the proximate UI defect."

- timestamp: 2026-09-28T14:36:00Z
  checked: "grep 'Proof Pier' across .go/.sh/Makefile (deploy/proof.sh)"
  found: "deploy/proof.sh line 39 generates a run-unique operator name (\"Proof Operator %s\" with a timestamp $SUFFIX) but line 54 hardcodes the pier's nameTh/nameEn as the literal constant 'Proof Pier' with no equivalent uniquifying suffix"
  implication: "Every make proof / make kong-roundtrip run leaves one more identically-named 'Proof Pier' fixture row in the dev catalog DB permanently (proof.sh does not clean up) -- root cause of the seed-data collision that made the UI defect above observable in this session"

## Resolution

root_cause: "Two contributing causes, both required (AND-gate) to reproduce the exact UAT symptom: (1) apps/admin's route/pier label rendering (apps/admin/src/app/(admin)/routes/queries.ts pierName(), used by page.tsx and route-sheet.tsx) shows only a pier's name_th with no disambiguation when two piers share a name -- so a route and its legitimate D-11 reverse-route counterpart render as visually identical rows; (2) deploy/proof.sh creates a new pier hardcoded to the literal name 'Proof Pier' on every run (unlike its own operator name, which is timestamp-suffixed), so repeated make proof/make kong-roundtrip runs during Phase 1/2 left two distinct, identically-named 'Proof Pier' fixture piers (under two different operators) sitting in the dev catalog DB. The backend itself (UpsertRoute, routes_active_pair_uq unique index, routes_check CHECK constraint, D-30 operator derivation) is correct and already covered by a passing automated test (02-06 D1 / UAT test 48) -- it was never actually asked to reject a true duplicate pair in this manual session; the two UpsertRoute calls that occurred (14:14:49, 14:15:04) submitted REVERSED pier_from/pier_to pairs (A->B then B->A, most likely via the 'สร้างเส้นทางย้อนกลับ' return-route button), which is by-design a different, allowed row -- but because both endpoint piers display identically as 'Proof Pier', the result was indistinguishable from an actual duplicate to the human tester, who could not construct or recognize a true same-pair duplicate test at all."
fix: (not applied -- find_root_cause_only mode)
verification: (not applicable -- no fix applied)
files_changed: []
