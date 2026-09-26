'use client';

import { PencilIcon, PlusIcon } from 'lucide-react';
import { useWhoami } from '@/lib/queries';
import {
  useAdminRoutes,
  useOwnPiers,
  usePublicPiers,
  pierName,
} from './queries';
import type { RouteJson } from '@gen/services/catalog/v1/catalog_pb';
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
import { RouteSheet } from './route-sheet';

// Local baht formatter for Task 1 — replaced by money.ts's formatSatang once
// that file exists (Task 2 of this plan).
function formatBahtDisplay(satang: string): string {
  const n = BigInt(satang);
  const hundred = BigInt(100);
  const baht = n / hundred;
  const frac = n % hundred;
  return `฿${baht.toString()}.${frac.toString().padStart(2, '0')}`;
}

function currentPricesLabel(route: RouteJson): string {
  const prices = route.currentPrices ?? [];
  const adult = prices.find((p) => p.ticketType === 'TICKET_TYPE_ADULT');
  const child = prices.find((p) => p.ticketType === 'TICKET_TYPE_CHILD');
  const parts: string[] = [];
  if (adult?.amountSatang)
    parts.push(`ผู้ใหญ่ ${formatBahtDisplay(adult.amountSatang)}`);
  if (child?.amountSatang)
    parts.push(`เด็ก ${formatBahtDisplay(child.amountSatang)}`);
  return parts.length > 0 ? parts.join(' / ') : '—';
}

export default function RoutesPage() {
  const { data, isLoading, isError, refetch } = useAdminRoutes();
  const { data: ownPiersData } = useOwnPiers();
  const { data: publicPiersData } = usePublicPiers();
  const { data: whoami } = useWhoami();

  const routes = data?.routes ?? [];
  const canWrite =
    whoami?.role === 'super_admin' || whoami?.role === 'pier_admin';

  const name = (id?: string) =>
    pierName(id, ownPiersData?.piers, publicPiersData?.piers);

  const addTrigger = (
    <Button>
      <PlusIcon />
      สร้างเส้นทางใหม่
    </Button>
  );

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-2xl font-bold">เส้นทาง</h1>
        {canWrite && <RouteSheet trigger={addTrigger} />}
      </div>

      <DataTable
        columns={[
          {
            header: 'เส้นทาง',
            cell: (route: RouteJson) => (
              <TruncatedCell
                text={`${name(route.pierFromId)} → ${name(route.pierToId)}`}
              />
            ),
          },
          {
            header: 'ระยะเวลา (นาที)',
            cell: (route: RouteJson) => route.durationMinutes ?? '—',
          },
          {
            header: 'ราคาปัจจุบัน',
            cell: (route: RouteJson) => currentPricesLabel(route),
          },
          {
            header: 'สถานะ',
            cell: (route: RouteJson) =>
              route.archived ? (
                <Badge variant="secondary">เก็บถาวร</Badge>
              ) : (
                <Badge>ใช้งาน</Badge>
              ),
          },
          ...(canWrite
            ? [
                {
                  header: '',
                  className: 'w-24 text-right',
                  cell: (route: RouteJson) => (
                    <div className="flex justify-end gap-1">
                      <RouteSheet
                        route={route}
                        trigger={
                          <Button
                            variant="ghost"
                            size="icon"
                            className="size-11"
                            aria-label="แก้ไขเส้นทาง"
                          >
                            <PencilIcon className="size-4" />
                          </Button>
                        }
                      />
                    </div>
                  ),
                },
              ]
            : []),
        ]}
        rows={routes}
        isLoading={isLoading}
        isError={isError}
        onRetry={() => refetch()}
        errorText="โหลดข้อมูลเส้นทางไม่สำเร็จ กรุณาลองใหม่"
        rowKey={(route: RouteJson) => route.routeId ?? ''}
        isMuted={(route: RouteJson) => Boolean(route.archived)}
        empty={
          <Empty>
            <EmptyHeader>
              <EmptyTitle>ยังไม่มีเส้นทาง</EmptyTitle>
              <EmptyDescription>
                เริ่มต้นด้วยการเพิ่มเส้นทางแรกของคุณ
              </EmptyDescription>
            </EmptyHeader>
            {canWrite && (
              <EmptyContent>
                <RouteSheet trigger={addTrigger} />
              </EmptyContent>
            )}
          </Empty>
        }
      />
    </div>
  );
}
