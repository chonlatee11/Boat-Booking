'use client';

import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { rpc } from '@/lib/api';
import type {
  OperatorJson,
  UpsertOperatorResponseJson,
} from '@gen/services/catalog/v1/catalog_pb';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import { Field, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Spinner } from '@/components/ui/spinner';

const MAX_NAME_LENGTH = 100;

export function OperatorDialog({
  operator,
  trigger,
}: {
  operator?: OperatorJson;
  trigger: React.ReactNode;
}) {
  const isEdit = Boolean(operator?.operatorId);
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState(operator?.name ?? '');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  function handleOpenChange(next: boolean) {
    setOpen(next);
    if (next) {
      setName(operator?.name ?? '');
      setError(null);
    }
  }

  const trimmed = name.trim();
  const isValid = trimmed.length >= 1 && trimmed.length <= MAX_NAME_LENGTH;

  async function handleSubmit() {
    setPending(true);
    setError(null);
    try {
      await rpc<UpsertOperatorResponseJson>('catalog', 'UpsertOperator', {
        operatorId: operator?.operatorId ?? '',
        name: trimmed,
      });
      toast.success('บันทึกผู้ประกอบการสำเร็จ');
      await queryClient.invalidateQueries({ queryKey: ['operators'] });
      setOpen(false);
    } catch {
      setError('บันทึกผู้ประกอบการไม่สำเร็จ กรุณาลองใหม่');
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
            {isEdit ? 'แก้ไขผู้ประกอบการ' : 'เพิ่มผู้ประกอบการใหม่'}
          </DialogTitle>
        </DialogHeader>
        <Field>
          <FieldLabel htmlFor="operator-name">ชื่อผู้ประกอบการ</FieldLabel>
          <Input
            id="operator-name"
            value={name}
            maxLength={MAX_NAME_LENGTH}
            disabled={pending}
            autoFocus
            onChange={(e) => setName(e.target.value)}
          />
        </Field>
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
