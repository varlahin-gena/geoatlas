import { describe, expect, it } from 'vitest';
import { COPY, TERM, scopeLabel, scopeOptionLabel } from './glossary';

describe('glossary', () => {
  it('exposes core RU terms', () => {
    expect(TERM.hunts).toBe('Охоты');
    expect(TERM.peer).toBe('Пир');
    expect(TERM.episode).toBe('Эпизод');
    expect(TERM.scope).toBe('Область');
  });

  it('keeps operator-facing copy without EN leftovers', () => {
    expect(COPY.episodesLoadFailed).not.toMatch(/episode|failed/i);
    expect(COPY.huntsEmptyTitle).not.toMatch(/saved|hunt/i);
    expect(COPY.huntsLoadFailed).toContain('охот');
  });

  it('labels token scopes for UI', () => {
    expect(scopeLabel('read')).toBe('чтение');
    expect(scopeLabel('ops')).toBe('ops');
    expect(scopeOptionLabel('admin')).toContain('полный API');
  });
});
