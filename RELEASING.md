# Releasing ГеоАтлас

Три оси версий **не сливаются** в одну цифру. Релиз обязан назвать, какая тройка в теге.

| Ось | Где | Когда двигать |
|-----|-----|----------------|
| Продукт | `VERSION`, git tag `vX.Y.Z`, `install-meta.json` | Только в коммите релиза. На сервере UI показывает версию из пакета (`vX.Y.Z`). |
| HTTP API | `openapi.yaml` → `info.version` | Новый путь / поле / breaking schema. Не за лицензию, баг парсера или описание. |
| Схема CH | `nm_schema_version` (`Ensure*`) | Как сейчас, независимо. В Notes релиза — строка, если Ensure* менялся. |

CI: `bash scripts/check-release-contract.sh` (job **release-contract**). Инвариант: если OpenAPI на дереве новее, чем в Notes секции текущего `VERSION`, блок **Unreleased** обязан содержать эту цифру. README цитирует `OpenAPI **N**` = `info.version`.

На push тега `v*` тот же скрипт сверяет `v`+`VERSION` и что Notes уже догнали OpenAPI (Unreleased перенесён).

## Когда какой bump продукта

| Вид | Пример | Тег |
|-----|--------|-----|
| Патч | баг, парсер по семплу, security, доки | `1.4.x` |
| Минор | заметное поведение для оператора | `1.5.0` |
| Мажор | осознанно ломаем install или HTTP API | `2.0.0` |

Ритм: не чаще одного продуктового релиза в **2 недели**, лучше 2–4. Несколько тегов в один день — только security. Между релизами копим Unreleased, не клеим тег на каждый коммит.

OpenAPI: минор (`1.8` → `1.9`) — новый endpoint или новое поле; патч (`1.8.0` → `1.8.1`) — изменение уже описанной схемы, которое видит клиент. Опечатка в description — номер не трогать.

## Чеклист перед тегом

1. Перенести `## [Unreleased]` в `## [X.Y.Z] — YYYY-MM-DD`.
2. Notes секции: `OpenAPI API doc version: **N**` (= `openapi.yaml`); схема CH, если менялась; продуктовая версия.
3. `VERSION` = `X.Y.Z`. Ссылка `[X.Y.Z]:` внизу CHANGELOG. Unreleased снова пустой (только заголовок).
4. CI зелёный, в т.ч. **release-contract**.
5. Локальный smoke после `./start.sh`:
   - `/api/live`, `/api/ready`
   - логин, карта, `/system`
   - API-токен scope=ops, `curl -H "Authorization: Bearer …" /api/ingest/stats`
6. (Опционально) `./scripts/watch-ingest.sh` без drops в покое.
7. (Опционально) `bash scripts/pack-release.sh` — локально проверить `dist/geoatlas-X.Y.Z.tar.gz`.

## Создать релиз

```bash
# working tree clean, main запушен, VERSION уже X.Y.Z
git tag -a "v$(tr -d '[:space:]' < VERSION)" -m "ГеоАтлас v$(tr -d '[:space:]' < VERSION)"
git push origin "v$(tr -d '[:space:]' < VERSION)"
```

Не тегировать раньше секции в CHANGELOG: job на `v*` упадёт, если OpenAPI ещё только в Unreleased.

На тег CI сам:

1. проверяет контракт (`VERSION` / CHANGELOG / OpenAPI);
2. создаёт GitHub Release из секции CHANGELOG (если релиза ещё нет);
3. собирает **`geoatlas-X.Y.Z.tar.gz`** (+ `.sha256`) и прикрепляет к релизу;
4. сканирует архив Syft и прикрепляет **`geoatlas-X.Y.Z.cdx.json`** (CycloneDX) и **`.spdx.json`** (SPDX).

Ручной `gh release create` не нужен, пока job **github-release** зелёный. Один tar.gz — и для Ubuntu, и для Oracle Linux; операторы на сервере **только** скачивают пакет и ставят/обновляют через install-скрипт или `./update.sh` (README, «Установка» / «Обновление системы»; без `git pull` на сервере).

Проверьте Assets релиза: `geoatlas-X.Y.Z.tar.gz`, `.sha256`, `.cdx.json`, `.spdx.json`.

## После релиза

- Установщик / README — только если менялся UX установки. Проверьте, что в Assets релиза есть tar.gz.
- Следующие коммиты **не** двигают `VERSION`, пока не решите релизить снова. Новое — в Unreleased (и bump `info.version`, если менялся HTTP-контракт).
