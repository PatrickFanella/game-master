const API_BASE = import.meta.env.VITE_API_BASE ?? '/api/v1';

export interface AuthUser {
  readonly id: string;
  readonly name: string;
  readonly email: string;
}

export interface AuthResponse {
  readonly token: string;
  readonly user: AuthUser;
}

export interface AuthError {
  readonly error: string;
}

async function authFetch<T>(path: string, body: Record<string, string>): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
    body: JSON.stringify(body),
  });

  const data = await res.json();

  if (!res.ok) {
    throw new Error((data as AuthError).error ?? `Request failed (${res.status})`);
  }

  return data as T;
}

export function register(name: string, email: string, password: string): Promise<AuthResponse> {
  return authFetch<AuthResponse>('/auth/register', { name, email, password });
}

export function login(email: string, password: string): Promise<AuthResponse> {
  return authFetch<AuthResponse>('/auth/login', { email, password });
}

export async function getMe(token: string): Promise<{ user: AuthUser }> {
  const res = await fetch(`${API_BASE}/auth/me`, {
    credentials: 'include',
    headers: { Authorization: `Bearer ${token}`, Accept: 'application/json' },
  });

  if (!res.ok) {
    throw new Error('Token expired or invalid');
  }

  return res.json() as Promise<{ user: AuthUser }>;
}

export async function logout(): Promise<void> {
  await fetch(`${API_BASE}/auth/logout`, {
    method: 'POST',
    credentials: 'include',
  });
}
