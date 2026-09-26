'use client';

import { PencilIcon, PlusIcon } from 'lucide-react';
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

export default function StaffPage() {
  const { data: whoami } = useWhoami();
  const isSuperAdmin = whoami?.role === 'super_admin';

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
            className: 'w-24 text-right',
            cell: (u: StaffUserJson) => (
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
              </div>
            ),
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
