import { useTranslations } from 'next-intl';
import { BoatList } from '@/components/boat-list';

export default function Home() {
  const t = useTranslations();

  return (
    <main className="mx-auto max-w-screen-sm px-4 py-6">
      <h1 className="text-2xl font-bold">{t('home.title')}</h1>
      <p className="mt-2 text-black/70">{t('home.subtitle')}</p>

      <h2 className="mt-8 text-xl font-semibold">{t('boats.heading')}</h2>
      <div className="mt-4">
        <BoatList />
      </div>
    </main>
  );
}
