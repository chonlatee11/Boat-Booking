import { useQuery } from '@tanstack/react-query';
import { rpc } from '@/lib/api';
import type {
  ListPiersResponseJson,
  ListOperatorsResponseJson,
} from '@gen/services/catalog/v1/catalog_pb';

// Hooks for the piers page live here (not src/lib/queries.ts) so the three
// wave-5 admin plans never edit the same file.

export function useAdminPiers() {
  return useQuery({
    queryKey: ['piers'],
    queryFn: () => rpc<ListPiersResponseJson>('catalog', 'ListPiers'),
  });
}

// Same ['operators'] cache key as src/lib/queries.ts's useOperators() so the
// operators list is fetched once and shared across both pages. Always
// enabled — ListOperators is safe for every role (server-side scope narrows
// the result), the operator field itself is only rendered for super_admin.
export function useOperatorOptions() {
  return useQuery({
    queryKey: ['operators'],
    queryFn: () => rpc<ListOperatorsResponseJson>('catalog', 'ListOperators'),
  });
}
