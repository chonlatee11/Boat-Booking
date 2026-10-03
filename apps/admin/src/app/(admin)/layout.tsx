'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useWhoami } from '@/lib/queries';
import { apiFetch, type ApiError } from '@/lib/api';
import { AdminShell } from '@/components/admin-shell';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

export default function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const { data, isLoading, isError, error } = useWhoami();
  const unauthenticated = isError && (error as ApiError).status === 401;

  useEffect(() => {
    if (unauthenticated) {
      router.replace('/login');
    }
  }, [unauthenticated, router]);

  if (isLoading || unauthenticated) {
    return (
      <div className="flex min-h-svh flex-col gap-4 p-6">
        <Skeleton className="h-8 w-48" />
        <Skeleton className="h-40 w-full" />
      </div>
    );
  }

  if (isError || !data) {
    return null;
  }

  if (data.role === 'customer') {
    return (
      <div className="flex min-h-svh items-center justify-center p-4">
        <Card className="w-full max-w-sm">
          <CardHeader>
            <CardTitle>บัญชีนี้ไม่มีสิทธิ์เข้าใช้งานระบบผู้ดูแล</CardTitle>
          </CardHeader>
          <CardContent>
            <Button
              className="w-full"
              onClick={async () => {
                await apiFetch('/api/v1/auth/logout', {
                  method: 'POST',
                }).catch(() => {});
                router.replace('/login');
              }}
            >
              ออกจากระบบ
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  return <AdminShell role={data.role}>{children}</AdminShell>;
}
