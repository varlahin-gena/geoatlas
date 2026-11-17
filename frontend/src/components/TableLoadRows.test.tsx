import { act, cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { TableLoadRows } from './Skeleton';

describe('TableLoadRows', () => {
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
  });

  it('shows skeleton, then stuck recovery with retry', () => {
    vi.useFakeTimers();
    const onRetry = vi.fn();
    render(
      <table>
        <tbody>
          <TableLoadRows cols={3} loading stuckAfterMs={100} onRetry={onRetry} />
        </tbody>
      </table>,
    );
    expect(document.querySelectorAll('.skeleton-table-row').length).toBeGreaterThan(0);
    act(() => {
      vi.advanceTimersByTime(100);
    });
    expect(screen.getByTestId('table-loading-stuck')).toBeInTheDocument();
    screen.getByRole('button', { name: 'Повторить' }).click();
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it('shows error with retry when not loading', () => {
    const onRetry = vi.fn();
    render(
      <table>
        <tbody>
          <TableLoadRows cols={2} loading={false} error="сеть недоступна" onRetry={onRetry} />
        </tbody>
      </table>,
    );
    expect(screen.getByText('Не удалось загрузить')).toBeInTheDocument();
    expect(screen.getByText('сеть недоступна')).toBeInTheDocument();
    screen.getByRole('button', { name: 'Повторить' }).click();
    expect(onRetry).toHaveBeenCalledTimes(1);
  });
});
