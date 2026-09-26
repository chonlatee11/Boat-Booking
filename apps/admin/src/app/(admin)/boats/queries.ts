import { useQuery } from '@tanstack/react-query';
import { rpc } from '@/lib/api';
import type {
  ListBoatsResponseJson,
  ListPiersResponseJson,
  PierJson,
} from '@gen/services/catalog/v1/catalog_pb';

// Hooks for the boats page live here (not src/lib/queries.ts) so the three
// wave-5 admin plans never edit the same file (02-11 pattern).

export function useBoats() {
  return useQuery({
    queryKey: ['boats'],
    queryFn: () => rpc<ListBoatsResponseJson>('catalog', 'ListBoats'),
  });
}

// Same ['piers'] cache key as the piers/routes pages' scoped ListPiers
// fetch — one fetch shared across every admin page that needs "own piers".
export function useOwnPiers() {
  return useQuery({
    queryKey: ['piers'],
    queryFn: () => rpc<ListPiersResponseJson>('catalog', 'ListPiers'),
  });
}

/** Looks up a pier's Thai name by id, falling back to the id's first 8 characters. */
export function pierName(id: string | undefined, piers: PierJson[]): string {
  if (!id) return '—';
  const found = piers.find((p) => p.pierId === id);
  return found?.nameTh ?? id.slice(0, 8);
}
