'use client';

import { PencilIcon, PlusIcon } from 'lucide-react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { rpc } from '@/lib/api';
import { useWhoami } from '@/lib/queries';
import { useBoats, useOwnPiers, pierName } from './queries';
import type { BoatJson } from '@gen/services/catalog/v1/catalog_pb';
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
import { BoatDialog } from './boat-dialog';

function StatusBadge({ boat }: { boat: BoatJson }) {
  if (boat.status === 'BOAT_STATUS_MAINTENANCE') {
    return (
      <Badge className="border-transparent bg-orange-100 text-orange-800">
        ซ่อมบำรุง
      </Badge>
    );
  }
  return (
    <Badge className="border-transparent bg-green-100 text-green-800">
      ใช้งาน
    </Badge>
  );
}

export default function BoatsPage() {
  const { data, isLoading, isError, refetch } = useBoats();
  const { data: ownPiersData } = useOwnPiers();
  const { data: whoami } = useWhoami();
  const queryClient = useQueryClient();

  const boats = data?.boats ?? [];
  const piers = ownPiersData?.piers ?? [];
  const canWrite =
    whoami?.role === 'super_admin' || whoami?.role === 'pier_admin';

  const addTrigger = (
    <Button>
      <PlusIcon />
      เพิ่มเรือใหม่
    </Button>
  );

  async function handleArchive(boat: BoatJson) {
    try {
      await rpc('catalog', 'ArchiveBoat', { boatId: boat.boatId });
      toast.success('เก็บถาวรเรือสำเร็จ');
      await queryClient.invalidateQueries({ queryKey: ['boats'] });
    } catch {
      toast.error('เก็บถาวรเรือไม่สำเร็จ กรุณาลองใหม่');
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-2xl font-bold">เรือ</h1>
        {canWrite && <BoatDialog trigger={addTrigger} />}
      </div>

      <DataTable
        columns={[
          {
            header: 'ชื่อ',
            cell: (boat: BoatJson) => <TruncatedCell text={boat.name ?? ''} />,
          },
          {
            header: 'ความจุ',
            cell: (boat: BoatJson) => boat.defaultCapacity ?? '—',
          },
          {
            header: 'ท่าประจำ',
            cell: (boat: BoatJson) => pierName(boat.homePierId, piers),
          },
          {
            header: 'สถานะ',
            cell: (boat: BoatJson) => <StatusBadge boat={boat} />,
          },
          ...(canWrite
            ? [
                {
                  header: '',
                  className: 'w-24 text-right',
                  cell: (boat: BoatJson) =>
                    boat.archived ? null : (
                      <div className="flex justify-end gap-1">
                        <BoatDialog
                          boat={boat}
                          boatsLoading={isLoading}
                          trigger={
                            <Button
                              variant="ghost"
                              size="icon"
                              className="size-11"
                              aria-label="แก้ไขเรือ"
                            >
                              <PencilIcon className="size-4" />
                            </Button>
                          }
                        />
                        <ArchiveDialog
                          entityLabel="เรือ"
                          name={boat.name ?? ''}
                          onConfirm={() => handleArchive(boat)}
                        />
                      </div>
                    ),
                },
              ]
            : []),
        ]}
        rows={boats}
        isLoading={isLoading}
        isError={isError}
        onRetry={() => refetch()}
        errorText="โหลดข้อมูลเรือไม่สำเร็จ กรุณาลองใหม่"
        rowKey={(boat: BoatJson) => boat.boatId ?? ''}
        isMuted={(boat: BoatJson) => Boolean(boat.archived)}
        empty={
          <Empty>
            <EmptyHeader>
              <EmptyTitle>ยังไม่มีเรือ</EmptyTitle>
              <EmptyDescription>
                เริ่มต้นด้วยการเพิ่มเรือแรกของคุณ
              </EmptyDescription>
            </EmptyHeader>
            {canWrite && (
              <EmptyContent>
                <BoatDialog trigger={addTrigger} />
              </EmptyContent>
            )}
          </Empty>
        }
      />
    </div>
  );
}
