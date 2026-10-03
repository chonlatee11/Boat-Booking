'use client';

import { useState } from 'react';
import { PencilIcon, PlusIcon } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { rpc, type ApiError } from '@/lib/api';
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
import { ArchiveDialog } from '@/components/archive-dialog';
import { PierSheet } from './pier-sheet';

const ACTIVE_ROUTES_PREFIX = 'active routes: ';

function hoursLabel(pier: PierJson): string {
  if (!pier.opensAt || !pier.closesAt) return '-';
  return `${pier.opensAt}–${pier.closesAt}`;
}

// D-15: ArchivePier's failed_precondition message is
// "failed precondition: active routes: {list}" — extract just the list.
function extractRouteList(message: string): string {
  const idx = message.indexOf(ACTIVE_ROUTES_PREFIX);
  return idx === -1
    ? message
    : message.slice(idx + ACTIVE_ROUTES_PREFIX.length);
}

export default function PiersPage() {
  const { data, isLoading, isError, refetch } = useAdminPiers();
  const { data: whoami } = useWhoami();
  const queryClient = useQueryClient();
  const [archiveBlockedError, setArchiveBlockedError] = useState<string | null>(
    null,
  );

  const piers = data?.piers ?? [];
  const canCreate = whoami?.role === 'super_admin';
  const canWrite =
    whoami?.role === 'super_admin' || whoami?.role === 'pier_admin';

  const addTrigger = (
    <Button>
      <PlusIcon />
      เพิ่มท่าเรือใหม่
    </Button>
  );

  async function handleArchive(pier: PierJson) {
    setArchiveBlockedError(null);
    try {
      await rpc('catalog', 'ArchivePier', { pierId: pier.pierId });
      toast.success('เก็บถาวรท่าเรือสำเร็จ');
      await queryClient.invalidateQueries({ queryKey: ['piers'] });
    } catch (err) {
      const apiErr = err as ApiError;
      if (apiErr.code === 'failed_precondition') {
        setArchiveBlockedError(
          `ไม่สามารถเก็บถาวรได้ เนื่องจากยังมีเส้นทางใช้งานอยู่: ${extractRouteList(apiErr.message)}`,
        );
      } else {
        toast.error('เก็บถาวรท่าเรือไม่สำเร็จ กรุณาลองใหม่');
      }
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-2xl font-bold">ท่าเรือ</h1>
        {canCreate && <PierSheet trigger={addTrigger} />}
      </div>

      {archiveBlockedError && (
        <p className="text-sm text-destructive">{archiveBlockedError}</p>
      )}

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
                <Badge className="border-transparent bg-green-100 text-green-800">
                  ใช้งาน
                </Badge>
              ),
          },
          ...(canWrite
            ? [
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
                        <ArchiveDialog
                          entityLabel="ท่าเรือ"
                          name={pier.nameTh ?? ''}
                          onConfirm={() => handleArchive(pier)}
                        />
                      </div>
                    ),
                },
              ]
            : []),
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
