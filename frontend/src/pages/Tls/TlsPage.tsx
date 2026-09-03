import { FormEvent, useCallback, useEffect, useRef, useState } from 'react';
import {
  fetchTlsStatus,
  postTlsReload,
  putTls,
  type TlsStatus,
} from '@/api/system';
import { AdminLayout } from '@/components/AdminLayout';
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
  const canSave = Boolean(status?.writable && certPEM && keyPEM && reauthPassword.trim() && !busy);

  return (
    <AdminLayout
      title="HTTPS-сертификаты"
      actions={
        <button type="button" className="btn" onClick={() => void load()} disabled={loading}>
          Обновить
        </button>
      }
    >
      <div className="page-content-inner narrow">
        <p className="page-lead">
          PEM-файлы сохраняются в каталог <code>certs/</code> на сервере и используются контейнером
          frontend (nginx). После первой установки или смены режима HTTPS может потребоваться
          перезапуск frontend.
        </p>

        {!status?.configured ? (
          <p className="hint warn-banner">
            Хранилище сертификатов недоступно (не смонтирован <code>TLS_CERT_DIR</code> в backend).
            Загрузка через UI работает только при развёртывании через Docker с томом{' '}
            <code>./certs</code>.
          </p>
        ) : null}

        <div className="card">
          <h2>Текущий статус</h2>
          {loading && !status ? (
            <p className="hint">Загрузка…</p>
          ) : (
            <dl className="tls-status-grid">
              <div>
                <dt>HTTPS</dt>
                <dd>{status?.https_enabled || 'auto'}</dd>
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
                <dt>Файлы на диске</dt>
                <dd>
                  {status?.cert_present ? 'fullchain.pem' : '—'} /{' '}
                  {status?.key_present ? 'privkey.pem' : '—'}
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
              {cert.self_signed ? <p className="hint warn-banner">Самоподписанный сертификат</p> : null}
              {cert.sans?.length ? (
                <p className="hint">
                  SAN: {cert.sans.slice(0, 8).join(', ')}
                  {cert.sans.length > 8 ? '…' : ''}
                </p>
              ) : null}
            </div>
          ) : status?.configured ? (
            <p className="hint">
              Сертификат ещё не загружен — HTTPS включится после добавления PEM и перезапуска frontend.
            </p>
          ) : null}

          {status?.configured ? (
            <button type="button" className="btn" disabled={busy} onClick={() => void onReload()}>
              Применить / перезагрузить nginx
            </button>
          ) : null}
        </div>

        <form className="card tls-upload-form" onSubmit={(e) => void onSubmit(e)}>
          <h2>Загрузить сертификат</h2>
          <p className="hint">
            Выберите <code>fullchain.pem</code> и <code>privkey.pem</code>, затем сохраните.
          </p>

          <input
            ref={certInputRef}
            type="file"
            accept=".pem,.crt,.cer,.txt"
            className="visually-hidden"
            disabled={!status?.writable || busy}
            onChange={(e) => void onPickCert(e.target.files?.[0] || null)}
          />
          <input
            ref={keyInputRef}
            type="file"
            accept=".pem,.key,.txt"
            className="visually-hidden"
            disabled={!status?.writable || busy}
            onChange={(e) => void onPickKey(e.target.files?.[0] || null)}
          />

          <div className="tls-upload-actions">
            <button
              type="button"
              className="btn"
              disabled={!status?.writable || busy}
              onClick={() => certInputRef.current?.click()}
            >
              {certName ? `Сертификат: ${certName}` : 'Выбрать fullchain.pem'}
            </button>
            <button
              type="button"
              className="btn"
              disabled={!status?.writable || busy}
              onClick={() => keyInputRef.current?.click()}
            >
              {keyName ? `Ключ: ${keyName}` : 'Выбрать privkey.pem'}
            </button>
          </div>
          <ReauthField value={reauthPassword} onChange={setReauthPassword} id="tlsReauthPassword" />
          <div className="tls-upload-actions">
            <button type="submit" className="btn primary" disabled={!canSave}>
              {busy ? 'Сохранение…' : 'Сохранить сертификаты'}
            </button>
          </div>
        </form>
      </div>
    </AdminLayout>
  );
}
