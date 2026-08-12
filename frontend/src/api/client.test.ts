import { afterEach, describe, expect, it, vi } from 'vitest';
import { ApiError, apiFetch, authHeaders, csrfToken, isAbortError } from './client';

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

describe('csrfToken / authHeaders', () => {
  afterEach(() => {
    document.cookie = 'nm_csrf=; Max-Age=0; path=/';
    delete (window as { NM_CONFIG?: unknown }).NM_CONFIG;
  });

  it('reads nm_csrf cookie', () => {
    document.cookie = 'nm_csrf=' + encodeURIComponent('tok&1');
    expect(csrfToken()).toBe('tok&1');
  });

  it('adds CSRF and optional Bearer', () => {
    document.cookie = 'nm_csrf=abc';
    window.NM_CONFIG = { apiAuthToken: 'secret' };
    expect(authHeaders()).toEqual({
      Authorization: 'Bearer secret',
      'X-CSRF-Token': 'abc',
    });
  });
});

describe('apiFetch', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    document.cookie = 'nm_csrf=; Max-Age=0; path=/';
  });

  it('sends same-origin credentials and CSRF on JSON POST', async () => {
    document.cookie = 'nm_csrf=csrf-xyz';
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
});
