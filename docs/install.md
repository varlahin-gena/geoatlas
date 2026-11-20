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
| 8080        | TCP      | Backend API             | Нет (только docker-сеть + nginx; не публиковать `ports`) |
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

Прод ставят только из архива **`geoatlas-X.Y.Z.tar.gz`** с [GitHub Releases](https://github.com/varlahin-gena/geoatlas/releases) (рядом лежит `.sha256` для проверки). В пакете — стек, единый установщик (`deploy/install.sh`) с OS-бэкендами Ubuntu и Oracle Linux / RHEL, `update.sh` и (в релизном tar.gz) каталог **`images/`** с Docker-образами. Это не `.deb`/`.rpm`: Docker и файрвол ставит установщик из пакета. Каталог установки по умолчанию — **`/opt/geoatlas`**.

| Задача                                | Что запускать                                              |
|---------------------------------------|------------------------------------------------------------|
| Первая установка (любая поддерживаемая ОС) | `deploy/install.sh` из распакованного пакета          |
| Обновление                            | `/opt/geoatlas/update.sh` + новый tar.gz                   |

Принудительное семейство ОС: `GA_FORCE_OS_FAMILY=debian|rhel`.

**Общий первый шаг на сервере** (Ubuntu и Oracle Linux одинаково):

```bash
VER=2.8.0   # или нужный релиз

cd /tmp
curl -fLO "https://github.com/varlahin-gena/geoatlas/releases/download/v${VER}/geoatlas-${VER}.tar.gz"
curl -fLO "https://github.com/varlahin-gena/geoatlas/releases/download/v${VER}/geoatlas-${VER}.tar.gz.sha256"
sha256sum -c "geoatlas-${VER}.tar.gz.sha256"
tar -xzf "geoatlas-${VER}.tar.gz"
cd "geoatlas-${VER}"
```

Дальше — `./deploy/install.sh` или `./update.sh` для уже стоящей системы ([обновление](operations.md#обновление-системы)).

После первого запуска: направьте syslog МСЭ на `:514` — [syslog.md](syslog.md); переменные `.env` — [configuration.md](configuration.md).

## Установщик (`deploy/install.sh`)

После шагов выше (пакет в `/tmp/geoatlas-${VER}`):

```bash
sudo ./deploy/install.sh
```

Скрипт определяет семейство ОС (`/etc/os-release`), подключает OS-бэкенд и общий оркестратор: ставит Docker, накладывает пакет в `/opt/geoatlas`, ставит **полный стек** (без выбора модулей), спрашивает доступ к сайту, режим производительности и пароль admin, настраивает сетевой экран и запускает систему.

Диалоги установки и удаления — **TUI** (`whiptail` → `dialog` → текст). Долгие шаги показывают **gauge**.

**Pipeline:**

1. Подготовка системы (пакеты ОС и Docker)
2. Наложение пакета в `/opt/geoatlas`
3. Доступ к сайту (HTTP или HTTPS, порты, сертификаты)
4. Режим производительности
5. Пароль администратора
6. Сетевой экран (открытие портов; на Ubuntu отдельно — включить экран)
7. Итог и запуск (`./start.sh`, можно отказаться)

**Полный стек** (всегда): ClickHouse, Backend, Frontend, UI-авторизация, API Bearer, syslog-ng (`:514`), stats-collector, репутация IP, Dozzle. Для CI тихий override: `GA_MODULES` / `GA_ENABLE_*`.

**OS-бэкенды:**

| Семейство | ОС | Пакеты / Docker | Сетевой экран | Дополнительно |
|-----------|----|-----------------|---------------|---------------|
| `debian` | Ubuntu, Debian, Mint, Pop | apt + Docker CE | UFW (`ENABLE_UFW`, `UFW_AUTO_ENABLE`) | — |
| `rhel` | Oracle Linux, RHEL, Rocky, Alma, CentOS | dnf + Docker CE | firewalld (`ENABLE_FIREWALL`) | снятие podman/buildah; SELinux (`DISABLE_SELINUX`) |

Принудительно: `GA_FORCE_OS_FAMILY=debian|rhel`.

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
