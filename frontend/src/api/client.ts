import type { paths } from './openapi';

export function csrfToken(): string {
  const m = document.cookie.match(/(?:^|;\s*)nm_csrf=([^;]*)/);
  return m ? decodeURIComponent(m[1]) : '';
}

export function authHeaders(extra?: HeadersInit): Record<string, string> {
  const h: Record<string, string> = {};
  if (extra) {
    const e = new Headers(extra);
    e.forEach((v, k) => {
      h[k] = v;
    });
  }
  const token = window.NM_CONFIG?.apiAuthToken || '';
  if (token) h.Authorization = `Bearer ${token}`;
  const csrf = csrfToken();
  if (csrf) h['X-CSRF-Token'] = csrf;
  return h;
}

export class ApiError extends Error {
  status: number;
  body: unknown;

  constructor(status: number, message: string, body?: unknown) {
    super(message);
    this.status = status;
    this.body = body;
  }
}

/** True for fetch/AbortController aborts (DOMException or Error named AbortError). */
export function isAbortError(e: unknown): boolean {
  if (e == null || typeof e !== 'object') return false;
  const name = (e as { name?: unknown }).name;
  return name === 'AbortError';
}

export const SESSION_EXPIRED_EVENT = 'nm-session-expired';

/** Auth endpoints where 401 means "bad credentials" / "not logged in", not an expired SPA session. */
export function isAuthApiPath(path: string): boolean {
  try {
    const p = path.startsWith('http') ? new URL(path).pathname : path.split('?')[0];
    return p === '/api/auth' || p.startsWith('/api/auth/');
  } catch {
    return path.includes('/api/auth/');
  }
}

export function notifySessionExpired(path: string, status: number): void {
  if (status !== 401 || isAuthApiPath(path)) return;
  if (typeof window === 'undefined') return;
  if (window.location.pathname === '/login') return;
  window.dispatchEvent(new Event(SESSION_EXPIRED_EVENT));
}

export async function apiFetch<T = unknown>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const headers = authHeaders(init.headers);
  if (init.body && !(init.body instanceof FormData) && !headers['Content-Type']) {
    headers['Content-Type'] = 'application/json';
  }
  const res = await fetch(path, {
    ...init,
    credentials: 'same-origin',
    headers,
  });
  if (res.status === 204) return undefined as T;
  const ct = res.headers.get('content-type') || '';
  const data = ct.includes('application/json')
    ? await res.json().catch(() => ({}))
    : await res.text();
  if (!res.ok) {
    notifySessionExpired(path, res.status);
    const msg =
      typeof data === 'object' && data && 'error' in data
        ? String((data as { error: unknown }).error)
        : `HTTP ${res.status}`;
    throw new ApiError(res.status, msg, data);
  }
  return data as T;
}

export async function apiFetchRaw(path: string, init: RequestInit = {}): Promise<Response> {
  const headers = authHeaders(init.headers);
  const res = await fetch(path, {
    ...init,
    credentials: 'same-origin',
    headers,
  });
  notifySessionExpired(path, res.status);
  return res;
}

/** JSON body of HTTP 200 for an OpenAPI path+method, if the spec declares application/json. */
export type ApiJson<P extends keyof paths, M extends keyof paths[P]> = paths[P][M] extends {
  responses: { 200: { content: { 'application/json': infer R } } };
}
  ? R
  : never;

export function apiGet<P extends keyof paths & string>(
  path: P,
  init?: RequestInit,
): Promise<ApiJson<P, 'get'>> {
  return apiFetch(path, init) as Promise<ApiJson<P, 'get'>>;
}

export function apiPost<P extends keyof paths & string>(
  path: P,
  body?: unknown,
  init?: RequestInit,
): Promise<ApiJson<P, 'post'>> {
  return apiFetch(path, {
    ...init,
    method: 'POST',
    body: body === undefined ? init?.body : JSON.stringify(body),
  }) as Promise<ApiJson<P, 'post'>>;
}

export type ApiQueryInput =
  | string
  | URLSearchParams
  | Record<string, string | number | boolean | undefined | null>;

function buildQuery(query?: ApiQueryInput): string {
  if (query == null || query === '') return '';
  if (typeof query === 'string') return query.replace(/^\?/, '');
  if (query instanceof URLSearchParams) return query.toString();
  const u = new URLSearchParams();
  for (const [k, v] of Object.entries(query)) {
    if (v === undefined || v === null || v === '') continue;
    u.set(k, String(v));
  }
  return u.toString();
}

/**
 * GET with query string; response typed from OpenAPI path when schema declares JSON.
 * Callers may cast when the spec omits `content` (users/tokens/geo lists).
 */
export function apiGetQuery<P extends keyof paths & string>(
  path: P,
  query?: ApiQueryInput,
  init?: RequestInit,
): Promise<ApiJson<P, 'get'> extends never ? unknown : ApiJson<P, 'get'>> {
  const qs = buildQuery(query);
  const url = qs ? `${path}?${qs}` : path;
  return apiFetch(url, init) as Promise<
    ApiJson<P, 'get'> extends never ? unknown : ApiJson<P, 'get'>
  >;
}

/** Substitute `{param}` segments; values are encodeURIComponent'd. */
export function apiPath(template: string, params: Record<string, string>): string {
  return template.replace(/\{(\w+)\}/g, (_, key: string) => {
    const v = params[key];
    if (v == null || v === '') {
      throw new Error(`apiPath: missing {${key}}`);
    }
    return encodeURIComponent(v);
  });
}

export function apiDelete(path: string, init?: RequestInit): Promise<unknown> {
  return apiFetch(path, { ...init, method: 'DELETE' });
}

export function apiPut(path: string, body?: unknown, init?: RequestInit): Promise<unknown> {
  return apiFetch(path, {
    ...init,
    method: 'PUT',
    body: body === undefined ? init?.body : JSON.stringify(body),
  });
}
