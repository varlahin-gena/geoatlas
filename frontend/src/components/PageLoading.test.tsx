import { act, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { PageLoading } from './Skeleton';

describe('PageLoading', () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it('shows skeleton first, then stuck recovery UI', () => {
    vi.useFakeTimers();
    render(<PageLoading stuckAfterMs={100} />);
    expect(screen.getByTestId('page-skeleton')).toBeInTheDocument();
    act(() => {
      vi.advanceTimersByTime(100);
    });
    expect(screen.getByTestId('page-loading-stuck')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Перезагрузить страницу' })).toBeInTheDocument();
  });
});
