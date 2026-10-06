# We1BBoard

Панель управления **Xray-core** для **Linux (Ubuntu/Debian)**: протоколы как у 3x-ui, мосты entry→exit на любом Xray outbound, tg-web-proxy, install/CLI, 3 темы UI.

Архитектурно опираемся на опыт 3x-ui (DB → config render → child process / gRPC, Runtime Local|Remote), без копирования её кода.

## Установка (одной командой)

Как у 3x-ui — root на сервере:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/WeIbSchatten/We1BBoard/main/install.sh)
```

Конкретная версия:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/WeIbSchatten/We1BBoard/main/install.sh) v1.0.0
```

После установки управление:

```bash
we1bboard                 # меню
we1bboard start|stop|restart|status
we1bboard update          # latest release
we1bboard legacy          # конкретный tag
we1bboard rollback        # откат на предыдущий бинарник
we1bboard ssl             # сертификаты
```

Интерактивный выбор при установке: БД (SQLite/PostgreSQL), порт/path/admin, SSL (domain / IP / custom / skip).  
Автопродление: `acme.sh` cron + restart сервиса.

Без TTY / cloud-init: `WE1B_NONINTERACTIVE=1` (или `curl … | bash`) — без вопросов, SSL по умолчанию skip (`WE1B_SSL_MODE=domain|ip|custom|none`).

## Целевая платформа

- Production: **Ubuntu / Debian** (systemd)
- Windows — только для локальной разработки UI/API, не для деплоя

## Docker — нужно ли?

**Основной путь — без Docker**, как у 3x-ui: бинарник + systemd + бинарники xray/mtg рядом.

## Стек

- Backend: Go (Gin, GORM, SQLite / PostgreSQL)
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

## Обновления и откат

```bash
we1bboard update
we1bboard legacy          # tag вручную
we1bboard rollback

# или напрямую
bash <(curl -fsSL https://raw.githubusercontent.com/WeIbSchatten/We1BBoard/main/scripts/update.sh)
bash <(curl -fsSL https://raw.githubusercontent.com/WeIbSchatten/We1BBoard/main/scripts/update.sh) v1.0.0
bash <(curl -fsSL https://raw.githubusercontent.com/WeIbSchatten/We1BBoard/main/scripts/update.sh) rollback
```

Перед обновлением текущий бинарник копируется в `we1bboard.prev` и в `/usr/local/we1bboard/backups/`.  
Данные (`/etc/we1bboard`) не трогаются.  
Релизы: GitHub Actions по тегу `v*` (linux amd64/arm64).

## База данных

```bash
# SQLite (default)
export WE1B_DATA_DIR=/etc/we1bboard

# PostgreSQL
export WE1B_DB_TYPE=postgres
export WE1B_DB_DSN='host=127.0.0.1 user=we1b password=secret dbname=we1bboard port=5432 sslmode=disable'
```

## Мосты (proxy chaining)

На entry-ноде создаётся Xray **outbound** к exit-ноде на любом протоколе:
`vless` / `vmess` / `trojan` / `shadowsocks` / `socks` / `http` / `wireguard`  
(+ network/security: tcp/ws/grpc/… + none/tls/reality).

## CLI панели

```bash
/usr/local/we1bboard/we1bboard run
/usr/local/we1bboard/we1bboard setting set panelPort 2053
/usr/local/we1bboard/we1bboard cert -webCert /path/fullchain.pem -webCertKey /path/privkey.pem
/usr/local/we1bboard/we1bboard reset-admin admin 'newpass'
```

(Команда `we1bboard` в PATH — это меню управления, как `x-ui` у 3x-ui.)

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
| `WE1B_NONINTERACTIVE` | `1` — без вопросов |

## Лицензия

MIT
