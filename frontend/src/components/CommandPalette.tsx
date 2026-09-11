import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { useAuth } from '@/auth/AuthContext';
import {
  buildCommandCatalog,
  filterCommands,
  type AppCommand,
} from '@/lib/commandCatalog';
import './command-palette.css';

function isTypingTarget(el: EventTarget | null): boolean {
  if (!(el instanceof HTMLElement)) return false;
  const tag = el.tagName;
  if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return true;
  if (el.isContentEditable) return true;
  return Boolean(el.closest('[contenteditable="true"]'));
}

export function CommandPaletteHost() {
  const { user, isAdmin, reputationEnabled, uiAuthEnabled } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [active, setActive] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  const catalog = useMemo(
    () =>
      buildCommandCatalog({
        isAdmin,
        reputationEnabled,
        uiAuthEnabled,
        pathname: location.pathname,
      }),
    [isAdmin, reputationEnabled, uiAuthEnabled, location.pathname],
  );

  const results = useMemo(() => filterCommands(catalog, query), [catalog, query]);

  useEffect(() => {
    setActive(0);
  }, [query, open]);

  useEffect(() => {
    if (!open) return;
    const id = window.setTimeout(() => inputRef.current?.focus(), 0);
    return () => window.clearTimeout(id);
  }, [open]);

  const close = useCallback(() => {
    setOpen(false);
    setQuery('');
  }, []);

  const runCommand = useCallback(
    async (cmd: AppCommand) => {
      close();
      if (cmd.run) {
        const cont = await cmd.run();
        if (cont === false) return;
      }
      if (cmd.href) navigate(cmd.href);
    },
    [close, navigate],
  );

  useEffect(() => {
    if (!user || user.authDisabled) return;
    if (location.pathname === '/login' || location.pathname === '/change-password') return;

    function onKey(e: KeyboardEvent) {
      const mod = e.metaKey || e.ctrlKey;
      if (mod && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        setOpen((v) => !v);
        return;
      }
      if (e.key === '/' && !mod && !e.altKey && !isTypingTarget(e.target)) {
        e.preventDefault();
        setOpen(true);
      }
    }
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [user, location.pathname]);

  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        e.preventDefault();
        close();
        return;
      }
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setActive((i) => Math.min(i + 1, Math.max(0, results.length - 1)));
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setActive((i) => Math.max(i - 1, 0));
        return;
      }
      if (e.key === 'Enter') {
        e.preventDefault();
        const cmd = results[active];
        if (cmd) void runCommand(cmd);
      }
    }
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [open, results, active, close, runCommand]);

  useEffect(() => {
    if (!open || !listRef.current) return;
    const el = listRef.current.querySelector<HTMLElement>(`[data-idx="${active}"]`);
    el?.scrollIntoView({ block: 'nearest' });
  }, [active, open, results]);

  if (!open) return null;

  let lastGroup = '';

  return (
    <div className="ga-cmd-overlay" role="presentation" onMouseDown={close}>
      <div
        className="ga-cmd-dialog"
        role="dialog"
        aria-modal="true"
        aria-label="Командная палитра"
        onMouseDown={(e) => e.stopPropagation()}
      >
        <input
          ref={inputRef}
          className="ga-cmd-input"
          type="search"
          placeholder="Перейти или выполнить…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          aria-controls="ga-cmd-list"
          aria-autocomplete="list"
        />
        <div className="ga-cmd-list" id="ga-cmd-list" role="listbox" ref={listRef}>
          {!results.length ? (
            <div className="ga-cmd-empty">Ничего не найдено</div>
          ) : (
            results.map((cmd, idx) => {
              const showGroup = cmd.groupLabel !== lastGroup;
              lastGroup = cmd.groupLabel;
              return (
                <div key={cmd.id}>
                  {showGroup ? <div className="ga-cmd-group">{cmd.groupLabel}</div> : null}
                  <button
                    type="button"
                    role="option"
                    data-idx={idx}
                    aria-selected={idx === active}
                    className={`ga-cmd-item${idx === active ? ' active' : ''}`}
                    onMouseEnter={() => setActive(idx)}
                    onClick={() => void runCommand(cmd)}
                  >
                    <span className="ga-cmd-item-label">{cmd.label}</span>
                    {cmd.href ? <span className="ga-cmd-item-meta">{cmd.href}</span> : null}
                  </button>
                </div>
              );
            })
          )}
        </div>
        <div className="ga-cmd-hint">
          <span>↑↓</span> выбор · <span>Enter</span> выполнить · <span>Esc</span> закрыть
        </div>
      </div>
    </div>
  );
}
