'use client';

import { useEffect, useRef, useState } from 'react';
import { Map as MapLibreMap, Marker } from 'maplibre-gl';
import 'maplibre-gl/dist/maplibre-gl.css';
import { Field, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';

// D-20: OpenFreeMap by default, swappable via env — no API key, no
// geocoding. Thailand center/zoom used until a marker is placed.
const STYLE_URL =
  process.env.NEXT_PUBLIC_MAP_STYLE_URL ??
  'https://tiles.openfreemap.org/styles/liberty';
const THAILAND_CENTER: [number, number] = [100.5, 13.7];

export type LatLng = { lat: number; lng: number };

/** Click/drag marker picker with manual lat/lng fallback (D-20). */
export function MapPicker({
  value,
  onChange,
}: {
  value?: LatLng;
  onChange: (value: LatLng) => void;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<MapLibreMap | null>(null);
  const markerRef = useRef<Marker | null>(null);
  const onChangeRef = useRef(onChange);
  const [tilesFailed, setTilesFailed] = useState(false);

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
    map.on('error', () => setTilesFailed(true));

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

  function handleLatChange(raw: string) {
    const lat = Number(raw);
    if (!Number.isFinite(lat)) return;
    onChange({ lat, lng: value?.lng ?? THAILAND_CENTER[0] });
  }

  function handleLngChange(raw: string) {
    const lng = Number(raw);
    if (!Number.isFinite(lng)) return;
    onChange({ lat: value?.lat ?? THAILAND_CENTER[1], lng });
  }

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
            type="number"
            step={0.000001}
            value={value?.lat ?? ''}
            onChange={(e) => handleLatChange(e.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="pier-lng">ลองจิจูด</FieldLabel>
          <Input
            id="pier-lng"
            type="number"
            step={0.000001}
            value={value?.lng ?? ''}
            onChange={(e) => handleLngChange(e.target.value)}
          />
        </Field>
      </div>
    </div>
  );
}
