import { FormEvent, useCallback, useEffect, useRef, useState } from 'react';
import {
  fetchTlsStatus,
  postTlsReload,
  putTls,
  type TlsStatus,
} from '@/api/system';
import { SettingsLayout } from '@/components/SettingsLayout';
import { ReauthField } from '@/components/ReauthModal';
import { useToast } from '@/components/Toast';
import { fmtDate } from '@/lib/format';
import './tls.css';

function readFile(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ''));
    reader.onerror = () => reject(reader.error || new Error('read failed'));
    reader.readAsText(file);
  });
}

function httpsModeLabel(raw?: string): string {
  const v = (raw || 'auto').trim().toLowerCase();
  if (v === '0' || v === 'false' || v === 'no' || v === 'off') {
    return 'выкл (только HTTP)';
  }
  if (v === '1' || v === 'true' || v === 'yes' || v === 'on') {
    return 'вкл';
  }
  return 'auto (вкл., если есть оба PEM)';
}

export default function TlsPage() {
  const { toast } = useToast();
  const certInputRef = useRef<HTMLInputElement>(null);
  const keyInputRef = useRef<HTMLInputElement>(null);
  const [status, setStatus] = useState<TlsStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [certPEM, setCertPEM] = useState('');
  const [keyPEM, setKeyPEM] = useState('');
  const [certName, setCertName] = useState('');
  const [keyName, setKeyName] = useState('');
  const [busy, setBusy] = useState(false);
  const [reauthPassword, setReauthPassword] = useState('');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await fetchTlsStatus();
      setStatus(data.tls || data);
    } catch (e) {
      toast(e instanceof Error ? e.message : 'Не удалось загрузить статус TLS', 'error');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    document.title = 'ГеоАтлас — HTTPS-сертификаты';
    void load();
  }, [load]);

  async function onPickCert(file: File | null) {
    if (!file) return;
    try {
      setCertPEM(await readFile(file));
      setCertName(file.name);
    } catch {
      toast('Не удалось прочитать файл сертификата', 'error');
    }
  }

  async function onPickKey(file: File | null) {
    if (!file) return;
    try {
      setKeyPEM(await readFile(file));
      setKeyName(file.name);
    } catch {
      toast('Не удалось прочитать файл ключа', 'error');
    }
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!certPEM.trim() || !keyPEM.trim()) {
      toast('Выберите fullchain.pem и privkey.pem', 'error');
      return;
    }
    if (!reauthPassword.trim()) {
      toast('Введите пароль для подтверждения', 'error');
      return;
    }
    setBusy(true);
    try {
      const res = await putTls({
        cert_pem: certPEM.trim(),
        key_pem: keyPEM.trim(),
        current_password: reauthPassword,
      });
      setCertPEM('');
      setKeyPEM('');
      setCertName('');
      setKeyName('');
      setReauthPassword('');
      if (certInputRef.current) certInputRef.current.value = '';
      if (keyInputRef.current) keyInputRef.current.value = '';
      toast('Сертификаты сохранены', 'success');
      if (res.reload?.message) {
        toast(res.reload.message, res.reload.reloaded ? 'success' : 'info');
      }
      void load();
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось сохранить сертификаты', 'error');
    } finally {
      setBusy(false);
    }
  }

  async function onReload() {
    setBusy(true);
    try {
      const res = await postTlsReload();
      if (res.reload?.message) toast(res.reload.message, res.reload.reloaded ? 'success' : 'info');
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Не удалось применить сертификаты', 'error');
    } finally {
      setBusy(false);
    }
  }

  const cert = status?.cert;
  const expiringSoon = cert?.days_left != null && cert.days_left <= 30;
  const canUpload = Boolean(status?.writable);
  const canSave = Boolean(canUpload && certPEM && keyPEM && reauthPassword.trim() && !busy);
  const httpsOff =
    status != null &&
    ['0', 'false', 'no', 'off'].includes(String(status.https_enabled || '').trim().toLowerCase());

  return (
    <SettingsLayout title="HTTPS-сертификаты" plainTitle>
      <div className="page-content-inner narrow">
        <p className="page-lead">
          Режим — <code>HTTPS_ENABLED</code> в <code>.env</code> (<code>0</code> / <code>1</code> /{' '}
          <code>auto</code>). PEM пишутся в том <code>./certs</code>.
        </p>

        <div className="page-actions">
          <button type="button" className="btn" onClick={() => void load()} disabled={loading}>
            {loading ? 'Обновление…' : 'Обновить'}
          </button>
        </div>

        {!status?.configured ? (
          <p className="tls-banner" role="status">
            Управление сертификатами через веб недоступно: не смонтирован{' '}
            <code>TLS_CERT_DIR</code> в backend. Положите PEM вручную в <code>certs/</code> на
            сервере или разверните стек через Docker Compose с томом <code>./certs</code>.
          </p>
        ) : null}

        {status?.configured && !status.writable ? (
          <p className="tls-banner" role="status">
            Каталог сертификатов смонтирован, но недоступен для записи процессу backend (uid 101).
            На хосте: <code>sudo chown -R 101:101 certs</code>, затем перезапустите стек (
            <code>./start.sh</code>). Форма загрузки скрыта, пока каталог не станет writable.
          </p>
        ) : null}

        {httpsOff ? (
          <p className="tls-banner" role="status">
            В конфигурации сейчас <code>HTTPS_ENABLED=0</code> (только HTTP). Загрузка PEM всё
            равно возможна; чтобы включить HTTPS, выставьте <code>HTTPS_ENABLED=auto</code> или{' '}
            <code>1</code> в <code>.env</code> и перезапустите стек.
          </p>
        ) : null}

        <section className="card tls-card" aria-labelledby="tls-status-title">
          <h2 id="tls-status-title" className="card-title">
            Текущий статус
          </h2>
          {loading && !status ? (
            <p className="tls-status-note">Загрузка…</p>
          ) : (
            <dl className="tls-status-grid">
              <div>
                <dt>HTTPS</dt>
                <dd>
                  {httpsModeLabel(status?.https_enabled)}
                  {status?.https_enabled ? (
                    <span className="hint"> ({status.https_enabled})</span>
                  ) : null}
                </dd>
              </div>
              <div>
                <dt>Порт HTTPS</dt>
                <dd>{status?.https_port || '443'}</dd>
              </div>
              <div>
                <dt>Редирект HTTP→HTTPS</dt>
                <dd>{status?.http_redirect === '0' ? 'выкл' : 'вкл'}</dd>
              </div>
              <div>
                <dt>Хранилище UI</dt>
                <dd>
                  {!status?.configured
                    ? 'нет (TLS_CERT_DIR)'
                    : status.writable
                      ? 'доступно для записи'
                      : 'только чтение / нет прав'}
                </dd>
              </div>
              <div>
                <dt>Файлы на диске</dt>
                <dd>
                  <span className="tls-files">
                    <span
                      className={`tls-file-chip${status?.cert_present ? ' is-present' : ''}`}
                    >
                      fullchain.pem
                    </span>
                    <span className={`tls-file-chip${status?.key_present ? ' is-present' : ''}`}>
                      privkey.pem
                    </span>
                  </span>
                </dd>
              </div>
            </dl>
          )}

          {cert ? (
            <div className={`tls-cert-summary${expiringSoon ? ' is-warn' : ''}`}>
              <div>
                <span className="hint">Субъект</span>
                <strong>{cert.subject}</strong>
              </div>
              <div>
                <span className="hint">Издатель</span>
                <strong>{cert.issuer}</strong>
              </div>
              <div>
                <span className="hint">Действует до</span>
                <strong>
                  {cert.not_after ? fmtDate(cert.not_after) : '—'}
                  {cert.days_left != null ? ` (${cert.days_left} дн.)` : ''}
                </strong>
              </div>
              {cert.self_signed ? (
                <p className="tls-banner tls-banner-inline">Самоподписанный сертификат</p>
              ) : null}
              {cert.sans?.length ? (
                <p className="hint">
                  SAN: {cert.sans.slice(0, 8).join(', ')}
                  {cert.sans.length > 8 ? '…' : ''}
                </p>
              ) : null}
            </div>
          ) : status?.configured ? (
            <p className="tls-status-note">
              Сертификат ещё не загружен — HTTPS включится после добавления PEM и применения
              конфигурации (при режиме auto или вкл.).
            </p>
          ) : null}

          {status?.configured ? (
            <div className="tls-card-actions">
              <button type="button" className="btn" disabled={busy} onClick={() => void onReload()}>
                Применить
              </button>
              <span className="hint">Перезагружает nginx с текущими PEM на диске</span>
            </div>
          ) : null}
        </section>

        {canUpload ? (
          <form
            className="card tls-card tls-upload-form"
            aria-labelledby="tls-upload-title"
            onSubmit={(e) => void onSubmit(e)}
          >
            <h2 id="tls-upload-title" className="card-title">
              Загрузить сертификат
            </h2>
            <p className="tls-upload-lead">
              Выберите <code>fullchain.pem</code> и <code>privkey.pem</code>, затем сохраните.
            </p>

            <input
              ref={certInputRef}
              type="file"
              accept=".pem,.crt,.cer,.txt"
              className="visually-hidden"
              disabled={busy}
              onChange={(e) => void onPickCert(e.target.files?.[0] || null)}
            />
            <input
              ref={keyInputRef}
              type="file"
              accept=".pem,.key,.txt"
              className="visually-hidden"
              disabled={busy}
              onChange={(e) => void onPickKey(e.target.files?.[0] || null)}
            />

            <div className="tls-pick-row">
              <div className="tls-pick">
                <button
                  type="button"
                  className="btn"
                  disabled={busy}
                  onClick={() => certInputRef.current?.click()}
                >
                  Выбрать сертификат…
                </button>
                <span className={`tls-pick-name${certName ? ' is-ready' : ''}`} title={certName || undefined}>
                  {certName || 'fullchain.pem ещё не выбран'}
                </span>
              </div>
              <div className="tls-pick">
                <button
                  type="button"
                  className="btn"
                  disabled={busy}
                  onClick={() => keyInputRef.current?.click()}
                >
                  Выбрать ключ…
                </button>
                <span className={`tls-pick-name${keyName ? ' is-ready' : ''}`} title={keyName || undefined}>
                  {keyName || 'privkey.pem ещё не выбран'}
                </span>
              </div>
            </div>

            <ReauthField
              value={reauthPassword}
              onChange={setReauthPassword}
              id="tlsReauthPassword"
              label="Пароль"
            />
            <div className="tls-card-actions">
              <button type="submit" className="btn primary" disabled={!canSave}>
                {busy ? 'Сохранение…' : 'Сохранить сертификаты'}
              </button>
            </div>
          </form>
        ) : null}
      </div>
    </SettingsLayout>
  );
}
