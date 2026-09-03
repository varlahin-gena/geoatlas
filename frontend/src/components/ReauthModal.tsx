import { FormEvent } from 'react';

type Props = {
  open: boolean;
  title: string;
  message?: string;
  confirmLabel?: string;
  busy?: boolean;
  password: string;
  onPasswordChange: (value: string) => void;
  onCancel: () => void;
  onConfirm: () => void;
};

export function ReauthField({
  id = 'reauthPassword',
  value,
  onChange,
  label = 'Ваш пароль',
  hint = 'Подтвердите действие повторным вводом пароля',
}: {
  id?: string;
  value: string;
  onChange: (value: string) => void;
  label?: string;
  hint?: string;
}) {
  return (
    <div className="field reauth-field">
      <label htmlFor={id}>{label}</label>
      <input
        id={id}
        type="password"
        required
        autoComplete="current-password"
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
      <span className="field-hint">{hint}</span>
    </div>
  );
}

export function ReauthModal({
  open,
  title,
  message,
  confirmLabel = 'Подтвердить',
  busy = false,
  password,
  onPasswordChange,
  onCancel,
  onConfirm,
}: Props) {
  if (!open) return null;

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    onConfirm();
  }

  return (
    <div
      className="modal-backdrop show"
      onClick={(e) => {
        if (e.target === e.currentTarget && !busy) onCancel();
      }}
    >
      <form className="modal" role="dialog" aria-modal="true" onSubmit={onSubmit}>
        <h3>{title}</h3>
        {message ? <p className="hint" style={{ marginTop: 0 }}>{message}</p> : null}
        <ReauthField
          id="reauthModalPassword"
          value={password}
          onChange={onPasswordChange}
          label="Ваш пароль"
        />
        <div className="modal-actions">
          <button type="button" className="btn" disabled={busy} onClick={onCancel}>
            Отмена
          </button>
          <button type="submit" className="btn primary" disabled={busy || !password.trim()}>
            {busy ? 'Проверка…' : confirmLabel}
          </button>
        </div>
      </form>
    </div>
  );
}
