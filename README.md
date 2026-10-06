# We1BBoard

Панель управления **Xray-core** для **Linux (Ubuntu/Debian)**: протоколы как у 3x-ui, мосты entry→exit на любом Xray outbound, tg-web-proxy, install/CLI, 3 темы UI.

Архитектурно опираемся на опыт 3x-ui (DB → config render → child process / gRPC, Runtime Local|Remote), без копирования её кода.

## Целевая платформа

- Production: **Ubuntu / Debian** (systemd + `scripts/install.sh`)
- Windows — только для локальной разработки UI/API, не для деплоя

## Docker — нужно ли?

**Основной путь — без Docker**, как у 3x-ui: бинарник + systemd + бинарники xray/mtg рядом.

Почему Docker не обязателен (и часто мешает):
- Xray **TUN**, raw sockets, `mtg`, `tproxy-server` удобнее на хосте
- Мосты RU↔EU и multi-node проще с host networking / реальными IP
- Обновление xray отдельно от панели — привычная схема 3x-ui

Docker имеет смысл только если нужна одинаковая упаковка для CI/лаб; для боевых VPS лучше install.sh.

## Стек

- Backend: Go (Gin, GORM, SQLite)
- Frontend: React + Vite
- Engines: Xray-core, mtg, tuic, hysteria2, tproxy-server

## Быстрый старт (dev)

```bash
cd frontend && npm install && npm run build
go build -o we1bboard ./cmd/we1bboard
./we1bboard run
```

Панель: `http://127.0.0.1:2053/we1b/` — `admin` / `admin`  
Бинарники: `/etc/we1bboard/bin/` или `WE1B_BIN_DIR`.

## Обновления и откат (как в 3x-ui)

```bash
# Управление
we1bboard-ctl                 # меню
we1bboard-ctl update          # latest release
we1bboard-ctl legacy          # поставить конкретный tag (v1.0.0)
we1bboard-ctl rollback        # быстрый откат на предыдущий бинарник

# Или напрямую
bash <(curl -fsSL https://raw.githubusercontent.com/WeIbSchatten/We1BBoard/main/scripts/update.sh)
bash <(curl -fsSL https://raw.githubusercontent.com/WeIbSchatten/We1BBoard/main/scripts/update.sh) v1.0.0
bash <(curl -fsSL https://raw.githubusercontent.com/WeIbSchatten/We1BBoard/main/scripts/update.sh) rollback
```

Перед обновлением текущий бинарник копируется в `we1bboard.prev` и в `/usr/local/we1bboard/backups/`.  
Данные (`/etc/we1bboard`) не трогаются.

Установка конкретной версии:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/WeIbSchatten/We1BBoard/v1.0.0/scripts/install.sh) v1.0.0
```

Релизы собираются GitHub Actions по тегу `v*` (linux amd64/arm64).

## Install (Ubuntu/Debian)


```bash
bash scripts/install.sh
# or non-menu:
bash scripts/install.sh install
```

Интерактивный выбор (как в 3x-ui):

1. **Database** — SQLite или PostgreSQL (DSN)
2. **Panel** — порт, path, admin, тема
3. **SSL**
   - Let's Encrypt **domain** (90 дней, auto-renew)
   - Let's Encrypt **IP** shortlived (~6 дней, auto-renew)
   - **Custom** пути к cert/key
   - **Skip** (HTTP за reverse proxy)

Автопродление: `acme.sh` cron + `--reloadcmd "systemctl restart we1bboard"`.

После установки в меню: пункт **8. SSL certificate setup**, **9. Change database**.

CLI:

```bash
we1bboard cert -webCert /root/cert/example.com/fullchain.pem -webCertKey /root/cert/example.com/privkey.pem
systemctl restart we1bboard
```

## База данных

Как в 3x-ui — SQLite по умолчанию, PostgreSQL опционально:

```bash
# SQLite (default)
export WE1B_DATA_DIR=/etc/we1bboard

# PostgreSQL
export WE1B_DB_TYPE=postgres
export WE1B_DB_DSN='host=127.0.0.1 user=we1b password=secret dbname=we1bboard port=5432 sslmode=disable'
# optional pool:
# WE1B_DB_MAX_OPEN_CONNS=25
# WE1B_DB_MAX_IDLE_CONNS=25
```

## Мосты (proxy chaining)

На entry-ноде создаётся Xray **outbound** к exit-ноде на любом протоколе:
`vless` / `vmess` / `trojan` / `shadowsocks` / `socks` / `http` / `wireguard`  
(+ network/security: tcp/ws/grpc/… + none/tls/reality).  
Либо полный JSON `dialerSettings` / `dialerStreamSettings` (как settings в 3x-ui).

## CLI

```bash
we1bboard          # меню
we1bboard run
we1bboard setting set panelPort 2053
we1bboard cert -webCert /path/fullchain.pem -webCertKey /path/privkey.pem
we1bboard reset-admin admin 'newpass'
```

## Env

| Variable | Default |
|----------|---------|
| `WE1B_DATA_DIR` | `/etc/we1bboard` (prod) / `~/.we1bboard` |
| `WE1B_DB_TYPE` | `sqlite` \| `postgres` |
| `WE1B_DB_DSN` | PostgreSQL DSN (required if postgres) |
| `WE1B_DB_PATH` | `$DATA/we1bboard.db` (sqlite) |
| `WE1B_BIN_DIR` | `$DATA/bin` |
| `WE1B_XRAY_BIN` | `$BIN/xray` |
| `WE1B_SSL_MODE` | `domain` \| `ip` \| `custom` \| `none` (noninteractive) |

## Лицензия

MIT
