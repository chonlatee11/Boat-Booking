'use client';

import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { rpc } from '@/lib/api';
import { useOwnPiers } from './queries';
import type {
  BoatJson,
  UpsertBoatResponseJson,
} from '@gen/services/catalog/v1/catalog_pb';
import type { BoatStatusJson } from '@gen/events/catalog/v1/boat_pb';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Field, FieldLabel, FieldError } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select';
import { Skeleton } from '@/components/ui/skeleton';
import { Button } from '@/components/ui/button';
import { Spinner } from '@/components/ui/spinner';

const MAX_NAME_LENGTH = 100;

export function BoatDialog({
  boat,
  boatsLoading,
  trigger,
}: {
  boat?: BoatJson;
  boatsLoading?: boolean;
  trigger: React.ReactNode;
}) {
  const isEdit = Boolean(boat?.boatId);
  const { data: ownPiersData } = useOwnPiers();
  const queryClient = useQueryClient();

  const [open, setOpen] = useState(false);
  const [name, setName] = useState('');
  const [defaultCapacity, setDefaultCapacity] = useState('');
  const [status, setStatus] = useState<BoatStatusJson>('BOAT_STATUS_ACTIVE');
  const [homePierId, setHomePierId] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [nameTouched, setNameTouched] = useState(false);

  function handleOpenChange(next: boolean) {
    setOpen(next);
    if (next) {
      setName(boat?.name ?? '');
      setDefaultCapacity(
        boat?.defaultCapacity ? String(boat.defaultCapacity) : '',
      );
      setStatus(boat?.status ?? 'BOAT_STATUS_ACTIVE');
      setHomePierId(boat?.homePierId ?? '');
      setError(null);
      setNameTouched(false);
    }
  }

  const ownPiers = (ownPiersData?.piers ?? []).filter((p) => !p.archived);
  const trimmedName = name.trim();
  const nameValid =
    trimmedName.length >= 1 && trimmedName.length <= MAX_NAME_LENGTH;
  const capacityValid =
    /^\d+$/.test(defaultCapacity) &&
    Number(defaultCapacity) >= 1 &&
    Number(defaultCapacity) <= 1000;
  const isValid = nameValid && capacityValid && Boolean(homePierId);
  // Edit mode has no dedicated GetBoat fetch — the row is already in memory
  // from the ListBoats query backing the table (02-09's operator-dialog /
  // 02-11's pier-sheet precedent) — the skeleton only shows if the boats
  // list happens to be refetching while the Dialog is open.
  const showSkeleton = isEdit && Boolean(boatsLoading);

  async function handleSubmit() {
    setPending(true);
    setError(null);
    try {
      await rpc<UpsertBoatResponseJson>('catalog', 'UpsertBoat', {
        boatId: boat?.boatId ?? '',
        name: trimmedName,
        defaultCapacity: Number(defaultCapacity),
        status,
        homePierId,
      });
      toast.success('บันทึกเรือสำเร็จ');
      await queryClient.invalidateQueries({ queryKey: ['boats'] });
      setOpen(false);
    } catch {
      setError('บันทึกเรือไม่สำเร็จ กรุณาลองใหม่');
    } finally {
      setPending(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{isEdit ? 'แก้ไขเรือ' : 'เพิ่มเรือใหม่'}</DialogTitle>
        </DialogHeader>
        {showSkeleton ? (
          <div className="flex flex-col gap-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className="h-9 w-full" />
            ))}
          </div>
        ) : (
          <div className="flex flex-col gap-4">
            <Field>
              <FieldLabel htmlFor="boat-name">ชื่อเรือ</FieldLabel>
              <Input
                id="boat-name"
                className="break-words"
                value={name}
                maxLength={MAX_NAME_LENGTH}
                disabled={pending}
                autoFocus
                onChange={(e) => {
                  setName(e.target.value);
                  setNameTouched(true);
                }}
              />
              {nameTouched && !trimmedName && (
                <FieldError>กรุณากรอกชื่อเรือ</FieldError>
              )}
            </Field>
            <Field>
              <FieldLabel htmlFor="boat-capacity">ความจุเริ่มต้น</FieldLabel>
              <Input
                id="boat-capacity"
                type="number"
                min={1}
                max={1000}
                value={defaultCapacity}
                disabled={pending}
                onChange={(e) => setDefaultCapacity(e.target.value)}
              />
              {!capacityValid && defaultCapacity && (
                <FieldError>ความจุต้องอยู่ระหว่าง 1-1000</FieldError>
              )}
            </Field>
            <Field>
              <FieldLabel htmlFor="boat-status">สถานะ</FieldLabel>
              <NativeSelect
                id="boat-status"
                value={status}
                disabled={pending}
                onChange={(e) => setStatus(e.target.value as BoatStatusJson)}
              >
                <NativeSelectOption value="BOAT_STATUS_ACTIVE">
                  ใช้งาน
                </NativeSelectOption>
                <NativeSelectOption value="BOAT_STATUS_MAINTENANCE">
                  ซ่อมบำรุง
                </NativeSelectOption>
              </NativeSelect>
            </Field>
            <Field>
              <FieldLabel htmlFor="boat-home-pier">ท่าประจำ</FieldLabel>
              <NativeSelect
                id="boat-home-pier"
                value={homePierId}
                disabled={pending}
                onChange={(e) => setHomePierId(e.target.value)}
              >
                <NativeSelectOption value="">เลือกท่าประจำ</NativeSelectOption>
                {ownPiers.map((p) => (
                  <NativeSelectOption key={p.pierId} value={p.pierId ?? ''}>
                    {p.nameTh}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </Field>
          </div>
        )}
        <DialogFooter className="flex-col items-stretch gap-2 sm:flex-col">
          {error && <p className="text-sm text-destructive">{error}</p>}
          <Button disabled={!isValid || pending} onClick={handleSubmit}>
            {pending && <Spinner />}
            บันทึก
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
