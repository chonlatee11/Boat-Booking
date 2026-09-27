'use client';

import { useState } from 'react';
import { BanIcon, CheckIcon, PencilIcon, PlusIcon } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { rpc, type ApiError } from '@/lib/api';
import { useWhoami } from '@/lib/queries';
import {
  useStaffUsers,
  useOperatorOptions,
  useAllPiers,
  operatorName,
  pierName,
} from './queries';
import type { StaffUserJson } from '@gen/services/identity/v1/users_pb';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Card, CardHeader, CardTitle } from '@/components/ui/card';
import { Spinner } from '@/components/ui/spinner';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import {
  Empty,
  EmptyHeader,
  EmptyTitle,
  EmptyDescription,
  EmptyContent,
} from '@/components/ui/empty';
import { DataTable, TruncatedCell } from '@/components/data-table';
import { StaffDialog } from './staff-dialog';

const ROLE_LABELS: Record<string, string> = {
  pier_admin: 'ผู้ดูแลท่าเรือ',
  staff: 'พนักงาน',
  super_admin: 'ผู้ดูแลระบบสูงสุด',
};

/**
 * Destructive disable confirmation (D-10) — mirrors ArchiveDialog's shape
 * but with staff-specific copy, so it stays local to this page rather than
 * generalizing ArchiveDialog for a one-off wording difference.
 */
