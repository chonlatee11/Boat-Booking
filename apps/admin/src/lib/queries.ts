import { useQuery } from '@tanstack/react-query';
import { apiFetch, rpc } from '@/lib/api';
import type { ListOperatorsResponseJson } from '@gen/services/catalog/v1/catalog_pb';

export type Role = 'customer' | 'staff' | 'pier_admin' | 'super_admin';

export type Whoami = {
  sub: string;
  operator_id: string;
  role: Role;
  pier_ids: string[];
};

export function useWhoami() {
  return useQuery({
    queryKey: ['whoami'],
    queryFn: () => apiFetch<Whoami>('/api/v1/whoami'),
    retry: false,
  });
}

export function useOperators() {
  return useQuery({
    queryKey: ['operators'],
    queryFn: () => rpc<ListOperatorsResponseJson>('catalog', 'ListOperators'),
  });
}
