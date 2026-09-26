'use client';

import { useState } from 'react';
import { PlusIcon } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import { rpc } from '@/lib/api';
import { useRoutePrices } from './queries';
import { bahtToSatang, formatSatang } from './money';
import type { TicketTypeJson } from '@gen/events/catalog/v1/price_pb';
import { Field, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select';
import { Button } from '@/components/ui/button';
import { Spinner } from '@/components/ui/spinner';
import { Skeleton } from '@/components/ui/skeleton';

function todayBangkok(): string {
  return new Intl.DateTimeFormat('en-CA', {
    timeZone: 'Asia/Bangkok',
  }).format(new Date());
}

function ticketTypeLabel(t: TicketTypeJson | undefined): string {
  return t === 'TICKET_TYPE_CHILD' ? 'เด็ก' : 'ผู้ใหญ่';
}

export function PriceSection({ routeId }: { routeId: string }) {
  const { data, isLoading } = useRoutePrices(routeId);
  const queryClient = useQueryClient();
  const prices = data?.prices ?? [];

  const today = todayBangkok();
  const [ticketType, setTicketType] =
    useState<TicketTypeJson>('TICKET_TYPE_ADULT');
  const [amountBaht, setAmountBaht] = useState('');
  const [effectiveFrom, setEffectiveFrom] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const satang = bahtToSatang(amountBaht);
  const isValid =
    satang !== null && Boolean(effectiveFrom) && effectiveFrom >= today;

  async function handleAdd() {
    if (satang === null) return;
    setPending(true);
    setError(null);
    try {
      await rpc('catalog', 'AddRoutePrice', {
        routeId,
        ticketType,
        amountSatang: satang,
        effectiveFrom,
      });
      setAmountBaht('');
      setEffectiveFrom('');
      await queryClient.invalidateQueries({
        queryKey: ['route-prices', routeId],
      });
      await queryClient.invalidateQueries({ queryKey: ['routes'] });
    } catch {
      setError('บันทึกราคาไม่สำเร็จ กรุณาลองใหม่');
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm font-medium">ราคา</p>
      {isLoading ? (
        <div className="flex flex-col gap-2">
          {Array.from({ length: 2 }).map((_, i) => (
            <Skeleton key={i} className="h-9 w-full" />
          ))}
        </div>
      ) : prices.length === 0 ? (
        <div className="rounded-lg border p-3 text-sm text-muted-foreground">
          <p className="font-medium text-foreground">ยังไม่ได้ตั้งราคา</p>
          <p>ตั้งราคาผู้ใหญ่และเด็กสำหรับเส้นทางนี้</p>
        </div>
      ) : (
        <div className="max-h-48 overflow-y-auto rounded-lg border">
          <ul className="divide-y">
            {prices.map((p, i) => (
              <li
                key={i}
                className="flex items-center justify-between gap-2 px-3 py-2 text-sm"
              >
                <span>{p.effectiveFrom}</span>
                <span>{ticketTypeLabel(p.ticketType)}</span>
                <span>{formatSatang(p.amountSatang ?? '0')}</span>
              </li>
            ))}
          </ul>
        </div>
      )}
      <div className="flex flex-col gap-2">
        <div className="grid grid-cols-2 gap-2">
          <Field>
            <FieldLabel htmlFor="price-ticket-type">ประเภทตั๋ว</FieldLabel>
            <NativeSelect
              id="price-ticket-type"
              value={ticketType}
              disabled={pending}
              onChange={(e) => setTicketType(e.target.value as TicketTypeJson)}
            >
              <NativeSelectOption value="TICKET_TYPE_ADULT">
                ผู้ใหญ่
              </NativeSelectOption>
              <NativeSelectOption value="TICKET_TYPE_CHILD">
                เด็ก
              </NativeSelectOption>
            </NativeSelect>
          </Field>
          <Field>
            <FieldLabel htmlFor="price-amount">ราคา (บาท)</FieldLabel>
            <Input
              id="price-amount"
              inputMode="decimal"
              value={amountBaht}
              disabled={pending}
              onChange={(e) => setAmountBaht(e.target.value)}
            />
          </Field>
        </div>
        <Field>
          <FieldLabel htmlFor="price-effective-from">วันที่มีผล</FieldLabel>
          <Input
            id="price-effective-from"
            type="date"
            min={today}
            value={effectiveFrom}
            disabled={pending}
            onChange={(e) => setEffectiveFrom(e.target.value)}
          />
        </Field>
        {error && <p className="text-sm text-destructive">{error}</p>}
        <Button
          type="button"
          variant="outline"
          disabled={!isValid || pending}
          onClick={handleAdd}
        >
          {pending && <Spinner />}
          <PlusIcon className="size-4" />
          เพิ่มราคาใหม่
        </Button>
      </div>
    </div>
  );
}
