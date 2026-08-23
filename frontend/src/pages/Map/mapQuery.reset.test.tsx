import { MemoryRouter, useSearchParams } from 'react-router-dom';
import { act, renderHook, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { ReactNode } from 'react';
import { useMapViewQuery } from './mapQuery';

function wrapperWith(initial: string) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return <MemoryRouter initialEntries={[initial]}>{children}</MemoryRouter>;
  };
}

describe('useMapViewQuery alert / resetToLiveView', () => {
  it('exposes alertFingerprint from URL', () => {
    const { result } = renderHook(() => useMapViewQuery(), {
      wrapper: wrapperWith('/?alert=fp-123&filter=blocked&q=test'),
    });
    expect(result.current.alertFingerprint).toBe('fp-123');
    expect(result.current.filter).toBe('blocked');
    expect(result.current.search).toBe('test');
  });

  it('resetToLiveView clears search, filter and alert', async () => {
    const { result } = renderHook(
      () => {
        const view = useMapViewQuery();
        const [params] = useSearchParams();
        return { view, params };
      },
      { wrapper: wrapperWith('/?alert=fp-9&filter=blocked&q=evil&country=Russia') },
    );

    expect(result.current.view.alertFingerprint).toBe('fp-9');

    await act(async () => {
      result.current.view.resetToLiveView();
    });

    await waitFor(() => {
      expect(result.current.view.filter).toBe('all');
      expect(result.current.view.search).toBe('');
      expect(result.current.view.focusedCountry).toBeNull();
      expect(result.current.view.alertFingerprint).toBeNull();
      expect(result.current.params.get('alert')).toBeNull();
      expect(result.current.params.toString()).toBe('');
    });
  });
});
