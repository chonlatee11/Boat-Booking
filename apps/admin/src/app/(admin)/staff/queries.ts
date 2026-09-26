import { useQuery } from '@tanstack/react-query';
import { rpc } from '@/lib/api';
import type { ListUsersResponseJson } from '@gen/services/identity/v1/users_pb';
import type {
  ListOperatorsResponseJson,
  ListPiersResponseJson,
  OperatorJson,
  PierJson,
} from '@gen/services/catalog/v1/catalog_pb';

// Hooks for the staff page live here (not src/lib/queries.ts) so this plan
// never edits the same file as its sibling wave-5 admin plans (02-11/02-12
// pattern).

export function useStaffUsers(enabled = true) {
  return useQuery({
    queryKey: ['staff-users'],
    // Non-generic rpc call (matches the archive-call precedent in
    // operators/piers/boats/routes pages) with the response type applied
    // via cast instead of a type argument.
    queryFn: () => rpc('users', 'ListUsers') as Promise<ListUsersResponseJson>,
    enabled,
  });
}

export function useOperatorOptions(enabled = true) {
  return useQuery({
    queryKey: ['operators'],
    queryFn: () => rpc<ListOperatorsResponseJson>('catalog', 'ListOperators'),
    enabled,
  });
}

// Unscoped ListPiers as a super_admin caller (this page is super_admin-only)
// returns every pier across every operator (services/catalog's ListPiers:
// scope.All() with no operator_id filter) — same ['piers'] cache key as the
// piers/routes/boats pages' own-piers fetch (02-11 pattern), since it is the
// identical RPC call.
export function useAllPiers(enabled = true) {
  return useQuery({
    queryKey: ['piers'],
    queryFn: () => rpc<ListPiersResponseJson>('catalog', 'ListPiers'),
    enabled,
  });
}

// Scoped to one operator (super_admin's operator_id filter) — the pier
// checkbox list in the staff Dialog; only fetches once an operator is
// chosen.
export function usePiersForOperator(operatorId: string) {
  return useQuery({
    queryKey: ['piers-for-operator', operatorId],
    queryFn: () =>
      rpc<ListPiersResponseJson>('catalog', 'ListPiers', { operatorId }),
    enabled: Boolean(operatorId),
  });
}

/** Looks up an operator's name by id, falling back to the id's first 8 characters. */
export function operatorName(
  id: string | undefined,
  operators: OperatorJson[],
): string {
  if (!id) return '—';
  const found = operators.find((o) => o.operatorId === id);
  return found?.name ?? id.slice(0, 8);
}

/** Looks up a pier's Thai name by id, falling back to the id's first 8 characters. */
export function pierName(id: string, piers: PierJson[]): string {
  const found = piers.find((p) => p.pierId === id);
  return found?.nameTh ?? id.slice(0, 8);
}
