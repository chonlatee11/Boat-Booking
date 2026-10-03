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
 * across the given pier lists (flattened, since ownPiers and publicPiers
 * overlap), falling back to a Thai not-found label if not found in any of
 * them. When another pier in the lists shares the same Thai name
 * (G-02-9 — e.g. a route and its reverse between two same-named piers),
 * appends the last 4 characters of this pier's id in parentheses so the
 * two are distinguishable. Uses the id's tail, not its head: UUIDv7 ids
 * start with a millisecond timestamp, so piers created close together
 * share a prefix, while the tail is random.
 */
export function pierName(
  id: string | undefined,
  ...pierLists: (PierJson[] | undefined)[]
): string {
  if (!id) return '';
  const piers = pierLists.flatMap((list) => list ?? []);
  const found = piers.find((p) => p.pierId === id);
  if (!found || !found.nameTh) return 'ไม่พบท่าเรือ';
  const hasDuplicate = piers.some(
    (p) => p.pierId !== id && p.nameTh === found.nameTh,
  );
  return hasDuplicate ? `${found.nameTh} (${id.slice(-4)})` : found.nameTh;
}
