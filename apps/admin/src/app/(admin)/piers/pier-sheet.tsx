'use client';

import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { rpc, type ApiError } from '@/lib/api';
import { useWhoami } from '@/lib/queries';
import { useOperatorOptions } from './queries';
import type {
  PierJson,
  UpsertPierResponseJson,
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
import { Textarea } from '@/components/ui/textarea';
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select';
import { Skeleton } from '@/components/ui/skeleton';
import { Button } from '@/components/ui/button';
import { Spinner } from '@/components/ui/spinner';
import { MapPicker, type LatLng } from '@/components/map-picker';
import { PhotoUpload } from '@/components/photo-upload';

// Zero-padded 24h "HH:MM" — locale-independent, so a native time picker
// rendered in a 12h AM/PM locale can no longer silently leave the value
// empty until the meridiem is committed (G-02-8).
const HHMM = /^([01]\d|2[0-3]):[0-5]\d$/;

export function PierSheet({
  pier,
  piersLoading,
  trigger,
}: {
  pier?: PierJson;
  piersLoading?: boolean;
  trigger: React.ReactNode;
}) {
  const isEdit = Boolean(pier?.pierId);
  const { data: whoami } = useWhoami();
  const isSuperAdmin = whoami?.role === 'super_admin';
  const { data: operatorsData } = useOperatorOptions();
  const queryClient = useQueryClient();

  const [open, setOpen] = useState(false);
  const [operatorId, setOperatorId] = useState('');
  const [nameTh, setNameTh] = useState('');
  const [nameEn, setNameEn] = useState('');
  const [address, setAddress] = useState('');
  const [opensAt, setOpensAt] = useState('');
  const [closesAt, setClosesAt] = useState('');
  const [location, setLocation] = useState<LatLng | undefined>(undefined);
  const [photoKey, setPhotoKey] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function handleOpenChange(next: boolean) {
    setOpen(next);
    if (next) {
      setOperatorId(pier?.operatorId ?? '');
      setNameTh(pier?.nameTh ?? '');
      setNameEn(pier?.nameEn ?? '');
      setAddress(pier?.address ?? '');
      setOpensAt(pier?.opensAt ?? '');
      setClosesAt(pier?.closesAt ?? '');
      setLocation(
        typeof pier?.lat === 'number' && typeof pier?.lng === 'number'
          ? { lat: pier.lat, lng: pier.lng }
          : undefined,
      );
      setPhotoKey(pier?.photoKey ?? '');
      setError(null);
    }
  }

  const trimmedNameTh = nameTh.trim();
  const trimmedNameEn = nameEn.trim();
  const namesValid = trimmedNameTh.length > 0 && trimmedNameEn.length > 0;
  // IN-07: a super_admin creating a pier must pick an operator, or the
  // server sends operatorId "" and answers NotFound with a generic
  // failure message. isEdit/staff callers keep the stored operatorId
  // (input 100 sends it unconditionally), so this only gates the
  // super_admin-create case.
  const operatorValid = isEdit || !isSuperAdmin || Boolean(operatorId);
  const hoursPartial = Boolean(opensAt) !== Boolean(closesAt);
  const hoursMalformed =
    (Boolean(opensAt) && !HHMM.test(opensAt)) ||
    (Boolean(closesAt) && !HHMM.test(closesAt));
  // Zero-padded "HH:MM" strings compare correctly with </>=.
  const hoursOutOfOrder =
    !hoursMalformed &&
    Boolean(opensAt) &&
    Boolean(closesAt) &&
    opensAt >= closesAt;
  const hoursValid = !hoursPartial && !hoursMalformed && !hoursOutOfOrder;
  const isValid =
    namesValid && Boolean(location) && hoursValid && operatorValid;
  // Edit mode has no dedicated GetPier fetch — the row is already in memory
  // from the ListPiers query backing the table (same pattern as 02-09's
  // operator-dialog) — but if the list happens to be refetching while the
  // Sheet is open (e.g. a retry click), show the skeleton rather than a
  // half-stale form.
  const showSkeleton = isEdit && Boolean(piersLoading);

  async function handleSubmit() {
    if (!location) return;
    setPending(true);
    setError(null);
    try {
      await rpc<UpsertPierResponseJson>('catalog', 'UpsertPier', {
        pierId: pier?.pierId ?? '',
        operatorId: pier?.operatorId ?? operatorId,
        nameTh: trimmedNameTh,
        nameEn: trimmedNameEn,
        lat: location.lat,
        lng: location.lng,
        address: address.trim(),
        opensAt,
        closesAt,
        photoKey,
      });
      toast.success('บันทึกท่าเรือสำเร็จ');
      await queryClient.invalidateQueries({ queryKey: ['piers'] });
      setOpen(false);
    } catch (err) {
      const apiErr = err as ApiError;
      // D-08: on create, failed_precondition can only mean the chosen
      // operator was archived (createPier's other error codes are
      // permission/not-found/invalid-argument) — map by code, not by
      // message text, so this doesn't depend on 02-15's backend wording.
      if (!isEdit && apiErr.code === 'failed_precondition') {
        setError('ผู้ประกอบการนี้ถูกเก็บถาวรแล้ว กรุณาเลือกผู้ประกอบการอื่น');
      } else {
        setError('บันทึกท่าเรือไม่สำเร็จ กรุณาลองใหม่');
      }
    } finally {
      setPending(false);
    }
  }

  const operators = operatorsData?.operators ?? [];

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetTrigger asChild>{trigger}</SheetTrigger>
      <SheetContent className="flex flex-col gap-0 p-0 sm:max-w-md">
        <SheetHeader>
          <SheetTitle>
            {isEdit ? 'แก้ไขท่าเรือ' : 'เพิ่มท่าเรือใหม่'}
          </SheetTitle>
        </SheetHeader>
        <div className="flex-1 overflow-y-auto px-4">
          {showSkeleton ? (
            <div className="flex flex-col gap-4 pb-4">
              {Array.from({ length: 5 }).map((_, i) => (
                <Skeleton key={i} className="h-9 w-full" />
              ))}
            </div>
          ) : (
            <div className="flex flex-col gap-4 pb-4">
              {!isEdit && isSuperAdmin && (
                <Field>
                  <FieldLabel htmlFor="pier-operator">ผู้ประกอบการ</FieldLabel>
                  <NativeSelect
                    id="pier-operator"
                    value={operatorId}
                    disabled={pending}
                    onChange={(e) => setOperatorId(e.target.value)}
                  >
                    <NativeSelectOption value="">
                      เลือกผู้ประกอบการ
                    </NativeSelectOption>
                    {operators
                      .filter((op) => !op.archived)
                      .map((op) => (
                        <NativeSelectOption
                          key={op.operatorId}
                          value={op.operatorId ?? ''}
                        >
                          {op.name}
                        </NativeSelectOption>
                      ))}
                  </NativeSelect>
                  {!operatorValid && (
                    <FieldError>กรุณาเลือกผู้ประกอบการ</FieldError>
                  )}
                </Field>
              )}
              <Field>
                <FieldLabel htmlFor="pier-name-th">
                  ชื่อท่าเรือ (ไทย)
                </FieldLabel>
                <Input
                  id="pier-name-th"
                  className="break-words"
                  value={nameTh}
                  disabled={pending}
                  onChange={(e) => setNameTh(e.target.value)}
                />
                {!trimmedNameTh && (
                  <FieldError>กรุณากรอกชื่อท่าเรือ (ไทย)</FieldError>
                )}
              </Field>
              <Field>
                <FieldLabel htmlFor="pier-name-en">
                  ชื่อท่าเรือ (อังกฤษ)
                </FieldLabel>
                <Input
                  id="pier-name-en"
                  className="break-words"
                  value={nameEn}
                  disabled={pending}
                  onChange={(e) => setNameEn(e.target.value)}
                />
                {!trimmedNameEn && (
                  <FieldError>กรุณากรอกชื่อท่าเรือ (อังกฤษ)</FieldError>
                )}
              </Field>
              <Field>
                <FieldLabel htmlFor="pier-address">ที่อยู่</FieldLabel>
                <Textarea
                  id="pier-address"
                  className="break-words"
                  value={address}
                  disabled={pending}
                  onChange={(e) => setAddress(e.target.value)}
                />
              </Field>
              <div className="grid grid-cols-2 gap-2">
                <Field>
                  <FieldLabel htmlFor="pier-opens">เวลาเปิด</FieldLabel>
                  <Input
                    id="pier-opens"
                    type="text"
                    placeholder="08:00"
                    maxLength={5}
                    value={opensAt}
                    disabled={pending}
                    onChange={(e) => setOpensAt(e.target.value.trim())}
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="pier-closes">เวลาปิด</FieldLabel>
                  <Input
                    id="pier-closes"
                    type="text"
                    placeholder="17:00"
                    maxLength={5}
                    value={closesAt}
                    disabled={pending}
                    onChange={(e) => setClosesAt(e.target.value.trim())}
                  />
                </Field>
              </div>
              {hoursPartial && (
                <FieldError>กรุณากรอกเวลาเปิดและเวลาปิดทั้งสองช่อง</FieldError>
              )}
              {hoursMalformed && (
                <FieldError>
                  กรุณากรอกเวลาแบบ 24 ชั่วโมง (HH:MM) เช่น 08:00
                </FieldError>
              )}
              {hoursOutOfOrder && (
                <FieldError>เวลาเปิดต้องอยู่ก่อนเวลาปิด</FieldError>
              )}
              <Field>
                <FieldLabel>ตำแหน่งที่ตั้ง</FieldLabel>
                <MapPicker value={location} onChange={setLocation} />
              </Field>
              <Field>
                <FieldLabel>รูปภาพ</FieldLabel>
                <PhotoUpload
                  photoKey={photoKey}
                  photoUrl={pier?.photoUrl}
                  onChange={setPhotoKey}
                />
              </Field>
            </div>
          )}
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
