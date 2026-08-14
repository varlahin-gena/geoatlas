import { useEffect, useRef, type RefObject } from 'react';
import { Link } from 'react-router-dom';
import { fmtNumber } from '@/lib/format';
import { formatNetworkHint, type DryRunPreview, type GeoStatus, type GeoWizardStep } from './geoWizard';

const FOCUSABLE =
  'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

function useDialogA11y(onDismiss: () => void) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const onDismissRef = useRef(onDismiss);
  onDismissRef.current = onDismiss;

  useEffect(() => {
    const root = dialogRef.current;
    if (!root) return;
    const prev = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const nodes = () => Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE));
    (nodes()[0] ?? root).focus();

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

interface Props {
  step: GeoWizardStep;
  setStep: (s: GeoWizardStep) => void;
  busy: boolean;
  geo: GeoStatus | null;
  preview: DryRunPreview | null;
  pendingFile: File | null;
  pollNote: string;
  curlSnippet: string;
  fileRef: RefObject<HTMLInputElement | null>;
  onDismiss: () => void;
  onCloseSuccess: () => void;
  onMoreUpload: () => void;
  onDryRun: (file: File) => void;
  onCommit: () => void;
  onWaitForCurl: () => void;
}

const STEPS: { id: GeoWizardStep; label: string }[] = [
  { id: 'why', label: '1. Почему пусто' },
  { id: 'upload', label: '2. Загрузка' },
  { id: 'done', label: '3. Готово' },
];

