---
status: diagnosed
trigger: "goal: find_root_cause_only / UAT gap G-02-7 (test 7) + related note in test 8 -- admin piers Sheet map picker: typing lat/lng crashes the page with a maplibre-gl 'Worker failed to load' console error and the marker does not move; separately (test 8) 'โหลดแผนที่ไม่สำเร็จ' shows even though tiles partially render"
created: "2026-09-28T14:42:10.000Z"
updated: "2026-09-28T14:42:10.000Z"
---

## Current Focus

hypothesis: CONFIRMED (two independent causes, both in apps/admin/src/components/map-picker.tsx)
test: static trace of handleLatChange/handleLngChange -> marker-sync useEffect -> maplibre-gl LngLat constructor, cross-referenced against maplibre-gl source in node_modules; static trace of map.on('error', ...) handler
expecting: n/a (root cause confirmed via source-level evidence; goal is find_root_cause_only, no fix applied)
next_action: none -- return ROOT CAUSE FOUND to caller

## Symptoms

expected: |
  admin หน้า piers ในฐานะ super_admin: สร้าง pier คลิกบนแผนที่แล้ว marker ย้ายตาม, ลาก marker แล้วช่อง lat/lng
  อัปเดตตาม, พิมพ์ lat/lng แล้ว marker ย้ายตาม โดยหน้าไม่ crash (map worker โหลดได้) (test 7 / G-02-7).
  Separately (test 8): block tile requests -> เปิด Sheet ใหม่ -> ควรขึ้น "โหลดแผนที่ไม่สำเร็จ" เฉพาะตอน tile จริงๆโหลดไม่ได้
actual: |
  Click/drag marker on map works. While typing latitude, page crashes into Next.js dev overlay:
  "Console Error: Worker failed to load. Check that the worker URL is correct." (2 issues); background
  shows an error page. Typing lat/lng does not move the marker as expected.
  Separately (test 8): Sheet shows "โหลดแผนที่ไม่สำเร็จ" even though tiles partially render.
errors: |
  Next.js dev overlay Console Error: "Worker failed to load. Check that the worker URL is correct."
  (maplibre-gl Dispatcher's mislabeled wrapper around ANY underlying Worker 'error' DOM event --
  maplibre-gl-dev.mjs:2346-2348)
reproduction: |
  apps/admin (:3002) as super_admin -> Piers -> open/create pier Sheet -> type digits into the
  latitude input one keystroke at a time (e.g. building "13.736717" without typing the decimal point
  first, or clearing the field) -> page crashes / marker does not track input as expected.
started: "Introduced in 02-11 (D1) map picker implementation; not previously covered by an automated
  numeric-input-under-maplibre-gl test"

## Eliminated

- hypothesis: "Map is re-created on every keystroke (mount useEffect deps include value, spawning new workers)"
  evidence: |
    map-picker.tsx:39-82 -- the map-creation useEffect has an empty dependency array `[]` (line 82,
    with an explicit eslint-disable-next-line react-hooks/exhaustive-deps and a comment confirming
    "Intentionally mount-only"). The separate marker-sync effect (lines 87-105) depends only on
    `[value?.lat, value?.lng]` and never calls `new MapLibreMap(...)` or `map.setCenter()` -- it only
    calls `marker.setLngLat(...)` or creates a `Marker` (not a `Map`). Typing therefore cannot recreate
    the Map/worker pool through this path.
  timestamp: "2026-09-28T14:42:10.000Z"
- hypothesis: "maplibre-gl worker URL is broken/misconfigured under Turbopack (bundler/CSP issue)"
  evidence: |
    User-confirmed follow-up note in the UAT gap itself: "ภายหลังผู้ใช้ยืนยันว่า ลาก marker บนแผนที่ได้ --
    ที่พังคือพิมพ์ lat/lng" (dragging the marker on the map works fine -- only typing breaks it). If the
    worker URL/bundling were actually broken, the very first vector-tile decode (needed immediately for
    the liberty/OpenFreeMap vector style) would fail on load, before any typing -- but click/drag and
    initial tile rendering already work. Confirms the worker genuinely loads; the crash is triggered by
    application-level input handling, not by Turbopack's worker bundling.
  timestamp: "2026-09-28T14:42:10.000Z"

## Evidence

