# Releasing ГеоАтлас

## Версии

| Что | Где | Пример |
|-----|-----|--------|
| Продукт | `VERSION`, git tag `vX.Y.Z` | `1.1.4` |
| HTTP API (OpenAPI) | `openapi.yaml` → `info.version` | `1.3.0` |
| Схема CH | `nm_schema_version` (Go Ensure*) | независимо |

## Чеклист перед тегом

1. CI зелёный на `main` (backend test+lint, CH integration, frontend smoke).
2. `VERSION` и секция в `CHANGELOG.md` совпадают с тегом.
3. Локальный smoke после `./start.sh`:
   - `/api/health`
   - логин admin, карта, `/system.html`
   - создать API-токен scope=ops, `curl -H "Authorization: Bearer …" /api/ingest/stats`
4. (Опционально) `./scripts/watch-ingest.sh` без drops в покое.

## Создать релиз

```bash
# убедиться что working tree clean и main запушен
git tag -a v1.1.4 -m "ГеоАтлас v1.1.4"
git push origin v1.1.4

gh release create v1.1.4 --title "v1.1.4" --notes-file /tmp/release-notes-1.1.4.md
# или вручную на GitHub → Releases, вставить секцию из CHANGELOG
```

## После релиза

- Обновить install-скрипты / README только если меняется UX установки.
- Следующая версия: правка `VERSION` + новая секция в `CHANGELOG.md` в отдельном PR/коммите.
