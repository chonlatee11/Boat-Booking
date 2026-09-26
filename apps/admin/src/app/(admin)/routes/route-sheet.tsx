'use client';

import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { rpc, type ApiError } from '@/lib/api';
import { useOwnPiers, usePublicPiers } from './queries';
import type {
  RouteJson,
  UpsertRouteResponseJson,
  CancellationTierJson,
} from '@gen/services/catalog/v1/catalog_pb';
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetFooter,
  SheetTitle,
  SheetTrigger,
} from '@/components/ui/sheet';
import { Field, FieldLabel, FieldError } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select';
import { Button } from '@/components/ui/button';
import { Spinner } from '@/components/ui/spinner';

export const DEFAULT_POLICY: CancellationTierJson[] = [
  { minHoursBefore: 24, refundPercent: 100 },
  { minHoursBefore: 2, refundPercent: 50 },
  { minHoursBefore: 0, refundPercent: 0 },
];

export function RouteSheet({
  route,
  trigger,
}: {
  route?: RouteJson;
  trigger: React.ReactNode;
}) {
  const isEdit = Boolean(route?.routeId);
  const { data: ownPiersData } = useOwnPiers();
  const { data: publicPiersData } = usePublicPiers();
  const queryClient = useQueryClient();

  const [open, setOpen] = useState(false);
  const [pierFromId, setPierFromId] = useState('');
  const [pierToId, setPierToId] = useState('');
  const [durationMinutes, setDurationMinutes] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function handleOpenChange(next: boolean) {
    setOpen(next);
    if (next) {
      setPierFromId(route?.pierFromId ?? '');
      setPierToId(route?.pierToId ?? '');
      setDurationMinutes(
        route?.durationMinutes ? String(route.durationMinutes) : '',
      );
      setError(null);
    }
  }

  const ownPiers = (ownPiersData?.piers ?? []).filter((p) => !p.archived);
  const publicPiers = (publicPiersData?.piers ?? []).filter(
    (p) => !p.archived && p.pierId !== pierFromId,
  );

  const durationValid =
    /^\d+$/.test(durationMinutes) &&
    Number(durationMinutes) >= 1 &&
    Number(durationMinutes) <= 1440;
  const isValid = Boolean(pierFromId) && Boolean(pierToId) && durationValid;

  async function handleSubmit() {
    setPending(true);
    setError(null);
    try {
      await rpc<UpsertRouteResponseJson>('catalog', 'UpsertRoute', {
        routeId: route?.routeId ?? '',
        pierFromId,
        pierToId,
        durationMinutes: Number(durationMinutes),
        cancellationPolicy: route?.cancellationPolicy ?? DEFAULT_POLICY,
      });
      toast.success('บันทึกเส้นทางสำเร็จ');
      await queryClient.invalidateQueries({ queryKey: ['routes'] });
      setOpen(false);
    } catch (err) {
      const apiErr = err as ApiError;
      if (apiErr.code === 'already_exists') {
        setError('มีเส้นทางนี้อยู่แล้ว');
      } else {
        setError('บันทึกเส้นทางไม่สำเร็จ กรุณาลองใหม่');
      }
    } finally {
      setPending(false);
    }
  }

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetTrigger asChild>{trigger}</SheetTrigger>
      <SheetContent className="flex flex-col gap-0 p-0 sm:max-w-md">
        <SheetHeader>
          <SheetTitle>
            {isEdit ? 'แก้ไขเส้นทาง' : 'สร้างเส้นทางใหม่'}
          </SheetTitle>
        </SheetHeader>
        <div className="flex-1 overflow-y-auto px-4">
          <div className="flex flex-col gap-4 pb-4">
            <Field>
              <FieldLabel htmlFor="route-pier-from">ท่าต้นทาง</FieldLabel>
              <NativeSelect
                id="route-pier-from"
                value={pierFromId}
                disabled={pending}
                onChange={(e) => setPierFromId(e.target.value)}
              >
                <NativeSelectOption value="">เลือกท่าต้นทาง</NativeSelectOption>
                {ownPiers.map((p) => (
                  <NativeSelectOption key={p.pierId} value={p.pierId ?? ''}>
                    {p.nameTh}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </Field>
            <Field>
              <FieldLabel htmlFor="route-pier-to">ท่าปลายทาง</FieldLabel>
              <NativeSelect
                id="route-pier-to"
                value={pierToId}
                disabled={pending}
                onChange={(e) => setPierToId(e.target.value)}
              >
                <NativeSelectOption value="">
                  เลือกท่าปลายทาง
                </NativeSelectOption>
                {publicPiers.map((p) => (
                  <NativeSelectOption key={p.pierId} value={p.pierId ?? ''}>
                    {p.nameTh}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </Field>
            <Field>
              <FieldLabel htmlFor="route-duration">
                ระยะเวลาเดินทาง (นาที)
              </FieldLabel>
              <Input
                id="route-duration"
                type="number"
                min={1}
                max={1440}
                value={durationMinutes}
                disabled={pending}
                onChange={(e) => setDurationMinutes(e.target.value)}
              />
              {!durationValid && durationMinutes && (
                <FieldError>ระยะเวลาต้องอยู่ระหว่าง 1-1440 นาที</FieldError>
              )}
            </Field>
            <div className="rounded-lg border p-3 text-sm text-muted-foreground">
              <p className="font-medium text-foreground">
                นโยบายยกเลิก (ค่าเริ่มต้น)
              </p>
              <p>มากกว่า 24 ชม. คืนเงิน 100%</p>
              <p>2–24 ชม. คืนเงิน 50%</p>
              <p>น้อยกว่า 2 ชม. คืนเงิน 0%</p>
            </div>
          </div>
        </div>
        <SheetFooter className="sticky bottom-0 border-t bg-popover">
          {error && <p className="text-sm text-destructive">{error}</p>}
          <Button disabled={!isValid || pending} onClick={handleSubmit}>
            {pending && <Spinner />}
            บันทึก
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  );
}