- timestamp: "2026-09-28T14:42:10.000Z"
  checked: apps/admin/src/components/map-picker.tsx:107-117 (handleLatChange/handleLngChange)
  found: |
    ```
    function handleLatChange(raw: string) {
      const lat = Number(raw);
      if (!Number.isFinite(lat)) return;
      onChange({ lat, lng: value?.lng ?? THAILAND_CENTER[0] });
    }
    ```
    Guard only rejects non-finite (NaN/Infinity) results. It does NOT: (a) distinguish an
    HTML5-invalid intermediate number-input string (e.g. "13.", which the DOM reports as `""` per the
    <input type=number> spec) from a genuinely-empty field -- `Number('')` is `0`, which IS finite, so
    it silently passes through as a valid value change to lat=0; (b) clamp/validate the numeric range
    at all -- any finite value, including e.g. 137 (which a user can type transiently while entering
    "13.736717" digit-by-digit before typing the "."), is passed straight to `onChange`.
  implication: |
    Two distinct defects: (1) empty/partial input silently resolves to 0 instead of being ignored,
    making the marker appear to jump/"not move as expected" while typing; (2) out-of-range values are
    never rejected before being handed to maplibre-gl.
- timestamp: "2026-09-28T14:42:10.000Z"
  checked: apps/admin/src/components/map-picker.tsx:87-105 (marker-sync useEffect) and :50-63 (placeMarker)
  found: |
    `markerRef.current.setLngLat([value.lng, value.lat])` (and `new Marker(...).setLngLat([lng, lat])`)
    is called directly with whatever `value` the parent passed down -- no try/catch, no range check.
    This effect fires synchronously whenever `value?.lat`/`value?.lng` change, i.e. on every keystroke
    that passes the (weak) guard in handleLatChange/handleLngChange.
  implication: |
    Any out-of-range or NaN lat/lng reaching this effect throws synchronously inside a React
    useEffect (commit phase), uncaught locally.
- timestamp: "2026-09-28T14:42:10.000Z"
  checked: apps/admin/node_modules/maplibre-gl/dist/maplibre-gl-shared-dev.mjs:19378-19388 (LngLat class)
  found: |
    ```
    constructor(lng, lat) {
      if (isNaN(lng) || isNaN(lat)) throw new Error(`Invalid LngLat object: (${lng}, ${lat})`);
      this.lng = +lng;
      this.lat = +lat;
      if (this.lat > 90 || this.lat < -90) throw new Error("Invalid LngLat latitude value: must be between -90 and 90");
    }
    ```
    `Marker.setLngLat()` calls `LngLat.convert(lnglat)` -> `new LngLat(...)`, so this constructor is
    exactly what's invoked from the marker-sync effect.
  implication: |
    Confirms a real, synchronous, uncaught-able-by-app JS exception is thrown by maplibre-gl itself
    whenever the app hands it |lat| > 90 or NaN -- this is not a hypothetical, it's the documented
    behavior of the exact library/version in node_modules (maplibre-gl 6.11.2).
- timestamp: "2026-09-28T14:42:10.000Z"
  checked: apps/admin/src/app/ -- searched for error.tsx / global-error.tsx
  found: "No error.tsx or global-error.tsx exists anywhere under apps/admin/src/app."
  implication: |
    There is no local/custom React error boundary to catch the MapPicker effect's throw. It propagates
    to Next.js's built-in dev error overlay, which replaces the whole route segment -- exactly matching
    the reported "Console Error" overlay with "หน้าเบื้องหลังเป็น error page" (background is an error
    page) rather than a contained/graceful failure.
- timestamp: "2026-09-28T14:42:10.000Z"
  checked: apps/admin/node_modules/maplibre-gl/dist/maplibre-gl-dev.mjs:2331-2354 (Dispatcher class) and :14901 (Style constructing its Dispatcher)
  found: |
    `Dispatcher.initActors` subscribes to the underlying `Worker`'s native `"error"` DOM event and,
    on ANY such event (not specifically a load/URL-resolution failure), fires:
    `this.fire(new ErrorEvent(new Error("Worker failed to load. Check that the worker URL is correct.")))`.
    The Dispatcher's evented-parent chain is `Dispatcher -> Style -> Map` (`new Dispatcher(...).setEventedParent(this)`
    inside the Style constructor, dev.mjs:14901), so this ErrorEvent bubbles up and is delivered to
    whatever listens to `map.on('error', ...)`.
  implication: |
    The exact string the user saw ("Worker failed to load. Check that the worker URL is correct.") is
    maplibre-gl's generic, mislabeled wrapper for ANY worker-thread runtime error -- it fires whenever a
    live worker throws/errors for ANY reason while processing a message, not only on initial
    script-load/URL failure. It is delivered through the Map's own 'error' event, i.e. through the same
    channel as tile-load errors.
