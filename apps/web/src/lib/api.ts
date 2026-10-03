const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8000';

export type ApiError = Error & {
  status: number;
  code?: string;
  attemptsLeft?: number;
};

// One shared in-flight refresh so concurrent 401s don't each trigger their
// own POST /api/v1/auth/refresh (D-10 session renewal).
let refreshPromise: Promise<boolean> | null = null;

function refreshSession(): Promise<boolean> {
  if (!refreshPromise) {
    refreshPromise = fetch(API_BASE_URL + '/api/v1/auth/refresh', {
      method: 'POST',
      credentials: 'include',
    })
      .then((res) => res.ok)
      .catch(() => false)
      .finally(() => {
        refreshPromise = null;
      });
  }
  return refreshPromise;
}

async function parseError(res: Response): Promise<ApiError> {
  let body: { code?: string; message?: string; attemptsLeft?: number } = {};
  try {
    body = await res.json();
  } catch {
    // no JSON body — keep the default status-only error
  }
  const error = new Error(
    body.message ?? `Request failed with status ${res.status}`,
  ) as ApiError;
  error.status = res.status;
  error.code = body.code;
  error.attemptsLeft = body.attemptsLeft;
  return error;
}

export async function apiFetch<T>(
  path: string,
  init?: RequestInit,
): Promise<T> {
  const isAuthPath = path.startsWith('/api/v1/auth/');

  const doFetch = () =>
    fetch(API_BASE_URL + path, {
      ...init,
      credentials: 'include',
      headers: { Accept: 'application/json', ...init?.headers },
    });

  let res = await doFetch();

  if (res.status === 401 && !isAuthPath) {
    const refreshed = await refreshSession();
    if (refreshed) {
      res = await doFetch();
    }
  }

  if (!res.ok) {
    throw await parseError(res);
  }

  if (res.status === 204) {
    return undefined as T;
  }

  return (await res.json()) as T;
}
