'use client';

import { useOperators } from '@/lib/queries';
import { Badge } from '@/components/ui/badge';

export default function OperatorsPage() {
  const { data, isLoading, isError } = useOperators();
  const operators = data?.operators ?? [];

  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-2xl font-bold">ผู้ประกอบการ</h1>
      {isLoading && <p>กำลังโหลด...</p>}
      {isError && <p>โหลดข้อมูลผู้ประกอบการไม่สำเร็จ กรุณาลองใหม่</p>}
      <ul className="flex flex-col gap-2">
        {operators.map((op) => (
          <li
            key={op.operatorId}
            className="flex items-center gap-2 rounded-lg border p-3"
          >
            <span>{op.name}</span>
            {op.archived && <Badge variant="secondary">เก็บถาวร</Badge>}
          </li>
        ))}
      </ul>
    </div>
  );
}
