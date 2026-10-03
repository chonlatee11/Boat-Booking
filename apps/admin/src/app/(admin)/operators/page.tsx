'use client';

import { useState } from 'react';
import { PencilIcon, PlusIcon } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { useOperators } from '@/lib/queries';
import { rpc, type ApiError } from '@/lib/api';
import type { OperatorJson } from '@gen/services/catalog/v1/catalog_pb';
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
import { OperatorDialog } from '@/components/operator-dialog';
import { ArchiveDialog } from '@/components/archive-dialog';
import { useWhoami } from '@/lib/queries';

export default function OperatorsPage() {
  const { data, isLoading, isError, refetch } = useOperators();
  const { data: whoami } = useWhoami();
  const queryClient = useQueryClient();
  const [archiveBlockedError, setArchiveBlockedError] = useState<string | null>(
    null,
  );

  const operators = data?.operators ?? [];
  const canWrite = whoami?.role === 'super_admin';

  const addTrigger = (
    <Button>
      <PlusIcon />
      เพิ่มผู้ประกอบการใหม่
    </Button>
  );

  async function handleArchive(operator: OperatorJson) {
    setArchiveBlockedError(null);
    try {
      await rpc('catalog', 'ArchiveOperator', {
        operatorId: operator.operatorId,
      });
      await queryClient.invalidateQueries({ queryKey: ['operators'] });
    } catch (err) {
      const apiErr = err as ApiError;
      if (apiErr.code === 'failed_precondition') {
        setArchiveBlockedError(
          'ไม่สามารถเก็บถาวรได้ เนื่องจากยังมีท่าเรือใช้งานอยู่',
        );
      } else {
        toast.error('เก็บถาวรผู้ประกอบการไม่สำเร็จ กรุณาลองใหม่');
      }
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-2xl font-bold">ผู้ประกอบการ</h1>
        {canWrite && <OperatorDialog trigger={addTrigger} />}
      </div>

      {archiveBlockedError && (
        <p className="text-sm text-destructive">{archiveBlockedError}</p>
      )}

      <DataTable
        columns={[
          {
            header: 'ชื่อ',
            cell: (op: OperatorJson) => <TruncatedCell text={op.name ?? ''} />,
          },
          {
            header: 'สถานะ',
            cell: (op: OperatorJson) =>
              op.archived ? (
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
                  cell: (op: OperatorJson) => (
                    <div className="flex justify-end gap-1">
                      <OperatorDialog
                        operator={op}
                        trigger={
                          <Button
                            variant="ghost"
                            size="icon"
                            className="size-11"
                            aria-label="แก้ไขผู้ประกอบการ"
                          >
                            <PencilIcon className="size-4" />
                          </Button>
                        }
                      />
                      <ArchiveDialog
                        entityLabel="ผู้ประกอบการ"
                        name={op.name ?? ''}
                        onConfirm={() => handleArchive(op)}
                      />
                    </div>
                  ),
                },
              ]
            : []),
        ]}
        rows={operators}
        isLoading={isLoading}
        isError={isError}
        onRetry={() => refetch()}
        errorText="โหลดข้อมูลผู้ประกอบการไม่สำเร็จ กรุณาลองใหม่"
        rowKey={(op: OperatorJson) => op.operatorId ?? ''}
        isMuted={(op: OperatorJson) => Boolean(op.archived)}
        empty={
          <Empty>
            <EmptyHeader>
              <EmptyTitle>ยังไม่มีผู้ประกอบการ</EmptyTitle>
              <EmptyDescription>
                เริ่มต้นด้วยการเพิ่มผู้ประกอบการแรกของคุณ
              </EmptyDescription>
            </EmptyHeader>
            {canWrite && (
              <EmptyContent>
                <OperatorDialog trigger={addTrigger} />
              </EmptyContent>
            )}
          </Empty>
        }
      />
    </div>
  );
}