- timestamp: "2026-09-28T14:42:10.000Z"
  checked: apps/admin/src/components/map-picker.tsx:73 (`map.on('error', () => setTilesFailed(true))`)
  found: |
    The handler treats every single 'error' event fired on the Map -- regardless of source (a
    transient/individual tile fetch failure, a style error, or a worker-thread error bubbled up via
    Dispatcher->Style->Map as described above) -- as a fatal "map failed to load" condition, and never
    resets `tilesFailed` back to `false` on subsequent success.
  implication: |
    This is the confirmed, independent root cause of test 8's "ขึ้น โหลดแผนที่ไม่สำเร็จ ทั้งที่ tile
    แสดงบางส่วน" (shows "map failed to load" even though tiles partially render) -- a single failed
    tile request (or the worker error described above) is enough to permanently flip `tilesFailed` to
    true for the rest of that Sheet's lifetime, even while most tiles are rendering successfully.

## Resolution

root_cause: |
  Two independent bugs in apps/admin/src/components/map-picker.tsx, both stemming from missing
  input/error discrimination around the maplibre-gl API surface (single category: application code --
  no config/environment/data cause found, so the AND-gate does not apply; these are two separate root
  causes for two related symptoms bundled in one UAT gap, not two co-conditions of one failure):

  1. (Crash + "marker doesn't move" while typing, test 7 / G-02-7): `handleLatChange`/`handleLngChange`
     (lines 107-117) parse the raw <input type="number"> string with plain `Number(raw)` and only guard
     via `Number.isFinite(lat)`. This guard does not reject (a) the empty string that the DOM reports
     for an HTML5-invalid intermediate number value such as "13." (`Number('')` is `0`, which IS
     finite -- so the marker silently jumps to lat/lng 0 mid-typing instead of being ignored, which is
     what makes typing "look broken"/marker not tracking input as expected), or (b) any in-range-looking
     but geographically invalid latitude a user can transiently produce while typing digit-by-digit
     before the decimal point (e.g. "137" while entering "13.736717"). That out-of-range/NaN value flows
     straight into the marker-sync `useEffect` (lines 87-105) / mount effect's `placeMarker` (lines
     50-63), which call `Marker.setLngLat(...)` with no validation and no try/catch. maplibre-gl's
     `LngLat` constructor (node_modules/maplibre-gl/dist/maplibre-gl-shared-dev.mjs:19383-19388)
     synchronously throws `Error("Invalid LngLat latitude value: must be between -90 and 90")` (or the
     NaN variant) in that case. Because the throw happens inside a React effect during commit, and
     apps/admin has no error.tsx/global-error.tsx anywhere, React's default dev error boundary takes
     over and replaces the whole page -- this is the "Console Error" overlay + "background is an error
     page" the user saw. The specific displayed message text ("Worker failed to load. Check that the
     worker URL is correct.") is maplibre-gl's own generic, mislabeled wrapper for ANY underlying Worker
     runtime `error` event (Dispatcher, maplibre-gl-dev.mjs:2346-2348), delivered through the same
     `map.on('error', ...)` channel (Dispatcher -> Style -> Map, dev.mjs:14901) -- most likely surfaced
     as the crash's secondary/paired console message during the mount-effect's `map.remove()` cleanup
     that runs when React unmounts the failed subtree, racing the in-flight worker pool teardown; it is
     not an actual Turbopack/bundler worker-loading failure (ruled out separately: click/drag and
     initial tile rendering already work in the same session, so the worker legitimately loaded).

  2. (Independent -- "โหลดแผนที่ไม่สำเร็จ" shown despite partial tiles, test 8): `map.on('error', () =>
     setTilesFailed(true))` (map-picker.tsx:73) treats every Map 'error' event as a permanent, fatal
     "map failed" condition with no discrimination by event source/type and no reset path back to
     false -- so a single transient tile failure (or the mislabeled worker error from bug #1) latches
     the Thai "โหลดแผนที่ไม่สำเร็จ" message on for the rest of the Sheet's life even while most tiles
     render fine.
fix: (not applied -- goal: find_root_cause_only)
verification: (not applicable -- diagnosis only)
files_changed: []
