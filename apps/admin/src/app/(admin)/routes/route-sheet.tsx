'use client';

import { useState } from 'react';
import { RepeatIcon } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { rpc, type ApiError } from '@/lib/api';
import { useOwnPiers, usePublicPiers } from './queries';
import { PolicyEditor, validatePolicy } from './policy-editor';
import { PriceSection } from './price-section';
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
import { Separator } from '@/components/ui/separator';
import { Button } from '@/components/ui/button';
import { Spinner } from '@/components/ui/spinner';

export const DEFAULT_POLICY: CancellationTierJson[] = [
  { minHoursBefore: 24, refundPercent: 100 },
  { minHoursBefore: 2, refundPercent: 50 },
  { minHoursBefore: 0, refundPercent: 0 },
];

export function RouteSheet({
  route,
  returnOf,
  trigger,
}: {
  route?: RouteJson;
  /** Set instead of `route` to open a create Sheet pre-filled with this
   * route's piers swapped, same duration/policy (D-11's "return route"). */
  returnOf?: RouteJson;
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
  const [policy, setPolicy] = useState<CancellationTierJson[]>(DEFAULT_POLICY);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function handleOpenChange(next: boolean) {
    setOpen(next);
    if (next) {
      const source = returnOf;
      if (source) {
        // D-11 return route: swap pier_from/pier_to, keep duration/policy,
        // never copy prices (fares often differ by direction).
        setPierFromId(source.pierToId ?? '');
        setPierToId(source.pierFromId ?? '');
        setDurationMinutes(
          source.durationMinutes ? String(source.durationMinutes) : '',
        );
        setPolicy(source.cancellationPolicy ?? DEFAULT_POLICY);
      } else {
        setPierFromId(route?.pierFromId ?? '');
        setPierToId(route?.pierToId ?? '');
        setDurationMinutes(
          route?.durationMinutes ? String(route.durationMinutes) : '',
        );
        setPolicy(route?.cancellationPolicy ?? DEFAULT_POLICY);
      }
      setError(null);
    }
  }

  const ownPiers = (ownPiersData?.piers ?? []).filter((p) => !p.archived);
  const publicPiers = (publicPiersData?.piers ?? []).filter(
    (p) => !p.archived && p.pierId !== pierFromId,
  );

  // Return-route swap may land on a pier outside the caller's own piers
  // (the backend would reject it with NotFound) — catch it client-side.
  const originOutOfScope =
    Boolean(returnOf) &&
    Boolean(pierFromId) &&
    ownPiersData !== undefined &&
    !ownPiers.some((p) => p.pierId === pierFromId);

  const durationValid =
    /^\d+$/.test(durationMinutes) &&
    Number(durationMinutes) >= 1 &&
    Number(durationMinutes) <= 1440;
  const policyError = validatePolicy(policy);
  const isValid =
    Boolean(pierFromId) &&
    Boolean(pierToId) &&
    durationValid &&
    !policyError &&
    !originOutOfScope;

  async function handleSubmit() {
    setPending(true);
    setError(null);
    try {
      await rpc<UpsertRouteResponseJson>('catalog', 'UpsertRoute', {
        routeId: isEdit ? (route?.routeId ?? '') : '',
        pierFromId,
        pierToId,
        durationMinutes: Number(durationMinutes),
        cancellationPolicy: policy,
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
              {originOutOfScope && (
                <FieldError>
                  ท่านี้ไม่ได้อยู่ในความรับผิดชอบของคุณ กรุณาเลือกท่าต้นทางอื่น
                </FieldError>
              )}
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
            <Separator />
            <div>
              <p className="mb-2 text-sm font-medium">นโยบายยกเลิก</p>
              <PolicyEditor
                tiers={policy}
                onChange={setPolicy}
                disabled={pending}
              />
            </div>
            {isEdit && route?.routeId && (
              <>
                <Separator />
                <PriceSection routeId={route.routeId} />
              </>
            )}
            {isEdit && route && !returnOf && (
              <RouteSheet
                returnOf={route}
                trigger={
                  <Button type="button" variant="outline">
                    <RepeatIcon className="size-4" />
                    สร้างเส้นทางย้อนกลับ
                  </Button>
                }
              />
            )}
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
