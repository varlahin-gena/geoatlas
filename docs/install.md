# Установка и удаление

Каталог установки по умолчанию: **`/opt/geoatlas`**.

## Требования

### Аппаратные (минимум / рекомендуется)

| Ресурс   | Минимум              | Рекомендуется         |
|----------|----------------------|-----------------------|
| CPU      | 2 ядра               | 4–8 ядер              |
| RAM      | 4 GiB                | 8–16 GiB              |
| Диск     | 20 GiB свободно      | 50+ GiB (логи, CH)    |

При установке скрипт автоматически определяет ресурсы и предлагает профиль производительности (см. [Профили производительности](operations.md#профили-производительности)).

### Программные

- **ОС**: Linux (Ubuntu 20.04+, Oracle Linux 8+, Rocky/Alma/RHEL/CentOS)
- **Docker Engine** 24+ с плагином **docker compose**
- Открытые порты **80** (UI HTTP), при HTTPS — ещё **443**, и **514/tcp**, **514/udp** (syslog)
- Доступ к `/proc` и `/sys/fs/cgroup` хоста (нужен `stats-collector`)

### Сетевые порты

| Порт        | Протокол | Назначение              | Доступ снаружи |
|-------------|----------|-------------------------|----------------|
| 80          | TCP      | Веб-интерфейс (HTTP)    | Да             |
| 443         | TCP      | Веб-интерфейс (HTTPS)   | Опционально    |
| 514         | TCP/UDP  | Syslog от МСЭ           | Да             |
| 8080        | TCP      | Backend API             | Нет (docker)   |
| 1514        | TCP      | Ingest от syslog-ng     | Нет (docker)   |
| 8123 / 9000 | TCP      | ClickHouse HTTP/native  | Нет (docker)   |

### HTTPS (свои сертификаты)

Положите PEM в `certs/fullchain.pem` и `certs/privkey.pem` (см. `certs/README.md`).

| Переменная     | По умолчанию | Назначение                                                      |
|----------------|--------------|-----------------------------------------------------------------|
| `HTTPS_ENABLED`| `auto`       | `1`/`true` — вкл.; `0` — выкл.; `auto` — вкл. если есть оба PEM |
| `HTTPS_PORT`   | `443`        | Хостовый порт TLS                                               |
| `HTTP_REDIRECT`| `1`          | Редирект HTTP→HTTPS                                             |
| `HTTP_PORT`    | `80`         | HTTP (и редирект)                                               |

Установщик (Ubuntu / Oracle Linux) спрашивает HTTPS в пошаговом режиме (при TTY).

## Установочный пакет

Один архив **`geoatlas-X.Y.Z.tar.gz`** (плюс `.sha256`; SBOM `.cdx.json` / `.spdx.json` — для аудита, не для установки) с [GitHub Releases](https://github.com/varlahin-gena/geoatlas/releases) — исходники стека, **оба** установщика (`deploy/ubuntu/…`, `deploy/oracle_linux/…`), `update.sh` и (для прод-пакета) каталог **`images/`** с готовыми Docker-образами. Это не `.deb`/`.rpm`: Docker и файрвол хоста ставит OS-скрипт из пакета.

| Задача                                | Что запускать                                              |
|---------------------------------------|------------------------------------------------------------|
| Первая установка, Ubuntu              | `deploy/ubuntu/install_ubuntu.sh` из распакованного пакета |
| Первая установка, Oracle Linux / RHEL | `deploy/oracle_linux/install_oraclelinux.sh` из пакета     |
| Обновление                            | `/opt/geoatlas/update.sh` + новый tar.gz                   |

**Общий первый шаг на сервере** (Ubuntu и Oracle Linux одинаково):

```bash
VER=2.1.3   # или нужный релиз
cd /tmp
curl -fLO "https://github.com/varlahin-gena/geoatlas/releases/download/v${VER}/geoatlas-${VER}.tar.gz"
curl -fLO "https://github.com/varlahin-gena/geoatlas/releases/download/v${VER}/geoatlas-${VER}.tar.gz.sha256"
sha256sum -c "geoatlas-${VER}.tar.gz.sha256"
tar -xzf "geoatlas-${VER}.tar.gz"
cd "geoatlas-${VER}"
```

Дальше — установщик вашей ОС (см. ниже) или `./update.sh` для уже стоящей системы ([обновление](operations.md#обновление-системы)).

После первого запуска: направьте syslog МСЭ на `:514` — [syslog.md](syslog.md); переменные `.env` — [configuration.md](configuration.md).

## Ubuntu

После шагов выше (пакет в `/tmp/geoatlas-${VER}`):

```bash
sudo ./deploy/ubuntu/install_ubuntu.sh
```

Скрипт устанавливает Docker, накладывает пакет в `/opt/geoatlas`, **интерактивно предлагает модули** и профиль производительности, настраивает UFW и запускает стек.

Диалоги установки и удаления — **TUI** (`whiptail` → `dialog` → текст). Долгие шаги (apt/Docker) показывают **gauge**.

**Что делает скрипт:**

1. Обновляет списки пакетов
2. Устанавливает `curl`, `ufw`, `whiptail` (опционально `dialog`)
3. Устанавливает Docker Engine и compose plugin (если ещё нет)
4. Накладывает пакет из текущего каталога в `/opt/geoatlas`
5. Спрашивает, какие модули ставить (checklist: авторизация, API-токен, syslog-ng, stats-collector, репутация IP, Dozzle)
6. Спрашивает **HTTPS** (свои PEM; можно оставить только HTTP)
7. Спрашивает **порт(ы)**: при HTTPS — порт TLS, затем HTTP (редирект); при HTTP-only — порт UI (80 / 8080 или свой)
8. Запускает детектор ресурсов и предлагает профиль
9. Настраивает UFW (HTTP, при HTTPS — TLS-порт, и при необходимости 514)
10. Вызывает `./start.sh` (можно отказаться на последнем шаге)

**Выбор модулей (интерактивно или через env):**

| Модуль           | По умолчанию | Что даёт                                                                        |
|------------------|--------------|---------------------------------------------------------------------------------|
| UI-авторизация   | вкл.         | логин, роли admin/operator/dashboard (`AUTH_DISABLED` при отказе)               |
| API Bearer-токен | вкл.         | защита мутирующих API (`API_AUTH_DISABLED` при отказе)                          |
| syslog-ng        | вкл.         | приём syslog на `:514` (Compose profile `syslog`)                               |
| stats-collector  | вкл.         | метрики / `/system` (Compose profile `stats`)                                   |
| Репутация IP     | вкл.         | модуль целиком; при отказе `REPUTATION_FETCH_ENABLED=false` (API/UI/фиды выкл.) |
| Dozzle           | вкл.         | realtime-логи UI `/dozzle/` (profile `dozzle`; `docker.sock` + start/stop/restart) |

Ядро (ClickHouse + Backend + Frontend) ставится всегда.

## Oracle Linux / RHEL

Поддерживаются Oracle Linux, RHEL, Rocky Linux, AlmaLinux, CentOS. **Тот же** `geoatlas-X.Y.Z.tar.gz`, что для Ubuntu.

После распаковки пакета на сервере:

```bash
sudo ./deploy/oracle_linux/install_oraclelinux.sh
```

**Что делает скрипт:**

1. Удаляет конфликтующие пакеты (`podman`, `buildah`, `runc`) при необходимости
2. Устанавливает Docker CE из официального репозитория
3. Настраивает SELinux (`container_manage_cgroup`) или переводит в permissive (опционально)
4. Накладывает пакет в `/opt/geoatlas`, предлагает модули, HTTPS, HTTP-порт, профиль, настраивает firewalld и запускает стек

## Ручная установка из пакета

Если нужны те же шаги без TUI (Docker уже установлен):

```bash
# Пакет уже скачан и распакован (см. «Установочный пакет»)
sudo mkdir -p /opt
sudo cp -a "$PWD" /opt/geoatlas
cd /opt/geoatlas

chmod +x start.sh stop.sh update.sh scripts/tune-resources.sh \
  deploy/common/detect_resources.sh deploy/common/select_modules.sh deploy/common/ui.sh \
  deploy/common/admin_auth.sh

# (Рекомендуется) модули и профиль
./deploy/common/select_modules.sh .
./scripts/tune-resources.sh
# или неинтерактивно:
# GA_ENABLE_AUTH=0 GA_AUTO_MODULES=1 ./deploy/common/select_modules.sh .
# GA_AUTO_PROFILE=1 ./deploy/common/detect_resources.sh .

./start.sh
```

Открыть порты вручную:

```bash
# Ubuntu (UFW)
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp   # если HTTPS
sudo ufw allow 514/tcp
sudo ufw allow 514/udp

# Oracle Linux / RHEL (firewalld)
sudo firewall-cmd --permanent --add-port=80/tcp
sudo firewall-cmd --permanent --add-port=443/tcp   # если HTTPS
sudo firewall-cmd --permanent --add-port=514/tcp
sudo firewall-cmd --permanent --add-port=514/udp
sudo firewall-cmd --reload
```

## Удаление

### Быстрый старт (автоопределение ОС)

```bash
cd /opt/geoatlas   # или каталог, откуда запускали скрипт
sudo bash deploy/uninstall.sh
```

Скрипт покажет **аудит** (контейнеры, volumes, размер каталога) и предложит **интерактивное меню** (whiptail / dialog):
1. Безопасное удаление — stop + файлы + firewall, данные ClickHouse сохраняются
2. Полное удаление (purge) — включая volumes и образы
3. Только остановить стек
4. Настроить вручную

Долгие шаги (compose down, rm, firewall) идут через **gauge**. Бэкенд диалогов тот же, что при установке (`GA_UI`).

### Ubuntu / Debian

```bash
sudo bash deploy/ubuntu/uninstall_ubuntu.sh
```

### Oracle Linux / RHEL

```bash
sudo bash deploy/oracle_linux/uninstall_oraclelinux.sh
```

### Остановка без удаления проекта

```bash
cd /opt/geoatlas
./stop.sh
```
