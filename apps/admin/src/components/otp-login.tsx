'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { apiFetch, type ApiError } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Field, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSlot,
} from '@/components/ui/input-otp';
import { REGEXP_ONLY_DIGITS } from 'input-otp';
import { Spinner } from '@/components/ui/spinner';

const RESEND_COOLDOWN_SECONDS = 60;

function otpErrorMessage(err: ApiError): string {
  if (err.status === 429) {
    return 'ขอรหัสได้สูงสุด 5 ครั้งต่อชั่วโมง กรุณาลองใหม่ภายหลัง';
  }
  if (err.code === 'invalid_argument') {
    const left = err.attemptsLeft ?? 0;
    return `รหัสไม่ถูกต้อง กรุณาลองใหม่ (เหลือ ${left} ครั้ง)`;
  }
  if (err.code === 'failed_precondition') {
    return 'รหัสหมดอายุหรือถูกล็อก กรุณาขอรหัสใหม่';
  }
  return 'เกิดข้อผิดพลาด กรุณาลองใหม่';
}

export function OtpLogin() {
  const router = useRouter();
  const [step, setStep] = useState<'destination' | 'code'>('destination');
  const [destination, setDestination] = useState('');
  const [code, setCode] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [cooldown, setCooldown] = useState(0);

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setInterval(() => setCooldown((c) => Math.max(0, c - 1)), 1000);
    return () => clearInterval(timer);
  }, [cooldown]);

  async function requestOtp() {
    setPending(true);
    setError(null);
    try {
      await apiFetch('/api/v1/auth/otp/request', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ destination }),
      });
      setStep('code');
      setCode('');
      setCooldown(RESEND_COOLDOWN_SECONDS);
    } catch (err) {
      setError(otpErrorMessage(err as ApiError));
    } finally {
      setPending(false);
    }
  }

  async function verifyOtp() {
    setPending(true);
    setError(null);
    try {
      await apiFetch('/api/v1/auth/otp/verify', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ destination, code }),
      });
      router.replace('/');
    } catch (err) {
      setError(otpErrorMessage(err as ApiError));
    } finally {
      setPending(false);
    }
  }

  if (step === 'destination') {
    return (
      <div className="flex flex-col gap-4">
        <Field>
          <FieldLabel htmlFor="destination">อีเมลหรือเบอร์โทรศัพท์</FieldLabel>
          <Input
            id="destination"
            autoFocus
            disabled={pending}
            value={destination}
            onChange={(e) => setDestination(e.target.value)}
          />
        </Field>
        {error && <p className="text-sm text-destructive">{error}</p>}
        <Button
          className="h-11"
          disabled={destination.trim().length === 0 || pending}
          onClick={requestOtp}
        >
          {pending && <Spinner />}
          ส่งรหัส OTP
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm break-all text-muted-foreground">
        รหัสถูกส่งไปที่ {destination}
      </p>
      <InputOTP
        maxLength={6}
        pattern={REGEXP_ONLY_DIGITS}
        value={code}
        onChange={setCode}
        disabled={pending}
      >
        <InputOTPGroup>
          <InputOTPSlot index={0} />
          <InputOTPSlot index={1} />
          <InputOTPSlot index={2} />
          <InputOTPSlot index={3} />
          <InputOTPSlot index={4} />
          <InputOTPSlot index={5} />
        </InputOTPGroup>
      </InputOTP>
      {error && <p className="text-sm text-destructive">{error}</p>}
      <Button
        className="h-11"
        disabled={code.length !== 6 || pending}
        onClick={verifyOtp}
      >
        {pending && <Spinner />}
        ยืนยันรหัส
      </Button>
      <Button
        variant="link"
        disabled={cooldown > 0 || pending}
        onClick={requestOtp}
      >
        {cooldown > 0
          ? `ส่งรหัสอีกครั้งได้ในอีก ${cooldown} วินาที`
          : 'ส่งรหัส OTP อีกครั้ง'}
      </Button>
    </div>
  );
}
