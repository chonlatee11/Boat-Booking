'use client';

import { useTranslations } from 'next-intl';
import { useQuery } from '@tanstack/react-query';
import { apiFetch } from '@/lib/api';
import type { ListBoatsResponseJson } from '@gen/services/catalog/v1/catalog_pb';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

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
      <div className="flex flex-col gap-3">
        <p>{t('error', { status })}</p>
        <Button className="h-11" onClick={() => refetch()}>
          {t('retry')}
        </Button>
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
        <Card key={boat.boatId}>
          <CardHeader>
            <CardTitle className="text-lg">{boat.name}</CardTitle>
          </CardHeader>
          <CardContent className="flex items-center justify-between text-sm">
            <span>{t('capacity', { count: boat.defaultCapacity ?? 0 })}</span>
            <span className="rounded-full bg-muted px-2 py-1 text-xs font-medium">
              {boat.status}
            </span>
          </CardContent>
        </Card>
      ))}
      <Button className="h-11" onClick={() => refetch()}>
        {t('retry')}
      </Button>
    </div>
  );
}
