import { useQuery } from '@tanstack/react-query';
import { apiFetch, rpc } from '@/lib/api';
import type {
  ListRoutesResponseJson,
  ListRoutePricesResponseJson,
  ListPiersResponseJson,
  PierJson,
} from '@gen/services/catalog/v1/catalog_pb';

// Hooks for the routes page live here (not src/lib/queries.ts) so the three
// wave-5 admin plans never edit the same file (02-11 pattern).

export function useAdminRoutes() {
  return useQuery({
    queryKey: ['routes'],
    queryFn: () => rpc<ListRoutesResponseJson>('catalog', 'ListRoutes'),
  });
}

// Same ['piers'] cache key as the piers page's useAdminPiers() (02-11) — one
// scoped ListPiers fetch shared across pages. pier_from choices come from
// here: own (operator-scoped) piers only (D-12).
export function useOwnPiers() {
  return useQuery({
    queryKey: ['piers'],
    queryFn: () => rpc<ListPiersResponseJson>('catalog', 'ListPiers'),
  });
}

// pier_to may be any non-archived pier of any operator (D-12) — the public
// (claim-less) projection, not the scoped admin one.
export function usePublicPiers() {
  return useQuery({
    queryKey: ['public-piers'],
    queryFn: () => apiFetch<ListPiersResponseJson>('/api/v1/public/piers'),
  });
}

export function useRoutePrices(routeId: string | undefined) {
  return useQuery({
    queryKey: ['route-prices', routeId],
    queryFn: () =>
      rpc<ListRoutePricesResponseJson>('catalog', 'ListRoutePrices', {
        routeId,
      }),
    enabled: Boolean(routeId),
  });
}

/**
 * Derived route display name helper (D-17): looks up a pier's Thai name
 * from the given pier lists (checked in order), falling back to the id's
 * first 8 characters if not found in any of them.
 */
export function pierName(
  id: string | undefined,
  ...pierLists: (PierJson[] | undefined)[]
): string {
  if (!id) return '';
  for (const list of pierLists) {
    const found = list?.find((p) => p.pierId === id);
    if (found) return found.nameTh ?? id.slice(0, 8);
  }
  return id.slice(0, 8);
}
