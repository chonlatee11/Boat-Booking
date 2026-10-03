'use client';

import { useTranslations } from 'next-intl';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from '@/i18n/navigation';
import { apiFetch } from '@/lib/api';
import { Button } from '@/components/ui/button';

export function AccountStatus() {
  const t = useTranslations('auth');
  const queryClient = useQueryClient();

  const { isSuccess } = useQuery({
    queryKey: ['whoami'],
    queryFn: () => apiFetch('/api/v1/whoami'),
    retry: false,
  });

  if (!isSuccess) {
    return (
      <Button variant="outline" className="h-11" asChild>
        <Link href="/login">{t('signIn')}</Link>
      </Button>
    );
  }

  async function signOut() {
    await apiFetch('/api/v1/auth/logout', { method: 'POST' });
    await queryClient.invalidateQueries({ queryKey: ['whoami'] });
  }

  return (
    <div className="flex items-center gap-2">
      <span className="text-sm text-muted-foreground">{t('signedIn')}</span>
      <Button variant="outline" className="h-11" onClick={signOut}>
        {t('signOut')}
      </Button>
    </div>
  );
}
