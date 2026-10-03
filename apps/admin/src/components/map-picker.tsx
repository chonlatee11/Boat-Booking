'use client';

import { useEffect, useRef, useState } from 'react';
import { Map as MapLibreMap, Marker } from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import { Field, FieldLabel, FieldError } from '@/components/ui/field';
import { Input } from '@/components/ui/input';

// D-20: OpenFreeMap by default, swappable via env — no API key, no
// geocoding. Thailand center/zoom used until a marker is placed.
const STYLE_URL =
  process.env.NEXT_PUBLIC_MAP_STYLE_URL ??
  'https://tiles.openfreemap.org/styles/liberty';
const THAILAND_CENTER: [number, number] = [100.5, 13.7];

export type LatLng = { lat: number; lng: number };

/**
 * Parses a coordinate draft string. Returns the number only when the
 * trimmed text is non-empty, finite, and within `limit` degrees — a
 * partial/empty/out-of-range draft (e.g. "", "13.", "137") returns null
 * instead of ever reaching maplibre-gl, whose LngLat constructor throws
 * synchronously outside that range (G-02-7).
 */
function parseCoord(text: string, limit: number): number | null {
  const trimmed = text.trim();
  if (!trimmed) return null;
  const n = Number(trimmed);
  if (!Number.isFinite(n) || Math.abs(n) > limit) return null;
  return n;
}

/** Click/drag marker picker with manual lat/lng fallback (D-20). */
export function MapPicker({
  value,
  onChange,
}: {
  value?: LatLng;
  onChange: (value: LatLng | undefined) => void;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MapLibreMap | null>(null);
  const markerRef = useRef<Marker | null>(null);
  const onChangeRef = useRef(onChange);
  const [tilesFailed, setTilesFailed] = useState(false);
  const [latText, setLatText] = useState(value ? String(value.lat) : '');
  const [lngText, setLngText] = useState(value ? String(value.lng) : '');
  // "Adjust state during render" pattern (no effect+setState loop, no
  // react-hooks/exhaustive-deps escape hatch needed): when `value` changes
  // identity because the map moved the marker (click/drag) or a different
  // pier's location loaded, re-sync the drafts unless they already read the
  // same number — which is exactly the case when this render's `value`
  // change originated from *this* input's own onChange, so typing never
  // gets clobbered mid-keystroke.
  const [prevValue, setPrevValue] = useState(value);
  if (value !== prevValue) {
    setPrevValue(value);
    if (value) {
      if (Number(latText) !== value.lat) setLatText(String(value.lat));
      if (Number(lngText) !== value.lng) setLngText(String(value.lng));
    }
  }

  useEffect(() => {
    onChangeRef.current = onChange;
  });

  // Create the map once on mount — client-only, MapLibre never runs during
  // SSR since this whole component is 'use client' and construction happens
  // inside an effect.
  useEffect(() => {
    if (!containerRef.current) return;

    const map = new MapLibreMap({
      container: containerRef.current,
      style: STYLE_URL,
      center: value ? [value.lng, value.lat] : THAILAND_CENTER,
      zoom: value ? 12 : 5,
    });
    mapRef.current = map;

    function placeMarker(lat: number, lng: number) {
      if (markerRef.current) {
        markerRef.current.setLngLat([lng, lat]);
        return;
      }
      const marker = new Marker({ draggable: true })
        .setLngLat([lng, lat])
        .addTo(map);
      marker.on('dragend', () => {
        const lngLat = marker.getLngLat();
        onChangeRef.current({ lat: lngLat.lat, lng: lngLat.lng });
      });
      markerRef.current = marker;
    }

    if (value) {
      placeMarker(value.lat, value.lng);
    }

    map.on('click', (e) => {
      placeMarker(e.lngLat.lat, e.lngLat.lng);
      onChangeRef.current({ lat: e.lngLat.lat, lng: e.lngLat.lng });
    });

    // UI-SPEC E3: "โหลดแผนที่ไม่สำเร็จ" must show only when the map errors
    // before any tile has loaded, and clear once a tile loads — a single
    // transient tile failure (or maplibre's mislabeled worker-error event,
    // see .planning/debug/pier-map-worker-crash.md) must never latch it
    // permanently once tiles are actually rendering (G-02-7/G-02-8).
    let tileLoaded = false;
    map.on('sourcedata', (e) => {
      if (e.tile) {
        tileLoaded = true;
        setTilesFailed(false);
      }
    });
    map.on('error', () => {
      if (!tileLoaded) setTilesFailed(true);
    });

    return () => {
      markerRef.current = null;
      map.remove();
    };
    // Intentionally mount-only: value is only used for the initial center
    // and marker; live updates are handled by the effect below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Keep the marker synced when the value changes externally (manual
  // lat/lng inputs) — the map itself doesn't re-center to avoid jumping
  // while typing.
  useEffect(() => {
    const map = mapRef.current;
    if (!map || !value) return;
    if (markerRef.current) {
      markerRef.current.setLngLat([value.lng, value.lat]);
      return;
    }
    const marker = new Marker({ draggable: true })
      .setLngLat([value.lng, value.lat])
      .addTo(map);
    marker.on('dragend', () => {
      const lngLat = marker.getLngLat();
      onChangeRef.current({ lat: lngLat.lat, lng: lngLat.lng });
    });
    markerRef.current = marker;
    // Depend on the primitive lat/lng, not the value object identity, so
    // this doesn't re-run on every parent re-render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value?.lat, value?.lng]);

  function handleLatTextChange(raw: string) {
    setLatText(raw);
    const lat = parseCoord(raw, 90);
    const lng = parseCoord(lngText, 180);
    onChange(lat !== null && lng !== null ? { lat, lng } : undefined);
  }

  function handleLngTextChange(raw: string) {
    setLngText(raw);
    const lat = parseCoord(latText, 90);
    const lng = parseCoord(raw, 180);
    onChange(lat !== null && lng !== null ? { lat, lng } : undefined);
  }

  const latInvalid = latText !== '' && parseCoord(latText, 90) === null;
  const lngInvalid = lngText !== '' && parseCoord(lngText, 180) === null;

  return (
    <div className="flex flex-col gap-2">
      <div
        ref={containerRef}
        className="h-64 w-full overflow-hidden rounded-lg border"
      />
      {!value && (
        <p className="text-sm text-muted-foreground">
          คลิกบนแผนที่เพื่อกำหนดตำแหน่ง
        </p>
      )}
      {tilesFailed && (
        <p className="text-sm text-destructive">โหลดแผนที่ไม่สำเร็จ</p>
      )}
      <div className="grid grid-cols-2 gap-2">
        <Field>
          <FieldLabel htmlFor="pier-lat">ละติจูด</FieldLabel>
          <Input
            id="pier-lat"
            type="text"
            inputMode="decimal"
            aria-invalid={latInvalid}
            value={latText}
            onChange={(e) => handleLatTextChange(e.target.value)}
          />
          {latInvalid && (
            <FieldError>ละติจูดต้องอยู่ระหว่าง -90 ถึง 90</FieldError>
          )}
        </Field>
        <Field>
          <FieldLabel htmlFor="pier-lng">ลองจิจูด</FieldLabel>
          <Input
            id="pier-lng"
            type="text"
            inputMode="decimal"
            aria-invalid={lngInvalid}
            value={lngText}
            onChange={(e) => handleLngTextChange(e.target.value)}
          />
          {lngInvalid && (
            <FieldError>ลองจิจูดต้องอยู่ระหว่าง -180 ถึง 180</FieldError>
          )}
        </Field>
      </div>
    </div>
  );
}
