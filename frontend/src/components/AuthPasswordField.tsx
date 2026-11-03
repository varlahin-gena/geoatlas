import { useState, type ChangeEvent, type FocusEvent, type KeyboardEvent, type Ref } from 'react';

type AuthPasswordFieldProps = {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  onBlur?: (e: FocusEvent<HTMLInputElement>) => void;
  autoComplete?: string;
  required?: boolean;
  name?: string;
  inputRef?: Ref<HTMLInputElement>;
};

/** Password input with show/hide toggle and Caps Lock hint (auth forms). */
export function AuthPasswordField({
  id,
  label,
  value,
  onChange,
  onBlur,
  autoComplete = 'current-password',
  required,
  name,
  inputRef,
}: AuthPasswordFieldProps) {
  const [visible, setVisible] = useState(false);
  const [capsOn, setCapsOn] = useState(false);

  return (
    <>
      <label htmlFor={id}>{label}</label>
      <div className="auth-password-wrap">
        <input
          ref={inputRef}
          id={id}
          name={name}
          type={visible ? 'text' : 'password'}
          autoComplete={autoComplete}
          required={required}
          value={value}
          onChange={(e: ChangeEvent<HTMLInputElement>) => onChange(e.target.value)}
          onKeyDown={(e: KeyboardEvent<HTMLInputElement>) => setCapsOn(e.getModifierState('CapsLock'))}
          onKeyUp={(e: KeyboardEvent<HTMLInputElement>) => setCapsOn(e.getModifierState('CapsLock'))}
          onBlur={(e: FocusEvent<HTMLInputElement>) => {
            setCapsOn(false);
            onBlur?.(e);
          }}
        />
        <button
          type="button"
          className="auth-password-toggle"
          aria-pressed={visible}
          aria-label={visible ? 'Скрыть пароль' : 'Показать пароль'}
          onClick={() => setVisible((v) => !v)}
        >
          {visible ? <EyeSlashIcon /> : <EyeIcon />}
        </button>
      </div>
      {capsOn ? (
        <p className="auth-caps-hint" role="status">
          Включён Caps Lock
        </p>
      ) : null}
    </>
  );
}

function EyeIcon() {
  return (
    <svg className="auth-password-icon" viewBox="0 0 256 256" aria-hidden="true">
      <path
        fill="currentColor"
        d="M247.31,124.76c-.35-.79-8.82-19.58-27.65-38.41C194.57,61.26,162.88,48,128,48S61.43,61.26,36.34,86.35C17.51,105.18,9,124,8.69,124.76a8,8,0,0,0,0,6.5c.35.79,8.82,19.57,27.65,38.4C61.43,194.74,93.12,208,128,208s66.57-13.26,91.66-38.34c18.83-18.83,27.3-37.61,27.65-38.4A8,8,0,0,0,247.31,124.76ZM128,192c-30.78,0-57.67-11.19-79.93-33.25A133.47,133.47,0,0,1,25,128,133.33,133.33,0,0,1,48.07,97.25C70.33,75.19,97.22,64,128,64s57.67,11.19,79.93,33.25A133.46,133.46,0,0,1,231.05,128C223.84,141.46,192.43,192,128,192Zm0-112a48,48,0,1,0,48,48A48.05,48.05,0,0,0,128,80Zm0,80a32,32,0,1,1,32-32A32,32,0,0,1,128,160Z"
      />
    </svg>
  );
}

function EyeSlashIcon() {
  // Phosphor EyeSlash Regular
  return (
    <svg className="auth-password-icon" viewBox="0 0 256 256" aria-hidden="true">
      <path
        fill="currentColor"
        d="M53.92,34.62A8,8,0,1,0,42.08,45.38L61.32,66.55C25,88.84,9.38,123.2,8.69,124.76a8,8,0,0,0,0,6.5C12.18,139.33,42.6,200,128,200a119.17,119.17,0,0,0,51.63-11.44l20.45,22.52a8,8,0,1,0,11.84-10.76Zm39.3,98.4,14.53,16A32,32,0,0,1,93.22,133ZM128,184c-49.5,0-81.67-26.44-100.12-47.58A134.77,134.77,0,0,1,48.47,103.34L71.87,129A48,48,0,0,0,138.63,167ZM128,56c49.5,0,81.67,26.44,100.12,47.58a134.77,134.77,0,0,1-20.41,32.92l22.47,24.71C243.37,146.26,247.44,134.47,247.31,134.26c-.35-.79-8.82-19.58-27.65-38.41C194.57,70.26,162.88,57,128,57A126.69,126.69,0,0,0,89.32,63.3L107,82.72A48.13,48.13,0,0,1,161.19,117.36l16.58,18.24A133.64,133.64,0,0,0,207.93,112.75C185.67,90.69,158.78,79.5,128,79.5Z"
      />
    </svg>
  );
}
