import { test } from 'node:test';
import assert from 'node:assert/strict';

// See apps/admin/src/app/(admin)/routes/money.test.ts for why this import
// is a typed dynamic import rather than a static one (node --test needs the
// literal .ts extension; tsc's bundler resolution rejects that extension).
const apiPath = './api.ts';
const { apiFetch }: typeof import('./api') = await import(apiPath);

function fakeResponse(status: number, body: unknown): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response;
}

test('apiFetch retries once via POST /api/v1/auth/refresh after a 401, then succeeds', async () => {
  const calls: { url: string; method?: string; credentials?: string }[] = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (
    url: string,
    init?: RequestInit,
  ): Promise<Response> => {
    calls.push({
      url,
      method: init?.method,
      credentials: init?.credentials as string,
    });
    if (calls.length === 1) return fakeResponse(401, { code: 'unauthorized' });
    if (calls.length === 2) return fakeResponse(200, {});
    return fakeResponse(200, { ok: true });
  }) as typeof fetch;

  try {
    const result = await apiFetch<{ ok: boolean }>('/api/v1/departures');
    assert.deepEqual(result, { ok: true });
    assert.equal(calls.length, 3);
    assert.equal(calls[0].url, 'http://localhost:8000/api/v1/departures');
    assert.equal(calls[1].url, 'http://localhost:8000/api/v1/auth/refresh');
    assert.equal(calls[1].method, 'POST');
    assert.equal(calls[1].credentials, 'include');
    assert.equal(calls[2].url, 'http://localhost:8000/api/v1/departures');
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test('apiFetch does not retry when refresh itself fails, and surfaces the original 401', async () => {
  const calls: string[] = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (url: string): Promise<Response> => {
    calls.push(url);
    if (calls.length === 1) return fakeResponse(401, { code: 'unauthorized' });
    return fakeResponse(401, { code: 'refresh_failed' });
  }) as typeof fetch;

  try {
    await assert.rejects(
      () => apiFetch('/api/v1/departures'),
      (err: unknown) => {
        assert.equal((err as { status: number }).status, 401);
        return true;
      },
    );
    assert.equal(calls.length, 2, 'should call the API once and refresh once, but not retry the API call');
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test('apiFetch does not attempt refresh for 401s on auth paths themselves', async () => {
  const calls: string[] = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (async (url: string): Promise<Response> => {
    calls.push(url);
    return fakeResponse(401, { code: 'bad_credentials' });
  }) as typeof fetch;

  try {
    await assert.rejects(() => apiFetch('/api/v1/auth/login'));
    assert.equal(calls.length, 1, 'auth paths must not trigger a refresh-and-retry loop');
  } finally {
    globalThis.fetch = originalFetch;
  }
});
