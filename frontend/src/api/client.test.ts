import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  ApiError,
  apiFetch,
  apiFetchRaw,
  apiGet,
  apiGetQuery,
  apiPath,
  apiPost,
  authHeaders,
  csrfToken,
  isAbortError,
  isAuthApiPath,
} from './client';

describe('isAbortError', () => {
  it('detects DOMException AbortError', () => {
    expect(isAbortError(new DOMException('Aborted', 'AbortError'))).toBe(true);
  });

  it('detects Error with name AbortError', () => {
    const e = new Error('aborted');
    e.name = 'AbortError';
    expect(isAbortError(e)).toBe(true);
  });

  it('rejects other errors', () => {
    expect(isAbortError(new Error('fail'))).toBe(false);
    expect(isAbortError(new DOMException('Denied', 'NotAllowedError'))).toBe(false);
    expect(isAbortError(null)).toBe(false);
    expect(isAbortError('AbortError')).toBe(false);
  });
});

describe('isAuthApiPath', () => {
  it('matches auth routes only', () => {
    expect(isAuthApiPath('/api/auth/login')).toBe(true);
    expect(isAuthApiPath('/api/auth/me?x=1')).toBe(true);
    expect(isAuthApiPath('/api/events')).toBe(false);
    expect(isAuthApiPath('/api/events?days=1')).toBe(false);
  });
});

describe('csrfToken / authHeaders', () => {
  afterEach(() => {
    document.cookie = 'ga_csrf=; Max-Age=0; path=/';
    delete (window as { GA_CONFIG?: unknown }).GA_CONFIG;
  });

  it('reads ga_csrf cookie', () => {
    document.cookie = 'ga_csrf=' + encodeURIComponent('tok&1');
    expect(csrfToken()).toBe('tok&1');
  });

  it('adds CSRF and optional Bearer', () => {
    document.cookie = 'ga_csrf=abc';
    window.GA_CONFIG = { apiAuthToken: 'secret' };
    expect(authHeaders()).toEqual({
      Authorization: 'Bearer secret',
      'X-CSRF-Token': 'abc',
    });
  });
});

describe('apiFetch', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    document.cookie = 'ga_csrf=; Max-Age=0; path=/';
  });

  it('sends same-origin credentials and CSRF on JSON POST', async () => {
    document.cookie = 'ga_csrf=csrf-xyz';
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    vi.stubGlobal('fetch', fetchMock);

    await apiFetch('/api/x', { method: 'POST', body: JSON.stringify({ a: 1 }) });

    expect(fetchMock).toHaveBeenCalledOnce();
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(init.credentials).toBe('same-origin');
    const headers = init.headers as Record<string, string>;
    expect(headers['X-CSRF-Token']).toBe('csrf-xyz');
    expect(headers['Content-Type']).toBe('application/json');
  });

  it('throws ApiError with server message', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ error: 'nope' }), {
          status: 403,
          headers: { 'Content-Type': 'application/json' },
        }),
      ),
    );
    await expect(apiFetch('/api/x')).rejects.toMatchObject({
      status: 403,
      message: 'nope',
    } satisfies Partial<ApiError>);
  });

  it('dispatches session-expired on 401 outside /api/auth', async () => {
    const spy = vi.fn();
    window.addEventListener('ga-session-expired', spy);
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ error: 'unauthorized' }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        }),
      ),
    );
    await expect(apiFetch('/api/events')).rejects.toMatchObject({ status: 401 });
    expect(spy).toHaveBeenCalledOnce();
    window.removeEventListener('ga-session-expired', spy);
  });

  it('does not dispatch session-expired on /api/auth/login 401', async () => {
    const spy = vi.fn();
    window.addEventListener('ga-session-expired', spy);
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ error: 'invalid credentials' }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        }),
      ),
    );
    await expect(apiFetch('/api/auth/login', { method: 'POST', body: '{}' })).rejects.toMatchObject({
      status: 401,
    });
    expect(spy).not.toHaveBeenCalled();
    window.removeEventListener('ga-session-expired', spy);
  });
});

describe('apiFetchRaw', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('dispatches session-expired on /upload-geo 401', async () => {
    const spy = vi.fn();
    window.addEventListener('ga-session-expired', spy);
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ error: 'unauthorized' }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        }),
      ),
    );
    const res = await apiFetchRaw('/upload-geo', { method: 'POST' });
    expect(res.status).toBe(401);
    expect(spy).toHaveBeenCalledOnce();
    window.removeEventListener('ga-session-expired', spy);
  });
});

describe('apiGet / apiPost', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('GET /api/auth/me does not dispatch session-expired on 401', async () => {
    const spy = vi.fn();
    window.addEventListener('ga-session-expired', spy);
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ error: 'unauthorized' }), {
          status: 401,
          headers: { 'Content-Type': 'application/json' },
        }),
      ),
    );
    await expect(apiGet('/api/auth/me')).rejects.toMatchObject({ status: 401 });
    expect(spy).not.toHaveBeenCalled();
    window.removeEventListener('ga-session-expired', spy);
  });

  it('POST /api/auth/login sends JSON body', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ username: 'admin', role: 'administrator', reputationEnabled: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    vi.stubGlobal('fetch', fetchMock);
    await apiPost('/api/auth/login', { username: 'admin', password: 'x' });
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(init.method).toBe('POST');
    expect(init.body).toBe(JSON.stringify({ username: 'admin', password: 'x' }));
  });
});

describe('apiGetQuery / apiPath', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('appends query params to OpenAPI path', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    vi.stubGlobal('fetch', fetchMock);
    await apiGetQuery('/api/system/history', { period: '1h' });
    expect(fetchMock.mock.calls[0][0]).toBe('/api/system/history?period=1h');
  });

  it('encodes path template params', () => {
    expect(apiPath('/api/users/{username}/role', { username: 'a/b' })).toBe(
      '/api/users/a%2Fb/role',
    );
  });
});
