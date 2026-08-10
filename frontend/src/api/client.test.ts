import { describe, expect, it } from 'vitest';
import { isAbortError } from './client';

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
