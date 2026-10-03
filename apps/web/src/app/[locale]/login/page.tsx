import { useTranslations } from 'next-intl';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { OtpLogin } from '@/components/otp-login';

export default function LoginPage() {
  const t = useTranslations('auth');

  return (
    <main className="mx-auto flex min-h-svh max-w-screen-sm items-center justify-center p-4">
      <Card className="w-full">
        <CardHeader>
          <CardTitle>{t('title')}</CardTitle>
        </CardHeader>
        <CardContent>
          <OtpLogin />
        </CardContent>
      </Card>
    </main>
  );
}
