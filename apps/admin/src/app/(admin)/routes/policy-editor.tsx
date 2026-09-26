'use client';

import { PlusIcon, TrashIcon } from 'lucide-react';
import type { CancellationTierJson } from '@gen/services/catalog/v1/catalog_pb';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Field, FieldLabel } from '@/components/ui/field';

export const POLICY_VALIDATION_ERROR =
  'นโยบายยกเลิกไม่ถูกต้อง: ต้องเรียงชั่วโมงจากมากไปน้อยและมีระดับ 0 ชั่วโมงเสมอ';

/**
 * Mirrors services/catalog/internal/domain/route.go's
 * ValidateCancellationPolicy (D-13): 1-10 tiers, strictly descending
 * min_hours_before (CancellationTierJson.minHoursBefore) >= 0, must include
 * a 0-hour tier, refund_percent (refundPercent) 0-100. Returns the UI-SPEC
 * error copy, or null when valid.
 */
export function validatePolicy(tiers: CancellationTierJson[]): string | null {
  if (tiers.length === 0 || tiers.length > 10) return POLICY_VALIDATION_ERROR;

  let sawZero = false;
  let prev = Number.POSITIVE_INFINITY;
  for (const tier of tiers) {
    const hours = tier.minHoursBefore ?? -1;
    const percent = tier.refundPercent ?? -1;
    if (hours < 0) return POLICY_VALIDATION_ERROR;
    if (hours >= prev) return POLICY_VALIDATION_ERROR;
    if (percent < 0 || percent > 100) return POLICY_VALIDATION_ERROR;
    if (hours === 0) sawZero = true;
    prev = hours;
  }
  if (!sawZero) return POLICY_VALIDATION_ERROR;
  return null;
}

export function PolicyEditor({
  tiers,
  onChange,
  disabled,
}: {
  tiers: CancellationTierJson[];
  onChange: (tiers: CancellationTierJson[]) => void;
  disabled?: boolean;
}) {
  const error = validatePolicy(tiers);

  function updateTier(index: number, patch: Partial<CancellationTierJson>) {
    onChange(tiers.map((t, i) => (i === index ? { ...t, ...patch } : t)));
  }

  function removeTier(index: number) {
    onChange(tiers.filter((_, i) => i !== index));
  }

  function addTier() {
    onChange([...tiers, { minHoursBefore: 0, refundPercent: 0 }]);
  }

  return (
    <div className="flex flex-col gap-2">
      {tiers.map((tier, index) => (
        <div key={index} className="flex items-end gap-2">
          <Field>
            <FieldLabel htmlFor={`policy-hours-${index}`}>
              ชั่วโมงก่อนออกเดินทางขั้นต่ำ
            </FieldLabel>
            <Input
              id={`policy-hours-${index}`}
              type="number"
              min={0}
              value={tier.minHoursBefore ?? 0}
              disabled={disabled}
              onChange={(e) =>
                updateTier(index, {
                  minHoursBefore: Number(e.target.value),
                })
              }
            />
          </Field>
          <Field>
            <FieldLabel htmlFor={`policy-percent-${index}`}>
              คืนเงิน %
            </FieldLabel>
            <Input
              id={`policy-percent-${index}`}
              type="number"
              min={0}
              max={100}
              value={tier.refundPercent ?? 0}
              disabled={disabled}
              onChange={(e) =>
                updateTier(index, {
                  refundPercent: Number(e.target.value),
                })
              }
            />
          </Field>
          {tiers.length > 1 && (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="size-11"
              aria-label="ลบระดับนโยบาย"
              disabled={disabled}
              onClick={() => removeTier(index)}
            >
              <TrashIcon className="size-4" />
            </Button>
          )}
        </div>
      ))}
      <Button
        type="button"
        variant="outline"
        size="sm"
        disabled={disabled}
        onClick={addTier}
      >
        <PlusIcon className="size-4" />
        เพิ่มระดับ
      </Button>
      {error && <p className="text-sm text-destructive">{error}</p>}
    </div>
  );
}
