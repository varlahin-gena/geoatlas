let uploadInFlight = 0;

function onUploadBeforeUnload(e) {
    e.preventDefault();
    e.returnValue = '';
}

function markUploadStart() {
    uploadInFlight++;
    if (uploadInFlight === 1) {
        window.addEventListener('beforeunload', onUploadBeforeUnload);
    }
}

function markUploadEnd() {
    uploadInFlight = Math.max(0, uploadInFlight - 1);
    if (uploadInFlight === 0) {
        window.removeEventListener('beforeunload', onUploadBeforeUnload);
    }
}

/** Браузерный обрыв fetch при уходе со страницы / сети — не путать с ошибкой API. */
function uploadInterruptedMessage(err) {
    const name = err && err.name ? String(err.name) : '';
    const msg = err && err.message ? String(err.message) : String(err || '');
    if (name === 'AbortError') {
        return 'Загрузка прервана. Дождитесь окончания или не переключайте страницу.';
    }
    if (/failed to fetch|networkerror|load failed|network request failed/i.test(msg)) {
        return 'Загрузка прервана (уход со страницы или обрыв связи). '
            + 'Не переключайте вкладки во время upload. '
            + 'Большой GeoIP лучше заливать с сервера через curl (см. README).';
    }
    return null;
}

async function uploadLogs() {
    const fi = document.getElementById('logFile');
    const f = fi.files[0]; if (!f) return;
    toast(`Загрузка логов: ${f.name}`, 'info', 2500);
    markUploadStart();
    try {
        const res = await fetch(`${API_BASE}/upload-logs`, {
            method: 'POST',
            credentials: 'same-origin',
            headers: nmAuthHeaders({ 'Content-Type': 'application/octet-stream' }),
            body: f
        });
        const data = await res.json().catch(() => null);
        if (!res.ok) throw new Error((data && data.error) ? data.error : `HTTP ${res.status}`);
        const s = data && data.stats || {};
        toast(`Логи загружены\nПолучено: ${s.received ?? 0}\nРаспарсено: ${s.parsed ?? 0}\nЗаписано: ${s.inserted ?? 0}`, 'success', 6000);
        refreshMap();
    } catch (e) {
        const interrupted = uploadInterruptedMessage(e);
        if (interrupted) toast(interrupted, 'warn');
        else toast('Ошибка загрузки логов: ' + e.message, 'error');
    } finally {
        markUploadEnd();
    }
}

async function uploadGeo() {
    const fi = document.getElementById('geoFile');
    const f = fi.files[0]; if (!f) return;
    toast(`Загрузка GeoIP: ${f.name}`, 'info', 2500);
    markUploadStart();
    let uploaded = false;
    try {
        const res = await fetch(`${API_BASE}/upload-geo`, {
            method: 'POST',
            credentials: 'same-origin',
            headers: nmAuthHeaders({ 'Content-Type': 'text/csv' }),
            body: f
        });
        const data = await res.json().catch(() => null);
        if (!res.ok) {
            const detail = (data && data.error) ? data.error : (`HTTP ${res.status}`);
            throw new Error(detail);
        }
        uploaded = true;
        const ranges = data && data.ranges ? data.ranges : '?';
        toast(`GeoIP загружена: ${ranges} диапазонов. Индекс обновляется (~1–3 мин)…`, 'success', 6000);
    } catch (e) {
        const interrupted = uploadInterruptedMessage(e);
        if (interrupted) toast(interrupted, 'warn');
        else toast('Ошибка обновления GeoIP: ' + e.message, 'error');
        return;
    } finally {
        // beforeunload только на время POST; ожидание индекса не блокирует уход со страницы.
        markUploadEnd();
    }
    if (!uploaded) return;
    // Backend перестраивает in-memory индекс в фоне; ждём готовности (можно уходить).
    for (let i = 0; i < 40; i++) {
        await new Promise(r => setTimeout(r, 5000));
        try {
            const h = await fetch(`${API_BASE}/api/health`, { credentials: 'same-origin' });
            if (h.ok && i >= 5) {
                refreshMap();
                return;
            }
        } catch (_) { /* backend может кратковременно перезапускаться */ }
    }
    refreshMap();
}

async function exportPNG() {
    try {
        if (!maplibreMap) throw new Error('View not initialized');
        // Composite MapLibre basemap + deck overlay canvases
        await new Promise(r => requestAnimationFrame(() => r()));
        const mapCanvas = maplibreMap.getCanvas();
        let deckCanvas = null;
        if (deckOverlay && typeof deckOverlay.getCanvas === 'function') {
            deckCanvas = deckOverlay.getCanvas();
        }
        if (!mapCanvas) throw new Error('Canvas not found');

        const w = mapCanvas.width;
        const h = mapCanvas.height;
        const out = document.createElement('canvas');
        out.width = w;
        out.height = h;
        const ctx = out.getContext('2d');
        ctx.fillStyle = mapBaseCss();
        ctx.fillRect(0, 0, w, h);
        ctx.drawImage(mapCanvas, 0, 0);
        if (deckCanvas && deckCanvas.width && deckCanvas.height) {
            ctx.drawImage(deckCanvas, 0, 0, w, h);
        }
        const dataURL = out.toDataURL('image/png');
        const a = document.createElement('a');
        a.href = dataURL;
        a.download = `geoatlas-${viewMode}-${new Date().toISOString().replace(/[:.]/g, '-')}.png`;
        a.click();
        toast('Снимок сохранён', 'success', 2500);
    } catch (e) {
        toast('Ошибка экспорта: ' + e.message, 'error');
    }
}