function DisableUserDialog({
  user,
  onConfirm,
}: {
  user: StaffUserJson;
  onConfirm: () => Promise<void> | void;
}) {
  const [open, setOpen] = useState(false);
  const [pending, setPending] = useState(false);

  async function handleConfirm() {
    setPending(true);
    await onConfirm();
    setPending(false);
    setOpen(false);
  }

  return (
    <AlertDialog open={open} onOpenChange={setOpen}>
      <AlertDialogTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          className="size-11"
          aria-label="ปิดการใช้งานผู้ใช้"
        >
          <BanIcon className="size-4" />
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            ปิดการใช้งานผู้ใช้ &apos;{user.name}&apos;?
          </AlertDialogTitle>
          <AlertDialogDescription>
            จะไม่สามารถเข้าสู่ระบบได้อีก (มีผลภายใน 15 นาที)
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={pending}>ยกเลิก</AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={pending}
            onClick={(e) => {
              e.preventDefault();
              void handleConfirm();
            }}
          >
            ปิดการใช้งาน
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

function EnableUserButton({
  onConfirm,
}: {
  onConfirm: () => Promise<void> | void;
}) {
  const [pending, setPending] = useState(false);

  async function handleClick() {
    setPending(true);
    await onConfirm();
    setPending(false);
  }

  return (
    <Button
      variant="outline"
      size="sm"
      disabled={pending}
      onClick={handleClick}
    >
      {pending ? <Spinner /> : <CheckIcon className="size-4" />}
      เปิดการใช้งาน
    </Button>
  );
}

export default function StaffPage() {
  const { data: whoami } = useWhoami();
  const isSuperAdmin = whoami?.role === 'super_admin';
  const queryClient = useQueryClient();

  // Only super_admin ever calls these — UserService itself enforces the
  // same rule server-side (this gate is cosmetic, T-02-13-01).
  const { data, isLoading, isError, refetch } = useStaffUsers(isSuperAdmin);
  const { data: operatorsData } = useOperatorOptions(isSuperAdmin);
  const { data: piersData } = useAllPiers(isSuperAdmin);

  if (!isSuperAdmin) {
    return (
      <Card className="max-w-sm">
        <CardHeader>
          <CardTitle>หน้านี้สำหรับผู้ดูแลระบบสูงสุดเท่านั้น</CardTitle>
        </CardHeader>
      </Card>
    );
  }

  const users = data?.users ?? [];
  const operators = operatorsData?.operators ?? [];
  const piers = piersData?.piers ?? [];

  const addTrigger = (
    <Button>
      <PlusIcon />
      เพิ่มผู้ใช้งานใหม่
    </Button>
  );

  async function handleSetDisabled(user: StaffUserJson, disabled: boolean) {
    try {
      await rpc('users', 'SetUserDisabled', { userId: user.userId, disabled });
      await queryClient.invalidateQueries({ queryKey: ['staff-users'] });
    } catch (err) {
      const apiErr = err as ApiError;
      // IN-09: re-enabling a user hit the disable-failure text for both
      // directions -- pick the message from the direction actually
      // requested.
      const fallback = disabled
        ? 'ปิดการใช้งานผู้ใช้ไม่สำเร็จ กรุณาลองใหม่'
        : 'เปิดการใช้งานผู้ใช้ไม่สำเร็จ กรุณาลองใหม่';
      toast.error(apiErr.message || fallback);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-2xl font-bold">ผู้ใช้งาน</h1>
        <StaffDialog trigger={addTrigger} />
      </div>

      <DataTable
        columns={[
          {
            header: 'อีเมล',
            cell: (u: StaffUserJson) => <TruncatedCell text={u.email ?? ''} />,
          },
          { header: 'ชื่อ', cell: (u: StaffUserJson) => u.name ?? '' },
          {
            header: 'บทบาท',
            cell: (u: StaffUserJson) => ROLE_LABELS[u.role ?? ''] ?? u.role,
          },
          {
            header: 'ผู้ประกอบการ',
            cell: (u: StaffUserJson) => operatorName(u.operatorId, operators),
          },
          {
            header: 'ท่าเรือ',
            cell: (u: StaffUserJson) => (
              <div className="flex flex-wrap gap-1">
                {(u.pierIds ?? []).map((id) => (
                  <Badge key={id} variant="secondary">
                    {pierName(id, piers)}
                  </Badge>
                ))}
              </div>
            ),
          },
          {
            header: 'สถานะ',
            cell: (u: StaffUserJson) =>
              u.disabled ? (
                <Badge variant="secondary">ปิดการใช้งาน</Badge>
              ) : (
                <Badge>ใช้งาน</Badge>
              ),
          },
          {
            header: '',
            className: 'w-40 text-right',
            cell: (u: StaffUserJson) => {
              // super_admin rows: UserService rejects both update and
              // disable for role=super_admin (02-07), so no action is ever
              // usable here — showing edit/disable would only ever fail.
              if (u.role === 'super_admin') {
                return null;
              }
              const isSelf = u.userId === whoami?.sub;
              return (
                <div className="flex justify-end gap-1">
                  <StaffDialog
                    user={u}
                    usersLoading={isLoading}
                    trigger={
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-11"
                        aria-label="แก้ไขผู้ใช้งาน"
                      >
                        <PencilIcon className="size-4" />
                      </Button>
                    }
                  />
                  {!isSelf &&
                    (u.disabled ? (
                      <EnableUserButton
                        onConfirm={() => handleSetDisabled(u, false)}
                      />
                    ) : (
                      <DisableUserDialog
                        user={u}
                        onConfirm={() => handleSetDisabled(u, true)}
                      />
                    ))}
                </div>
              );
            },
          },
        ]}
        rows={users}
        isLoading={isLoading}
        isError={isError}
        onRetry={() => refetch()}
        errorText="โหลดข้อมูลผู้ใช้งานไม่สำเร็จ กรุณาลองใหม่"
        rowKey={(u: StaffUserJson) => u.userId ?? ''}
        isMuted={(u: StaffUserJson) => Boolean(u.disabled)}
        empty={
          <Empty>
            <EmptyHeader>
              <EmptyTitle>ยังไม่มีผู้ใช้งาน</EmptyTitle>
              <EmptyDescription>
                เริ่มต้นด้วยการเพิ่มผู้ใช้งานแรกของคุณ
              </EmptyDescription>
            </EmptyHeader>
            <EmptyContent>
              <StaffDialog trigger={addTrigger} />
            </EmptyContent>
          </Empty>
        }
      />
    </div>
  );
}
