'use client';

import type { ReactNode } from 'react';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { Skeleton } from '@/components/ui/skeleton';
import { Button } from '@/components/ui/button';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';

const SKELETON_ROW_COUNT = 5;

export type DataTableColumn<T> = {
  header: string;
  cell: (row: T) => ReactNode;
  className?: string;
};

export function DataTable<T>({
  columns,
  rows,
  isLoading,
  isError,
  onRetry,
  errorText,
  empty,
  rowKey,
  isMuted,
}: {
  columns: DataTableColumn<T>[];
  rows?: T[];
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  errorText: string;
  empty: ReactNode;
  rowKey: (row: T) => string;
  isMuted?: (row: T) => boolean;
}) {
  if (isError) {
    return (
      <div className="flex flex-col items-start gap-3 rounded-lg border p-6">
        <p className="text-sm text-muted-foreground">{errorText}</p>
        <Button variant="outline" onClick={onRetry}>
          ลองใหม่
        </Button>
      </div>
    );
  }

  if (!isLoading && (rows?.length ?? 0) === 0) {
    return <>{empty}</>;
  }

  return (
    <div className="overflow-x-auto rounded-lg border">
      <Table>
        <TableHeader>
          <TableRow>
            {columns.map((col) => (
              <TableHead key={col.header} className={col.className}>
                {col.header}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {isLoading
            ? Array.from({ length: SKELETON_ROW_COUNT }).map((_, i) => (
                <TableRow key={i}>
                  {columns.map((col, j) => (
                    <TableCell key={j} className={col.className}>
                      <Skeleton className="h-5 w-full" />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            : rows?.map((row) => (
                <TableRow
                  key={rowKey(row)}
                  className={cn(isMuted?.(row) && 'text-muted-foreground')}
                >
                  {columns.map((col, j) => (
                    <TableCell key={j} className={col.className}>
                      {col.cell(row)}
                    </TableCell>
                  ))}
                </TableRow>
              ))}
        </TableBody>
      </Table>
    </div>
  );
}

/** Truncates long cell text with a tooltip carrying the full value. */
export function TruncatedCell({ text }: { text: string }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <span className="block max-w-[220px] truncate">{text}</span>
      </TooltipTrigger>
      <TooltipContent>{text}</TooltipContent>
    </Tooltip>
  );
}
