'use client';

import { useTranslations } from 'next-intl';
import { useQuery } from '@tanstack/react-query';
import { apiFetch } from '@/lib/api';
import type { ListBoatsResponseJson } from '@gen/services/catalog/v1/catalog_pb';

export function BoatList() {
  const t = useTranslations('boats');

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['boats'],
    queryFn: () => apiFetch<ListBoatsResponseJson>('/api/v1/public/boats'),
  });

  if (isLoading) {
    return <p>{t('loading')}</p>;
  }

  if (isError) {
    const status = (error as Error & { status?: number }).status ?? '—';
    return (
      <div>
        <p>{t('error', { status })}</p>
        <button
          type="button"
          onClick={() => refetch()}
          className="min-h-11 rounded-xl bg-[#0B6E99] px-4 py-2 text-white"
        >
          {t('retry')}
        </button>
      </div>
    );
  }

  const boats = data?.boats ?? [];

  if (boats.length === 0) {
    return <p>{t('empty')}</p>;
  }

  return (
    <div className="flex flex-col gap-3">
      {boats.map((boat) => (
        <div key={boat.boatId} className="rounded-xl border border-black/10 p-4">
          <p className="text-lg font-semibold">{boat.name}</p>
          <p>{t('capacity', { count: boat.defaultCapacity ?? 0 })}</p>
          <p>{boat.status}</p>
        </div>
      ))}
      <button
        type="button"
        onClick={() => refetch()}
        className="min-h-11 rounded-xl bg-[#0B6E99] px-4 py-2 text-white"
      >
        {t('retry')}
      </button>
    </div>
  );
}
