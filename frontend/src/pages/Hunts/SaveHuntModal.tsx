import { FormEvent, useEffect, useId, useRef, useState } from 'react';
import { useToast } from '@/components/Toast';
import type { HuntMapState } from '@/api/hunts';
import { huntSnapshotLines, saveHuntFromMap, suggestHuntName } from './saveHuntFromMap';
import './hunts.css';

const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

function useDialogA11y(onDismiss: () => void) {
  const dialogRef = useRef<HTMLFormElement>(null);
  const onDismissRef = useRef(onDismiss);
  onDismissRef.current = onDismiss;

  useEffect(() => {
    const root = dialogRef.current;
    if (!root) return;
    const prev = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const nodes = () => Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE));
    const preferred = root.querySelector<HTMLElement>('#save-hunt-name');
    (preferred ?? nodes()[0] ?? root).focus();

    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        e.preventDefault();
        onDismissRef.current();
        return;
      }
      if (e.key !== 'Tab') return;
      const list = nodes();
      if (list.length === 0) {
        e.preventDefault();
        root?.focus();
        return;
      }
      const first = list[0];
      const last = list[list.length - 1];
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      }
    }
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('keydown', onKey);
      prev?.focus();
    };
  }, []);

  return dialogRef;
}

type Props = {
  mapState: HuntMapState;
  onClose: () => void;
};

export function SaveHuntModal({ mapState, onClose }: Props) {
  const { toast } = useToast();
  const titleId = useId();
  const dialogRef = useDialogA11y(onClose);
  const [name, setName] = useState(() => suggestHuntName(mapState));
  const [busy, setBusy] = useState(false);
  const snapshot = huntSnapshotLines(mapState);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!name.trim() || busy) return;
    setBusy(true);
    try {
      const ok = await saveHuntFromMap(name, mapState, toast);
      if (ok) onClose();
    } finally {
      setBusy(false);
    }
  }

  return (
    <div
      className="modal-backdrop show save-hunt-backdrop"
      role="presentation"
      onClick={(e) => {
        if (e.target === e.currentTarget && !busy) onClose();
      }}
    >
      <form
        ref={dialogRef}
        className="modal save-hunt-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
        onSubmit={(e) => void onSubmit(e)}
      >
        <h3 id={titleId}>Сохранить охоту</h3>
        <p className="save-hunt-lead">Текущий вид карты — период, группировка и фильтры.</p>

        <div className="field">
          <label htmlFor="save-hunt-name">Название</label>
          <input
            id="save-hunt-name"
            type="text"
            autoComplete="off"
            required
            maxLength={120}
            value={name}
            disabled={busy}
            onChange={(e) => setName(e.target.value)}
          />
        </div>

        {snapshot.length ? (
          <ul className="save-hunt-snapshot">
            {snapshot.map((line) => (
              <li key={line}>{line}</li>
            ))}
          </ul>
        ) : null}

        <div className="modal-actions">
          <button type="button" className="btn" disabled={busy} onClick={onClose}>
            Отмена
          </button>
          <button
            type="submit"
            className="btn primary"
            disabled={busy || !name.trim()}
            aria-busy={busy}
          >
            {busy ? 'Сохранение…' : 'Сохранить'}
          </button>
        </div>
      </form>
    </div>
  );
}
