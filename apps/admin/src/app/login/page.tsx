import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { OtpLogin } from '@/components/otp-login';

export default function LoginPage() {
  return (
    <div className="flex min-h-svh items-center justify-center p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>เข้าสู่ระบบผู้ดูแล</CardTitle>
        </CardHeader>
        <CardContent>
          <OtpLogin />
        </CardContent>
      </Card>
    </div>
  );
}
