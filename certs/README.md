# TLS-сертификаты (свои)

Положите сюда PEM-файлы:

| Файл | Назначение |
|------|------------|
| `fullchain.pem` | сертификат + цепочка (full chain) |
| `privkey.pem` | приватный ключ |

## Включение HTTPS

1. Скопируйте файлы в этот каталог (имена как выше, либо задайте `SSL_CERT_FILE` / `SSL_KEY_FILE` внутри контейнера), **или** загрузите PEM через веб-интерфейс: **Настройки → Доступ → HTTPS-сертификаты** (требуется том `./certs` в backend).
2. В `.env` (или окружении):

```env
HTTPS_ENABLED=1
HTTPS_PORT=443
HTTP_PORT=80
HTTP_REDIRECT=1
```

`HTTPS_ENABLED=auto` (по умолчанию в entrypoint) тоже включит HTTPS, если оба PEM на месте.

3. Перезапустите стек: `./start.sh` (или `docker compose … up -d` через `start.sh` / `ga_compose`, чтобы подхватился `docker-compose.https.yml`).

4. Откройте `https://<host>/`. При `HTTP_REDIRECT=1` HTTP перенаправляется на HTTPS.

## Замечания

- Не коммитьте ключи: `*.pem` в `.gitignore`.
- Самоподписанный сертификат браузер будет ругать — это нормально для lab/внутренней сети.
- Syslog (`:514`) по-прежнему без TLS — отдельно от UI HTTPS.
