'use client';

import { useLocale, useTranslations } from 'next-intl';
import { usePathname, useRouter } from '@/i18n/navigation';
import { Button } from '@/components/ui/button';

export function LocaleSwitcher() {
  const t = useTranslations('locale');
  const locale = useLocale();
  const pathname = usePathname();
  const router = useRouter();
  const nextLocale = locale === 'th' ? 'en' : 'th';

  return (
    <Button
      variant="outline"
      className="h-11"
      aria-label={t('switch')}
      onClick={() => router.replace(pathname, { locale: nextLocale })}
    >
      {nextLocale.toUpperCase()}
    </Button>
  );
}
