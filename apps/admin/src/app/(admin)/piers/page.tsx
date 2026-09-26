'use client';

import { PencilIcon, PlusIcon } from 'lucide-react';
import { useWhoami } from '@/lib/queries';
import { useAdminPiers } from './queries';
import type { PierJson } from '@gen/services/catalog/v1/catalog_pb';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import {
  Empty,
  EmptyHeader,
  EmptyTitle,
  EmptyDescription,
  EmptyContent,
} from '@/components/ui/empty';
import { DataTable, TruncatedCell } from '@/components/data-table';
import { PierSheet } from './pier-sheet';

function hoursLabel(pier: PierJson): string {
  if (!pier.opensAt || !pier.closesAt) return '-';
  return `${pier.opensAt}–${pier.closesAt}`;
}

export default function PiersPage() {
  const { data, isLoading, isError, refetch } = useAdminPiers();
  const { data: whoami } = useWhoami();

  const piers = data?.piers ?? [];
  const canCreate = whoami?.role === 'super_admin';

  const addTrigger = (
    <Button>
      <PlusIcon />
      เพิ่มท่าเรือใหม่
    </Button>
  );

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-2xl font-bold">ท่าเรือ</h1>
        {canCreate && <PierSheet trigger={addTrigger} />}
      </div>

      <DataTable
        columns={[
          {
            header: 'ชื่อ',
            cell: (pier: PierJson) => (
              <TruncatedCell
                text={`${pier.nameTh ?? ''} / ${pier.nameEn ?? ''}`}
              />
            ),
          },
          {
            header: 'ที่อยู่',
            cell: (pier: PierJson) => (
              <TruncatedCell text={pier.address ?? ''} />
            ),
          },
          {
            header: 'เวลาเปิด–ปิด',
            cell: (pier: PierJson) => hoursLabel(pier),
          },
          {
            header: 'สถานะ',
            cell: (pier: PierJson) =>
              pier.archived ? (
                <Badge variant="secondary">เก็บถาวร</Badge>
              ) : (
                <Badge>ใช้งาน</Badge>
              ),
          },
          {
            header: '',
            className: 'w-24 text-right',
            cell: (pier: PierJson) =>
              pier.archived ? null : (
                <div className="flex justify-end gap-1">
                  <PierSheet
                    pier={pier}
                    piersLoading={isLoading}
                    trigger={
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-11"
                        aria-label="แก้ไขท่าเรือ"
                      >
                        <PencilIcon className="size-4" />
                      </Button>
                    }
                  />
                </div>
              ),
          },
        ]}
        rows={piers}
        isLoading={isLoading}
        isError={isError}
        onRetry={() => refetch()}
        errorText="โหลดข้อมูลท่าเรือไม่สำเร็จ กรุณาลองใหม่"
        rowKey={(pier: PierJson) => pier.pierId ?? ''}
        isMuted={(pier: PierJson) => Boolean(pier.archived)}
        empty={
          <Empty>
            <EmptyHeader>
              <EmptyTitle>ยังไม่มีท่าเรือ</EmptyTitle>
              <EmptyDescription>
                เริ่มต้นด้วยการเพิ่มท่าเรือแรกของคุณ
              </EmptyDescription>
            </EmptyHeader>
            {canCreate && (
              <EmptyContent>
                <PierSheet trigger={addTrigger} />
              </EmptyContent>
            )}
          </Empty>
        }
      />
    </div>
  );
}