export function GeoWizardModal(props: Props) {
  const {
    step,
    setStep,
    busy,
    geo,
    preview,
    pendingFile,
    pollNote,
    curlSnippet,
    fileRef,
    onDismiss,
    onCloseSuccess,
    onMoreUpload,
    onDryRun,
    onCommit,
    onWaitForCurl,
  } = props;

  const dialogRef = useDialogA11y(onDismiss);

  async function copyCurl() {
    try {
      await navigator.clipboard.writeText(curlSnippet);
    } catch {
      /* ignore */
    }
  }

  return (
    <div
      className="modal-backdrop show geo-wizard-backdrop"
      role="presentation"
      onClick={(e) => {
        if (e.target === e.currentTarget) onDismiss();
      }}
    >
      <div
        ref={dialogRef}
        className="modal geo-wizard"
        role="dialog"
        aria-modal="true"
        aria-labelledby="geo-wizard-title"
        tabIndex={-1}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="geo-wizard-header">
          <h3 id="geo-wizard-title">Мастер GeoIP</h3>
          <p className="geo-wizard-sub">
            Карта рисует только IP с координатами. Без базы GeoIP дуги не появятся.
          </p>
        </div>

        <nav className="geo-wizard-steps" aria-label="Шаги мастера">
          {STEPS.map((s) => (
            <button
              key={s.id}
              type="button"
              className={`geo-wizard-step${step === s.id ? ' active' : ''}`}
              onClick={() => setStep(s.id)}
              disabled={busy && s.id !== step}
            >
              {s.label}
            </button>
          ))}
        </nav>

        <div className="geo-wizard-body">
          {step === 'why' ? (
            <div className="geo-wizard-panel">
              <p>
                После установки ClickHouse и ingest уже работают, но таблица <code>geo_ranges</code>{' '}
                пуста. События пишутся, а на карту не попадают — нет lat/lon.
              </p>
              <ul>
                <li>
                  База в индексе:{' '}
                  <strong>{geo ? fmtNumber(geo.count) : '…'}</strong> диапазонов
                  {geo && !geo.indexReady ? ' (индекс ещё загружается)' : ''}
                </li>
                <li>Формат CSV — как в SIEM KUMA (Network, Country, Region, City, Latitude, Longitude)</li>
                <li>Большие файлы надёжнее заливать с сервера через curl</li>
              </ul>
              <div className="modal-actions">
                <button type="button" className="btn" onClick={onDismiss}>
                  Позже
                </button>
                <button
                  type="button"
                  className="btn primary"
                  onClick={() => setStep('upload')}
                  disabled={busy}
                >
                  Загрузить базу
                </button>
              </div>
            </div>
          ) : null}

          {step === 'upload' ? (
            <div className="geo-wizard-panel">
              <div className="geo-wizard-paths">
                <section>
                  <h4>Из файла</h4>
                  <p className="geo-wizard-hint">
                    Сначала проверка (<code>dry_run</code>), затем запись. Лимит:{' '}
                    {geo?.uploadMaxBytes
                      ? `${fmtNumber(geo.uploadMaxBytes)} байт`
                      : 'по профилю установки'}
                    {geo?.uploadMaxRanges ? `, до ${fmtNumber(geo.uploadMaxRanges)} диапазонов` : ''}.
                  </p>
                  <input
                    ref={fileRef}
                    type="file"
                    accept=".csv,text/csv"
                    className="visually-hidden"
                    tabIndex={-1}
                    onChange={(e) => {
                      const f = e.target.files?.[0];
                      if (f) void onDryRun(f);
                      e.target.value = '';
                    }}
                  />
                  <div className="geo-wizard-row">
                    <button
                      type="button"
                      className="btn"
                      disabled={busy}
                      onClick={() => fileRef.current?.click()}
                    >
                      Выбрать CSV…
                    </button>
                    {pendingFile ? (
                      <span className="geo-wizard-file">
                        {pendingFile.name} ({fmtNumber(pendingFile.size)} байт)
                      </span>
                    ) : null}
                  </div>
                  {preview ? (
                    <div className="geo-wizard-preview">
                      <div className="geo-wizard-preview-title">
                        Проверка: {fmtNumber(preview.ranges)} диапазонов
                      </div>
                      {preview.sample.length ? (
                        <table>
                          <thead>
                            <tr>
                              <th>Network</th>
                              <th>Country</th>
                              <th>City</th>
                            </tr>
                          </thead>
                          <tbody>
                            {preview.sample.slice(0, 5).map((row, i) => (
                              <tr key={i}>
                                <td>{formatNetworkHint(row.start_ip, row.end_ip)}</td>
                                <td>{row.country || '—'}</td>
                                <td>{row.city || '—'}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      ) : null}
                      <button
                        type="button"
                        className="btn primary"
                        disabled={busy || !pendingFile}
                        onClick={() => void onCommit()}
                      >
                        Записать в базу
                      </button>
                    </div>
                  ) : null}
                </section>

                <section>
                  <h4>С сервера (рекомендуется для больших CSV)</h4>
                  <p className="geo-wizard-hint">
                    Скопируйте CSV на хост и выполните команду (нужен{' '}
                    <code>API_AUTH_TOKEN</code> из <code>.env</code>).
                  </p>
                  <pre className="geo-wizard-curl">{curlSnippet}</pre>
                  <div className="geo-wizard-row">
                    <button type="button" className="btn" onClick={() => void copyCurl()} disabled={busy}>
                      Копировать
                    </button>
                    <button
                      type="button"
                      className="btn"
                      disabled={busy}
                      onClick={() => {
                        setStep('done');
                        void onWaitForCurl();
                      }}
                    >
                      Я залил через curl — ждать
                    </button>
                  </div>
                </section>

                <section>
                  <h4>Позже точечно</h4>
                  <p className="geo-wizard-hint">
                    Когда появятся события, IP без координат можно добавить на странице{' '}
                    <Link to="/geo-missing">IP без GeoIP</Link>.
                  </p>
                </section>
              </div>

              <div className="modal-actions">
                <button type="button" className="btn" onClick={() => setStep('why')} disabled={busy}>
                  Назад
                </button>
                <button type="button" className="btn" onClick={onDismiss}>
                  Закрыть
                </button>
              </div>
            </div>
          ) : null}

          {step === 'done' ? (
            <div className="geo-wizard-panel">
              {geo && geo.count > 0 ? (
                <>
                  <p>
                    База GeoIP на месте: <strong>{fmtNumber(geo.count)}</strong> диапазонов
                    {geo.indexReady ? '' : ' (индекс ещё догружается)'}.
                  </p>
                  <p className="geo-wizard-hint">
                    Обновите карту. Если дуг всё ещё нет — проверьте syslog / загрузку логов и период.
                  </p>
                </>
              ) : (
                <>
                  <p>База пока пуста или индекс ещё поднимается.</p>
                  <p className="geo-wizard-hint">{pollNote || 'Можно закрыть и открыть мастер снова из сайдбара.'}</p>
                </>
              )}
              {pollNote && geo && geo.count > 0 ? (
                <p className="geo-wizard-hint">{pollNote}</p>
              ) : null}
              <div className="modal-actions">
                <button
                  type="button"
                  className="btn"
                  disabled={busy}
                  onClick={onMoreUpload}
                >
                  Ещё загрузка
                </button>
                <button
                  type="button"
                  className="btn primary"
                  disabled={busy}
                  onClick={onCloseSuccess}
                >
                  На карту
                </button>
              </div>
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
