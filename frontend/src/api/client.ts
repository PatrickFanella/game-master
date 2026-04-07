const DEFAULT_API_BASE = '/api/v1';

const API_BASE = normalizeBase(import.meta.env.VITE_API_BASE ?? DEFAULT_API_BASE);

export class APIError extends Error {
  readonly status: number;
  readonly body: unknown;

  constructor(status: number, message: string, body: unknown) {
    super(message);
    this.name = 'APIError';
    this.status = status;
    this.body = body;
  }
}

type APIRequestInit = Omit<RequestInit, 'body'> & {
  body?: BodyInit | unknown;
};

function normalizeBase(base: string): string {
  return base.endsWith('/') ? base.slice(0, -1) : base;
}

function resolvePath(path: string): string {
  return path.startsWith('/') ? `${API_BASE}${path}` : `${API_BASE}/${path}`;
}

function isBodyInit(body: unknown): body is BodyInit {
  return (
    typeof body === 'string' ||
    body instanceof FormData ||
    body instanceof URLSearchParams ||
    body instanceof Blob ||
    body instanceof ArrayBuffer ||
    ArrayBuffer.isView(body) ||
    body instanceof ReadableStream
  );
}

export async function apiFetch<TResponse>(path: string, init: APIRequestInit = {}): Promise<TResponse> {
  const { body, headers, ...rest } = init;
  const requestHeaders = new Headers(headers);

  let requestBody: BodyInit | undefined;
  if (body !== undefined) {
    if (body === null || !isBodyInit(body)) {
      requestHeaders.set('Content-Type', 'application/json');
      requestBody = JSON.stringify(body);
    } else {
      requestBody = body;
    }
  }

  requestHeaders.set('Accept', 'application/json');

  const token = localStorage.getItem('gm_token');
  if (token) {
    requestHeaders.set('Authorization', `Bearer ${token}`);
  }

  const response = await fetch(resolvePath(path), {
    credentials: 'include',
    ...rest,
    headers: requestHeaders,
    body: requestBody,
  });

  const raw = await response.text();
  const parsed = raw ? (JSON.parse(raw) as unknown) : null;

  if (!response.ok) {
    const message =
      parsed && typeof parsed === 'object' && 'error' in parsed && typeof parsed.error === 'string'
        ? parsed.error
        : `Request failed with status ${response.status}`;

    throw new APIError(response.status, message, parsed);
  }

  return parsed as TResponse;
}

export async function apiFetchVoid(path: string, init: APIRequestInit = {}): Promise<void> {
  await apiFetch<null>(path, init);
}

export { API_BASE };
