'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useWhoami } from '@/lib/queries';

export default function AdminHomePage() {
  const router = useRouter();
  const { data } = useWhoami();

  useEffect(() => {
    if (!data) return;
    router.replace(data.role === 'super_admin' ? '/operators' : '/piers');
  }, [data, router]);

  return null;
}
