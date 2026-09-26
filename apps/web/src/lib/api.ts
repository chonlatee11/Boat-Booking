const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8000';

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(API_BASE_URL + path, {
    ...init,
    credentials: 'include',
    headers: { Accept: 'application/json', ...init?.headers },
  });

  if (!res.ok) {
    const error = new Error(`Request to ${path} failed with status ${res.status}`);
    (error as Error & { status: number }).status = res.status;
    throw error;
  }

  return (await res.json()) as T;
}
