import { afterEach, describe, expect, it, vi } from 'vitest';
import { copyText } from './clipboard';

describe('copyText', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    document.body.replaceChildren();
    Reflect.deleteProperty(document, 'execCommand');
  });

  it('returns false for empty input', async () => {
    expect(await copyText('')).toBe(false);
  });

  it('uses clipboard API in a secure context', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal('isSecureContext', true);
    vi.stubGlobal('navigator', { clipboard: { writeText } });
    expect(await copyText('curl …')).toBe(true);
    expect(writeText).toHaveBeenCalledWith('curl …');
  });

  it('falls back to execCommand when clipboard API is unavailable', async () => {
    vi.stubGlobal('isSecureContext', false);
    vi.stubGlobal('navigator', {});
    Object.defineProperty(document, 'execCommand', {
      configurable: true,
      value: vi.fn().mockReturnValue(true),
    });
    expect(await copyText('hello')).toBe(true);
    expect(document.execCommand).toHaveBeenCalledWith('copy');
  });

  it('falls back when clipboard.writeText rejects', async () => {
    vi.stubGlobal('isSecureContext', true);
    vi.stubGlobal('navigator', {
      clipboard: { writeText: vi.fn().mockRejectedValue(new Error('denied')) },
    });
    Object.defineProperty(document, 'execCommand', {
      configurable: true,
      value: vi.fn().mockReturnValue(true),
    });
    expect(await copyText('retry')).toBe(true);
    expect(document.execCommand).toHaveBeenCalledWith('copy');
  });
});
