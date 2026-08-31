# GeoIP

GeoIP-база загружается через веб-интерфейс (кнопка загрузки на главной странице) в виде CSV
или пополняется по одному диапазону со страницы **IP без GeoIP** (`/geo-missing`).
Поддерживается формат SIEM KUMA.

**Формат CSV:**

```csv
Network,Country,Region,City,Latitude,Longitude
10.0.0.0-10.0.255.254,Россия,OOO_Company,Москва,55.76167,37.60667
192.168.1.0-192.168.1.255,Россия,LAN,Office,55.75,37.62
```

На странице `/geo-missing` для IP без координат доступна кнопка **«добавить в базу»**
(форма с полями шаблона выше) и **«Выгрузить GeoIP CSV»** — скачивание актуальной базы.

Без GeoIP-базы узлы на карте не отображаются (нет координат).

## Загрузка GeoIP с сервера (рекомендуется для больших CSV)

Большие базы (сотни МБ) через браузер часто упираются в ширину канала между рабочей станцией и сервером.
**Не переключайте страницу/вкладку во время upload** — браузер оборвёт запрос (`Failed to fetch`).
Надёжнее положить CSV на хост и залить через API **с самого сервера** (localhost → nginx → backend).

Нужен Bearer с scope **ops** или **admin**: токен из контейнера `backend` (ниже) или именованный токен из UI `/api-tokens`. Токен берите через `docker exec backend` — так вы гарантированно совпадаете с тем, что реально проверяет API (не зависит от `source .env` / `COMPOSE_PROJECT_NAME`).

**1. Скопировать CSV на сервер** (с рабочей станции):

```bash
scp geoip.csv root@сервер:/opt/geoatlas/geoip.csv
```

**2. На сервере — токен из живого backend:**

```bash
cd /opt/geoatlas
# контейнер должен быть Up (healthy): docker ps --filter name=^backend$
TOKEN="$(docker exec backend printenv API_AUTH_TOKEN)"
echo -n "$TOKEN" | wc -c   # ожидаете > 0 (обычно 32+)
```

**3. Проверить авторизацию** (ожидаете HTTP `200`):

```bash
# если UI не на 80-м порту — подставьте HTTP_PORT из .env (например :8080)
curl -sS -o /tmp/auth_body -w "HTTP %{http_code}\n" \
  -H "Authorization: Bearer $TOKEN" \
  "http://127.0.0.1/api/auth/check-ops"
cat /tmp/auth_body
```

При **401** токен пустой/неверный или UI на другом порту. Не продолжайте upload, пока `check-ops` не вернёт 200.

**4. (Опционально) dry-run без записи** — отловит пересечения диапазонов и лимиты:

```bash
curl -sS -w "\nHTTP %{http_code}\n" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: text/csv" \
  --data-binary @/opt/geoatlas/geoip.csv \
  "http://127.0.0.1/upload-geo?dry_run=1"
```

**5. Залить CSV:**

```bash
curl -sS -w "\nHTTP %{http_code}\n" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: text/csv" \
  --data-binary @/opt/geoatlas/geoip.csv \
  "http://127.0.0.1/upload-geo"
```

Ожидаете JSON `{"ok":true,"ranges":N}` и HTTP `200`.

**6. Проверить результат:**

```bash
docker exec clickhouse sh -c \
  'clickhouse-client --password "$CLICKHOUSE_PASSWORD" -q "SELECT count() FROM geo_ranges"'
docker logs backend --since=10m 2>&1 | grep -iE 'geo index loaded|geo csv|upload|overlap|error'
```

В логах — `geo index loaded`. На большой базе backend 1–3 минуты грузит CPU/RAM (парсинг и in-memory индекс) — это нормально.

**Типичные ответы upload:**

| HTTP | Причина | Что делать |
|------|---------|------------|
| **401** | нет/неверный Bearer | шаг 2–3; токен только из `docker exec backend` |
| **400** `overlapping geo ranges` | в CSV пересекаются сети (границы включительные) | править CSV, снова `dry_run=1` |
| **409** | опасный replace / мало RAM / занят heavy-job | см. ниже: очистить базу или не перезаливать |
| **413** | файл или число ranges сверх лимита | уменьшить CSV / `GEOIP_UPLOAD_MAX_*` / профиль памяти |

Если nginx снова отдаёт 401, а `docker exec backend` токен верный — заливка мимо nginx (прямо в процесс backend):

```bash
TOKEN="$(docker exec backend printenv API_AUTH_TOKEN)"
docker cp /opt/geoatlas/geoip.csv backend:/tmp/geoip.csv
docker exec backend wget -qO- \
  --header="Authorization: Bearer $TOKEN" \
  --header="Content-Type: text/csv" \
  --post-file=/tmp/geoip.csv \
  http://127.0.0.1:8080/upload-geo
```

## Повторная загрузка большой GeoIP и HTTP 502 / OOM

Индекс GeoIP целиком держится в RAM backend. Повторный upload того же большого CSV (миллионы диапазонов), когда индекс уже загружен, снова парсит файл в память **поверх** существующего индекса → пик RAM удваивается → Docker cgroup может убить процесс (`oom-kill` / `Memory cgroup out of memory`). Снаружи это часто выглядит как **HTTP 502** в UI, а контейнер `backend` ненадолго перезапускается.

**Сервер теперь режет опасные upload до OOM:**

| Ограничение                     | Env                                                  | Откуда дефолт                                                                        |
|---------------------------------|------------------------------------------------------|--------------------------------------------------------------------------------------|
| Размер тела `/upload-geo`       | `GEOIP_UPLOAD_MAX_BYTES` (или `MAX_GEO_UPLOAD_SIZE`) | `install-profile.json` → `limits.backend.memory_gb` (small≈512 MiB, medium≈1 GiB, …) |
| Число диапазонов в CSV          | `GEOIP_UPLOAD_MAX_RANGES`                            | тот же профиль (small≈4 M)                                                           |
| Replace поверх крупного индекса | —                                                    | **409 до чтения body**, если индекс уже ≥ половины лимита ranges                     |
| Soft mem headroom               | из `install-profile` `limits.backend.memory_gb` × ¾  | **409 после parse**, если `HeapAlloc + upload ≈` выше soft limit (запас 512 MiB)     |

Ответы: **413** (файл/число ranges слишком велики), **409** (опасный replace / нехватка headroom / занят heavy-job слот).

Ход загрузки смотрите в логах: `geo upload start` → `geo csv parsed` / `geo upload early reject` / `geo upload rejected …`. Точечная правка одной строки — страница **База GeoIP** (`/geo-ranges`), не полный CSV.
