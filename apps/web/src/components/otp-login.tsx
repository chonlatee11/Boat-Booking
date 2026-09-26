'use client';

import { useEffect, useState } from 'react';
import { useTranslations } from 'next-intl';
import { useQueryClient } from '@tanstack/react-query';
import { useRouter } from '@/i18n/navigation';
import { apiFetch, type ApiError } from '@/lib/api';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Input } from '@/components/ui/input';
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSlot,
} from '@/components/ui/input-otp';
import { REGEXP_ONLY_DIGITS } from 'input-otp';
import { Spinner } from '@/components/ui/spinner';

const RESEND_COOLDOWN_SECONDS = 60;

export function OtpLogin() {
  const t = useTranslations('auth');
  const router = useRouter();
  const queryClient = useQueryClient();
  const [step, setStep] = useState<'destination' | 'code'>('destination');
  const [destination, setDestination] = useState('');
  const [code, setCode] = useState('');
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [cooldown, setCooldown] = useState(0);

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setInterval(
      () => setCooldown((c) => Math.max(0, c - 1)),
      1000,
    );
    return () => clearInterval(timer);
  }, [cooldown]);

  function otpErrorMessage(err: ApiError): string {
    if (err.status === 429) {
      return t('errors.rateLimited');
    }
    if (err.code === 'invalid_argument') {
      return t('errors.wrongCode', { n: err.attemptsLeft ?? 0 });
    }
    if (err.code === 'failed_precondition') {
      return t('errors.expired');
    }
    return t('errors.generic');
  }

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
      await queryClient.invalidateQueries({ queryKey: ['whoami'] });
      router.replace('/');
    } catch (err) {
      const apiErr = err as ApiError;
      setError(otpErrorMessage(apiErr));
      if (apiErr.code === 'failed_precondition') {
        setStep('destination');
      }
    } finally {
      setPending(false);
    }
  }

  if (step === 'destination') {
    return (
      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-2">
          <Label htmlFor="destination">{t('destinationLabel')}</Label>
          <Input
            id="destination"
            autoFocus
            disabled={pending}
            value={destination}
            onChange={(e) => setDestination(e.target.value)}
          />
        </div>
        {error && (
          <p role="alert" className="text-sm text-destructive">
            {error}
          </p>
        )}
        <Button
          className="h-11"
          disabled={destination.trim().length === 0 || pending}
          onClick={requestOtp}
        >
          {pending && <Spinner />}
          {t('send')}
        </Button>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm break-all text-muted-foreground">
        {t('codeSentTo', { destination })}
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
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      <Button
        className="h-11"
        disabled={code.length !== 6 || pending}
        onClick={verifyOtp}
      >
        {pending && <Spinner />}
        {t('verify')}
      </Button>
      <Button
        variant="link"
        disabled={cooldown > 0 || pending}
        onClick={requestOtp}
      >
        {cooldown > 0 ? t('resendIn', { n: cooldown }) : t('resend')}
      </Button>
    </div>
  );
}
