'use client';

import { useState } from 'react';
import { ArchiveIcon } from 'lucide-react';
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
import { Button } from '@/components/ui/button';

/**
 * Reusable archive confirmation for every entity list (operators, piers,
 * routes, boats). The trigger is a 44x44px icon-only button with a Thai
 * aria-label naming the action (UI-SPEC touch-target contract).
 */
export function ArchiveDialog({
  entityLabel,
  name,
  onConfirm,
}: {
  entityLabel: string;
  name: string;
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
          aria-label={`เก็บถาวร${entityLabel}`}
        >
          <ArchiveIcon className="size-4" />
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            เก็บถาวร{entityLabel} &apos;{name}&apos;?
          </AlertDialogTitle>
          <AlertDialogDescription>
            จะไม่แสดงอีกต่อไปแต่ข้อมูลยังถูกเก็บไว้
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
            เก็บถาวร
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
