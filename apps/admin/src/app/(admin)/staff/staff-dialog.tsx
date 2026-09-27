'use client';

import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { rpc, type ApiError } from '@/lib/api';
import { useOperatorOptions, usePiersForOperator } from './queries';
import type {
  StaffUserJson,
  UpsertUserResponseJson,
} from '@gen/services/identity/v1/users_pb';
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
const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

type StaffRole = 'pier_admin' | 'staff';

export function StaffDialog({
  user,
  usersLoading,
  trigger,
}: {
  user?: StaffUserJson;
  usersLoading?: boolean;
  trigger: React.ReactNode;
}) {
  const isEdit = Boolean(user?.userId);
  const { data: operatorsData } = useOperatorOptions();
  const queryClient = useQueryClient();

  const [open, setOpen] = useState(false);
  const [email, setEmail] = useState('');
  const [name, setName] = useState('');
  const [role, setRole] = useState<StaffRole>('staff');
  const [operatorId, setOperatorId] = useState('');
  const [pierIds, setPierIds] = useState<string[]>([]);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const { data: piersData } = usePiersForOperator(operatorId);

  function handleOpenChange(next: boolean) {
    setOpen(next);
    if (next) {
      setEmail(user?.email ?? '');
      setName(user?.name ?? '');
      setRole((user?.role as StaffRole) ?? 'staff');
      setOperatorId(user?.operatorId ?? '');
      setPierIds(user?.pierIds ?? []);
      setError(null);
    }
  }

  function handleOperatorChange(next: string) {
    setOperatorId(next);
    setPierIds([]);
  }

  function togglePier(pierId: string, checked: boolean) {
    setPierIds((prev) =>
      checked ? [...prev, pierId] : prev.filter((id) => id !== pierId),
    );
  }

  const operators = (operatorsData?.operators ?? []).filter((o) => !o.archived);
  const piers = (piersData?.piers ?? []).filter((p) => !p.archived);

  // WR-08: a pier archived after being assigned stays in user.pierIds
  // (archiving a pier does not touch identity's pier_ids), so `pierIds`
  // state can hold an id that `piers` above no longer lists. Render and
  // submit only the ids still valid for this operator, derived fresh every
  // render rather than synced into state — before piersData has loaded,
  // skip the filter so an about-to-arrive valid pier doesn't flash as
  // unchecked for one render.
  const selectedPierIds = piersData
    ? pierIds.filter((id) => piers.some((p) => p.pierId === id))
    : pierIds;

  const trimmedEmail = email.trim();
  const trimmedName = name.trim();
  const emailValid = EMAIL_PATTERN.test(trimmedEmail);
  const nameValid =
    trimmedName.length >= 1 && trimmedName.length <= MAX_NAME_LENGTH;
  const isValid =
    emailValid &&
    nameValid &&
    Boolean(operatorId) &&
    selectedPierIds.length >= 1;

  // Edit mode has no dedicated GetUser fetch — the row is already in memory
  // from the ListUsers query backing the table (02-09's operator-dialog /
  // 02-12's boat-dialog precedent) — the skeleton only shows if the staff
  // list happens to be refetching while the Dialog is open.
  const showSkeleton = isEdit && Boolean(usersLoading);

  async function handleSubmit() {
    setPending(true);
    setError(null);
    try {
      await rpc<UpsertUserResponseJson>('users', 'UpsertUser', {
        userId: user?.userId ?? '',
        email: trimmedEmail,
        name: trimmedName,
        role,
        operatorId,
        pierIds: selectedPierIds,
      });
      toast.success('บันทึกผู้ใช้งานสำเร็จ');
      await queryClient.invalidateQueries({ queryKey: ['staff-users'] });
      setOpen(false);
    } catch (err) {
      const apiErr = err as ApiError;
      if (apiErr.code === 'already_exists') {
        setError('อีเมลนี้มีผู้ใช้งานแล้ว');
      } else if (apiErr.code === 'invalid_argument') {
        setError(apiErr.message);
      } else {
        setError('บันทึกผู้ใช้งานไม่สำเร็จ กรุณาลองใหม่');
      }
    } finally {
      setPending(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>{trigger}</DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {isEdit ? 'แก้ไขผู้ใช้งาน' : 'เพิ่มผู้ใช้งานใหม่'}
          </DialogTitle>
        </DialogHeader>
        {showSkeleton ? (
          <div className="flex flex-col gap-4">
            {Array.from({ length: 5 }).map((_, i) => (
              <Skeleton key={i} className="h-9 w-full" />
            ))}
          </div>
        ) : (
          <div className="flex flex-col gap-4">
            <Field>
              <FieldLabel htmlFor="staff-email">อีเมล</FieldLabel>
              <Input
                id="staff-email"
                type="email"
                value={email}
                disabled={pending || isEdit}
                autoFocus={!isEdit}
                onChange={(e) => setEmail(e.target.value)}
              />
              {!isEdit && email && !emailValid && (
                <FieldError>กรุณากรอกอีเมลให้ถูกต้อง</FieldError>
              )}
            </Field>
            <Field>
              <FieldLabel htmlFor="staff-name">ชื่อ</FieldLabel>
              <Input
                id="staff-name"
                className="break-words"
                value={name}
                maxLength={MAX_NAME_LENGTH}
                disabled={pending}
                onChange={(e) => setName(e.target.value)}
              />
              {!trimmedName && <FieldError>กรุณากรอกชื่อ</FieldError>}
            </Field>
            <Field>
              <FieldLabel htmlFor="staff-role">บทบาท</FieldLabel>
              <NativeSelect
                id="staff-role"
                value={role}
                disabled={pending}
                onChange={(e) => setRole(e.target.value as StaffRole)}
              >
                <NativeSelectOption value="pier_admin">
                  ผู้ดูแลท่าเรือ
                </NativeSelectOption>
                <NativeSelectOption value="staff">พนักงาน</NativeSelectOption>
              </NativeSelect>
            </Field>
            <Field>
              <FieldLabel htmlFor="staff-operator">ผู้ประกอบการ</FieldLabel>
              <NativeSelect
                id="staff-operator"
                value={operatorId}
                disabled={pending}
                onChange={(e) => handleOperatorChange(e.target.value)}
              >
                <NativeSelectOption value="">
                  เลือกผู้ประกอบการ
                </NativeSelectOption>
                {operators.map((o) => (
                  <NativeSelectOption
                    key={o.operatorId}
                    value={o.operatorId ?? ''}
                  >
                    {o.name}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </Field>
            <Field>
              <FieldLabel>ท่าเรือ</FieldLabel>
              {!operatorId ? (
                <p className="text-sm text-muted-foreground">
                  เลือกผู้ประกอบการก่อน
                </p>
              ) : piers.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  ผู้ประกอบการนี้ยังไม่มีท่าเรือ
                </p>
              ) : (
                <div className="flex flex-col gap-2">
                  {piers.map((p) => (
                    <label
                      key={p.pierId}
                      className="flex items-center gap-2 text-sm"
                    >
                      <input
                        type="checkbox"
                        className="size-4"
                        checked={selectedPierIds.includes(p.pierId ?? '')}
                        disabled={pending}
                        onChange={(e) =>
                          togglePier(p.pierId ?? '', e.target.checked)
                        }
                      />
                      {p.nameTh}
                    </label>
                  ))}
                </div>
              )}
              {operatorId && selectedPierIds.length === 0 && (
                <FieldError>เลือกท่าเรืออย่างน้อยหนึ่งแห่ง</FieldError>
              )}
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
