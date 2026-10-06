#!/usr/bin/env bash
set -euo pipefail

# We1BBoard interactive installer for Ubuntu/Debian (3x-ui style)
#
# One command (recommended):
#   bash <(curl -Ls https://raw.githubusercontent.com/WeIbSchatten/We1BBoard/main/install.sh)
# Specific version:
#   bash <(curl -Ls https://raw.githubusercontent.com/WeIbSchatten/We1BBoard/main/install.sh) v1.0.0
# After install:
#   we1bboard

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

APP_NAME="we1bboard"
INSTALL_DIR="/usr/local/we1bboard"
PANEL_BIN="${INSTALL_DIR}/we1bboard"
BIN_PATH="${PANEL_BIN}"
MGMT_BIN="/usr/bin/we1bboard"
SERVICE_PATH="/etc/systemd/system/${APP_NAME}.service"
ENV_FILE="/etc/default/we1bboard"
DATA_DIR="/etc/we1bboard"
CERT_ROOT="/root/cert"
REPO_RELEASE_BASE="${WE1B_RELEASE_BASE:-https://github.com/WeIbSchatten/We1BBoard/releases/latest/download}"
REPO="WeIbSchatten/We1BBoard"
RAW_BASE="https://raw.githubusercontent.com/${REPO}/main"

# Non-interactive: WE1B_NONINTERACTIVE=1 or stdin is not a TTY (curl | bash, cloud-init)
if [[ "${WE1B_NONINTERACTIVE:-0}" == "1" ]] || [[ ! -t 0 ]]; then
  NONINTERACTIVE=1
else
  NONINTERACTIVE=0
fi

# Default: install immediately (like 3x-ui). Use "menu" for the control menu.
INSTALL_MODE="install"
TARGET_TAG=""
ARG1="${1:-}"
case "${ARG1}" in
  --install|install)
    INSTALL_MODE="install"
    TARGET_TAG="${2:-}"
    ;;
  menu|--menu)
    INSTALL_MODE="menu"
    ;;
  ssl|--ssl)
    INSTALL_MODE="ssl"
    ;;
  -h|--help|help)
    cat <<EOF
We1BBoard installer

  bash <(curl -Ls https://raw.githubusercontent.com/${REPO}/main/install.sh)
  bash <(curl -Ls https://raw.githubusercontent.com/${REPO}/main/install.sh) v1.0.0

  Arguments:
    (none) / install   Install or reinstall panel
    vX.Y.Z             Install specific release tag
    menu               Open management menu
    ssl                SSL certificate setup only
EOF
    exit 0
    ;;
  v*)
    INSTALL_MODE="install"
    TARGET_TAG="${ARG1}"
    ;;
  "")
    INSTALL_MODE="install"
    ;;
  *)
    if [[ "${ARG1}" =~ ^[0-9]+\. ]]; then
      INSTALL_MODE="install"
      TARGET_TAG="v${ARG1}"
    else
      err() { echo "$*" >&2; }
      echo "Unknown argument: ${ARG1} (try --help)" >&2
      exit 1
    fi
    ;;
esac

SSL_HOST=""
SSL_SCHEME="http"
PANEL_PORT="2053"
PANEL_PATH="/we1b/"

log() { echo -e "${GREEN}[We1BBoard]${NC} $*"; }
warn() { echo -e "${YELLOW}[warn]${NC} $*"; }
err() { echo -e "${RED}[error]${NC} $*" >&2; }
step() { echo -e "\n${BLUE}==> Step $1:${NC} $2"; }

banner() {
  echo -e "${BLUE}"
  cat <<'EOF'
 __          __ ___ ____  ____                      _
 \ \        / /__ \  _ \|  _ \                    | |
  \ \  /\  / /   ) |_) | |_) | ___   __ _ _ __ __| |
   \ \/  \/ /   / /|  _ <|  _ < / _ \ / _` | '__/ _` |
    \  /\  /   / /_| |_) | |_) | (_) | (_| | | | (_| |
     \/  \/   |____|____/|____/ \___/ \__,_|_|  \__,_|
EOF
  echo -e "${NC}"
  echo -e "  ${GREEN}Xray panel installer${NC}  ·  Ubuntu / Debian"
  echo
}

need_root() {
  if [[ ${EUID} -ne 0 ]]; then
    err "Run as root (sudo -i or: curl ... | sudo bash)"
    exit 1
  fi
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) err "unsupported arch"; exit 1 ;;
  esac
}

get_server_ip() {
  local ip=""
  ip="$(curl -4 -fsSL --max-time 5 https://api.ipify.org 2>/dev/null || true)"
  if [[ -z "${ip}" ]]; then
    ip="$(curl -4 -fsSL --max-time 5 https://ifconfig.me 2>/dev/null || true)"
  fi
  if [[ -z "${ip}" ]]; then
    ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
  fi
  echo "${ip}"
}

is_ipv4() {
  [[ "$1" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]
}

is_domain() {
  [[ "$1" =~ ^([A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?\.)+[A-Za-z]{2,}$ ]]
}

prompt() {
  local __var="$1" __msg="$2" __def="${3:-}"
  local __input=""
  if [[ "${NONINTERACTIVE}" == "1" ]]; then
    printf -v "${__var}" '%s' "${__def}"
    return
  fi
  if [[ -n "${__def}" ]]; then
    read -r -p "${__msg} [${__def}]: " __input || true
    __input="${__input:-${__def}}"
  else
    read -r -p "${__msg}: " __input || true
  fi
  printf -v "${__var}" '%s' "${__input}"
}

install_deps() {
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y curl wget tar ca-certificates unzip socat cron
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y curl wget tar ca-certificates unzip socat cronie
  elif command -v yum >/dev/null 2>&1; then
    yum install -y curl wget tar ca-certificates unzip socat cronie
  fi
}

install_mgmt_script() {
  local tmp
  tmp="$(mktemp)"
  local src=""
  if [[ -n "${BASH_SOURCE[0]:-}" ]]; then
    local here
    here="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd || true)"
    if [[ -n "${here}" && -f "${here}/we1bboard.sh" ]]; then
      src="${here}/we1bboard.sh"
    elif [[ -n "${here}" && -f "${here}/scripts/we1bboard.sh" ]]; then
      src="${here}/scripts/we1bboard.sh"
    fi
  fi
  if [[ -n "${src}" ]]; then
    cp -f "${src}" "${tmp}"
  else
    if ! curl -fsSL "${RAW_BASE}/scripts/we1bboard.sh" -o "${tmp}"; then
      err "Failed to download management script"
      rm -f "${tmp}"
      return 1
    fi
  fi
  install -m 755 "${tmp}" "${MGMT_BIN}"
  ln -sf "${MGMT_BIN}" /usr/bin/we1bboard-ctl
  rm -f "${tmp}"
  # `we1bboard` in PATH = management script (like x-ui); panel binary stays under INSTALL_DIR
  rm -f /usr/local/bin/we1bboard 2>/dev/null || true
  log "Management command: we1bboard"
}

download_panel() {
  local arch="$1"
  local tag="${TARGET_TAG:-}"
  mkdir -p "${INSTALL_DIR}/bin" "${DATA_DIR}"
  local tmp
  tmp="$(mktemp -d)"
  local url=""
  if [[ -n "${tag}" ]]; then
    [[ "${tag}" != v* ]] && tag="v${tag}"
    url="https://github.com/${REPO}/releases/download/${tag}/we1bboard-linux-${arch}.tar.gz"
  else
    url="${REPO_RELEASE_BASE}/we1bboard-linux-${arch}.tar.gz"
  fi
  log "Downloading ${url}"
  if ! curl -fL --retry 3 --retry-delay 2 -o "${tmp}/panel.tar.gz" "${url}"; then
    warn "Release archive not found — expecting local binary ./we1bboard"
    if [[ -f ./we1bboard ]]; then
      cp ./we1bboard "${PANEL_BIN}"
    else
      err "Place we1bboard binary next to install.sh or publish a GitHub Release"
      rm -rf "${tmp}"
      exit 1
    fi
  else
    tar -xzf "${tmp}/panel.tar.gz" -C "${tmp}"
    if [[ -f "${tmp}/we1bboard" ]]; then
      cp "${tmp}/we1bboard" "${PANEL_BIN}"
    elif [[ -f "${tmp}/we1bboard/we1bboard" ]]; then
      cp "${tmp}/we1bboard/we1bboard" "${PANEL_BIN}"
    else
      err "Archive layout unexpected"
      rm -rf "${tmp}"
      exit 1
    fi
  fi
  chmod +x "${PANEL_BIN}"
  install_mgmt_script
  rm -rf "${tmp}"
}

write_env_file() {
  local db_type="${1:-sqlite}"
  local db_dsn="${2:-}"
  cat > "${ENV_FILE}" <<EOF
WE1B_DATA_DIR=${DATA_DIR}
WE1B_BIN_DIR=${INSTALL_DIR}/bin
WE1B_DB_TYPE=${db_type}
WE1B_DB_DSN=${db_dsn}
EOF
  chmod 600 "${ENV_FILE}"
}

write_service() {
  cat > "${SERVICE_PATH}" <<EOF
[Unit]
Description=We1BBoard Xray Panel
After=network.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=-${ENV_FILE}
ExecStart=${PANEL_BIN} run
Restart=on-failure
RestartSec=5
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
}

# ---------- Database choice (like 3x-ui) ----------
choose_database() {
  echo
  echo -e "${YELLOW}Choose database backend:${NC}"
  echo -e "${GREEN}1.${NC} SQLite (default, file at ${DATA_DIR}/we1bboard.db)"
  echo -e "${GREEN}2.${NC} PostgreSQL (WE1B_DB_DSN)"
  local choice="1"
  if [[ "${NONINTERACTIVE}" == "1" ]]; then
    case "${WE1B_DB_TYPE:-sqlite}" in
      postgres|postgresql|pg) choice="2" ;;
      *) choice="1" ;;
    esac
  else
    read -r -p "Select [1]: " choice || true
    choice="${choice:-1}"
  fi

  case "${choice}" in
    2)
      local dsn="${WE1B_DB_DSN:-}"
      if [[ -z "${dsn}" ]]; then
        prompt dsn "PostgreSQL DSN" "host=127.0.0.1 user=we1b password=CHANGE_ME dbname=we1bboard port=5432 sslmode=disable"
      fi
      write_env_file "postgres" "${dsn}"
      export WE1B_DB_TYPE=postgres
      export WE1B_DB_DSN="${dsn}"
      log "Database: PostgreSQL"
      ;;
    *)
      write_env_file "sqlite" ""
      export WE1B_DB_TYPE=sqlite
      export WE1B_DB_DSN=""
      log "Database: SQLite"
      ;;
  esac
  export WE1B_DATA_DIR="${DATA_DIR}"
}

# ---------- acme.sh SSL (auto-renew) ----------
install_acme() {
  if [[ -x "${HOME}/.acme.sh/acme.sh" ]]; then
    return 0
  fi
  log "Installing acme.sh..."
  curl -fsSL https://get.acme.sh | sh
  # shellcheck disable=SC1090
  [[ -f "${HOME}/.acme.sh/acme.sh.env" ]] && source "${HOME}/.acme.sh/acme.sh.env" || true
  if [[ ! -x "${HOME}/.acme.sh/acme.sh" ]]; then
    err "Failed to install acme.sh"
    return 1
  fi
  "${HOME}/.acme.sh/acme.sh" --upgrade --auto-upgrade >/dev/null 2>&1 || true
  log "acme.sh installed (cron auto-renew enabled)"
}

apply_panel_cert() {
  local cert="$1" key="$2"
  export WE1B_DATA_DIR="${DATA_DIR}"
  # shellcheck disable=SC1090
  [[ -f "${ENV_FILE}" ]] && set -a && source "${ENV_FILE}" && set +a
  "${BIN_PATH}" cert -webCert "${cert}" -webCertKey "${key}"
}

setup_domain_certificate() {
  local domain=""
  prompt domain "Domain name" "${WE1B_DOMAIN:-}"
  if ! is_domain "${domain}"; then
    err "Invalid domain: ${domain}"
    return 1
  fi
  install_acme || return 1

  local certPath="${CERT_ROOT}/${domain}"
  mkdir -p "${certPath}"
  systemctl stop "${APP_NAME}" >/dev/null 2>&1 || true

  local webPort="80"
  while true; do
    prompt webPort "ACME HTTP-01 listen port (usually 80, must be open from Internet)" "80"
    if [[ "${webPort}" =~ ^[0-9]+$ ]] && (( webPort >= 1 && webPort <= 65535 )); then
      break
    fi
    err "Port must be a number 1–65535 (got: ${webPort})"
    [[ "${NONINTERACTIVE}" == "1" ]] && return 1
  done

  "${HOME}/.acme.sh/acme.sh" --set-default-ca --server letsencrypt --force >/dev/null 2>&1 || true
  if ! "${HOME}/.acme.sh/acme.sh" --issue -d "${domain}" --standalone --httpport "${webPort}" --force; then
    err "Failed to issue certificate for ${domain}. Is port 80 open?"
    return 1
  fi

  local reloadCmd="systemctl restart ${APP_NAME} 2>/dev/null || true"
  "${HOME}/.acme.sh/acme.sh" --installcert -d "${domain}" --force \
    --key-file "${certPath}/privkey.pem" \
    --fullchain-file "${certPath}/fullchain.pem" \
    --reloadcmd "${reloadCmd}" || true

  if [[ ! -s "${certPath}/fullchain.pem" || ! -s "${certPath}/privkey.pem" ]]; then
    err "Certificate files missing after installcert"
    return 1
  fi
  chmod 600 "${certPath}/privkey.pem"
  chmod 644 "${certPath}/fullchain.pem"
  "${HOME}/.acme.sh/acme.sh" --upgrade --auto-upgrade >/dev/null 2>&1 || true

  apply_panel_cert "${certPath}/fullchain.pem" "${certPath}/privkey.pem"
  SSL_HOST="${domain}"
  SSL_SCHEME="https"
  log "Domain certificate installed; acme.sh will auto-renew and restart ${APP_NAME}"
}

setup_ip_certificate() {
  local ipv4="$1"
  local ipv6="${2:-}"
  if ! is_ipv4 "${ipv4}"; then
    err "Invalid IPv4: ${ipv4}"
    return 1
  fi
  install_acme || return 1

  local certDir="${CERT_ROOT}/ip"
  mkdir -p "${certDir}"
  systemctl stop "${APP_NAME}" >/dev/null 2>&1 || true

  local webPort="80"
  while true; do
    prompt webPort "ACME HTTP-01 listen port" "80"
    if [[ "${webPort}" =~ ^[0-9]+$ ]] && (( webPort >= 1 && webPort <= 65535 )); then
      break
    fi
    err "Port must be a number 1–65535"
    [[ "${NONINTERACTIVE}" == "1" ]] && return 1
  done
  local domain_args=(-d "${ipv4}")
  if [[ -n "${ipv6}" ]]; then
    domain_args+=(-d "${ipv6}")
  fi

  "${HOME}/.acme.sh/acme.sh" --set-default-ca --server letsencrypt --force >/dev/null 2>&1 || true
  if ! "${HOME}/.acme.sh/acme.sh" --issue \
      "${domain_args[@]}" \
      --standalone \
      --server letsencrypt \
      --certificate-profile shortlived \
      --days 6 \
      --httpport "${webPort}" \
      --force; then
    err "Failed to issue IP certificate. Ensure port 80 is open."
    return 1
  fi

  local reloadCmd="systemctl restart ${APP_NAME} 2>/dev/null || true"
  "${HOME}/.acme.sh/acme.sh" --installcert -d "${ipv4}" --force \
    --key-file "${certDir}/privkey.pem" \
    --fullchain-file "${certDir}/fullchain.pem" \
    --reloadcmd "${reloadCmd}" || true

  if [[ ! -s "${certDir}/fullchain.pem" || ! -s "${certDir}/privkey.pem" ]]; then
    err "Certificate files missing"
    return 1
  fi
  chmod 600 "${certDir}/privkey.pem"
  chmod 644 "${certDir}/fullchain.pem"
  "${HOME}/.acme.sh/acme.sh" --upgrade --auto-upgrade >/dev/null 2>&1 || true

  apply_panel_cert "${certDir}/fullchain.pem" "${certDir}/privkey.pem"
  SSL_HOST="${ipv4}"
  SSL_SCHEME="https"
  log "IP certificate installed (~6 days); acme.sh auto-renews before expiry"
}

setup_custom_certificate() {
  local domain="" cert="" key=""
  prompt domain "Domain (or IP) for panel URL" ""
  while true; do
    prompt cert "Path to fullchain/certificate (.pem/.crt)" ""
    if [[ -f "${cert}" && -s "${cert}" ]]; then break; fi
    err "File not found or empty: ${cert}"
    [[ "${NONINTERACTIVE}" == "1" ]] && return 1
  done
  while true; do
    prompt key "Path to private key (.pem/.key)" ""
    if [[ -f "${key}" && -s "${key}" ]]; then break; fi
    err "File not found or empty: ${key}"
    [[ "${NONINTERACTIVE}" == "1" ]] && return 1
  done
  apply_panel_cert "${cert}" "${key}"
  SSL_HOST="${domain:-$(get_server_ip)}"
  SSL_SCHEME="https"
  warn "Custom cert: renew manually (or wire your own acme reloadcmd to: systemctl restart ${APP_NAME})"
}

prompt_and_setup_ssl() {
  local server_ip
  server_ip="$(get_server_ip)"
  SSL_SCHEME="http"
  SSL_HOST="${server_ip}"

  echo
  echo -e "${YELLOW}Choose SSL certificate setup method:${NC}"
  echo -e "${GREEN}1.${NC} Let's Encrypt for Domain (90-day, auto-renews via acme.sh)"
  echo -e "${GREEN}2.${NC} Let's Encrypt for IP Address (~6-day shortlived, auto-renews)"
  echo -e "${GREEN}3.${NC} Custom SSL Certificate (existing files)"
  echo -e "${GREEN}4.${NC} Skip SSL (HTTP only — behind reverse proxy / SSH tunnel)"
  echo -e "${BLUE}Note:${NC} Options 1 & 2 need port 80 open. Renewals restart ${APP_NAME} automatically."

  local ssl_choice="2"
  if [[ "${NONINTERACTIVE}" == "1" ]]; then
    case "${WE1B_SSL_MODE:-none}" in
      domain) ssl_choice="1" ;;
      ip) ssl_choice="2" ;;
      custom) ssl_choice="3" ;;
      none|skip|"") ssl_choice="4" ;;
      *) ssl_choice="4" ;;
    esac
  else
    read -r -p "Choose an option [2]: " ssl_choice || true
    ssl_choice="${ssl_choice:-2}"
    if [[ "${ssl_choice}" != "1" && "${ssl_choice}" != "3" && "${ssl_choice}" != "4" ]]; then
      ssl_choice="2"
    fi
  fi

  case "${ssl_choice}" in
    1)
      if ! setup_domain_certificate; then
        warn "Domain SSL failed — panel will start without TLS"
        SSL_SCHEME="http"
        SSL_HOST="${server_ip}"
      fi
      ;;
    2)
      local conf_ip="${server_ip}"
      if [[ "${NONINTERACTIVE}" != "1" ]]; then
        local yn=""
        read -r -p "Is ${server_ip} the correct public IPv4? [Y/n]: " yn || true
        if [[ -n "${yn}" && "${yn}" != "y" && "${yn}" != "Y" ]]; then
          prompt conf_ip "Public IPv4" ""
        fi
      fi
      local ipv6=""
      prompt ipv6 "Optional IPv6 to include (empty=skip)" ""
      if ! setup_ip_certificate "${conf_ip}" "${ipv6}"; then
        warn "IP SSL failed — panel will start without TLS"
        SSL_SCHEME="http"
        SSL_HOST="${conf_ip}"
      fi
      ;;
    3)
      setup_custom_certificate || true
      ;;
    4)
      log "SSL skipped — HTTP only"
      SSL_SCHEME="http"
      SSL_HOST="${server_ip}"
      export WE1B_DATA_DIR="${DATA_DIR}"
      # shellcheck disable=SC1090
      [[ -f "${ENV_FILE}" ]] && set -a && source "${ENV_FILE}" && set +a
      "${BIN_PATH}" setting set certFile ""
      "${BIN_PATH}" setting set keyFile ""
      ;;
  esac
}

configure_panel() {
  local port path user pass theme accent
  prompt port "Panel port" "2053"
  prompt path "Panel base path" "/we1b/"
  prompt user "Admin username" "admin"
  local default_pass
  default_pass="$(openssl rand -base64 18 2>/dev/null | tr -dc 'A-Za-z0-9' | head -c 16)"
  if [[ -z "${default_pass}" || ${#default_pass} -lt 12 ]]; then
    default_pass="$(head -c 32 /dev/urandom 2>/dev/null | od -An -tx1 | tr -d ' \n' | head -c 16)"
  fi
  if [[ "${NONINTERACTIVE}" == "1" ]]; then
    pass="${WE1B_ADMIN_PASSWORD:-${default_pass}}"
    user="${WE1B_ADMIN_USER:-admin}"
  else
    prompt pass "Admin password (min 8 chars)" "${default_pass}"
  fi
  if [[ ${#pass} -lt 8 ]]; then
    err "password must be at least 8 characters"
    exit 1
  fi
  prompt theme "Theme (light/night/amoled)" "night"
  prompt accent "Accent (blue/purple)" "blue"

  PANEL_PORT="${port}"
  PANEL_PATH="${path}"

  export WE1B_DATA_DIR="${DATA_DIR}"
  # shellcheck disable=SC1090
  [[ -f "${ENV_FILE}" ]] && set -a && source "${ENV_FILE}" && set +a

  "${BIN_PATH}" setting set panelPort "${port}"
  "${BIN_PATH}" setting set panelPath "${path}"
  "${BIN_PATH}" setting set theme "${theme}"
  "${BIN_PATH}" setting set accent "${accent}"
  "${BIN_PATH}" reset-admin "${user}" "${pass}"

  umask 077
  cat > "${DATA_DIR}/install-result.env" <<EOF
WE1B_USERNAME=$(printf '%q' "${user}")
WE1B_PASSWORD=$(printf '%q' "${pass}")
WE1B_PANEL_PORT=$(printf '%q' "${port}")
WE1B_PANEL_PATH=$(printf '%q' "${path}")
EOF
  chmod 600 "${DATA_DIR}/install-result.env"
  log "Credentials saved to ${DATA_DIR}/install-result.env (mode 600)"
  if [[ "${NONINTERACTIVE}" == "1" ]]; then
    log "Admin: ${user} / (see install-result.env)"
  fi
}

start_panel() {
  systemctl enable --now "${APP_NAME}"
  systemctl status "${APP_NAME}" --no-pager || true
}

print_access() {
  echo
  echo -e "${GREEN}┌──────────────────────────────────────────────────────────┐${NC}"
  echo -e "${GREEN}│  Installation complete                                   │${NC}"
  echo -e "${GREEN}└──────────────────────────────────────────────────────────┘${NC}"
  echo
  log "Panel URL: ${SSL_SCHEME}://${SSL_HOST}:${PANEL_PORT}${PANEL_PATH}"
  log "Login: see ${DATA_DIR}/install-result.env  (or the password you entered)"
  if [[ "${SSL_SCHEME}" == "https" ]]; then
    log "TLS: enabled (acme auto-renew restarts ${APP_NAME})"
  else
    warn "TLS: off — run: we1bboard ssl   when ready"
  fi
  echo
  echo -e "┌───────────────────────────────────────────────────────┐"
  echo -e "│ ${BLUE}we1bboard${NC} control menu:                              │"
  echo -e "│                                                       │"
  echo -e "│ ${BLUE}we1bboard${NC}              - Admin management menu        │"
  echo -e "│ ${BLUE}we1bboard start${NC}        - Start                        │"
  echo -e "│ ${BLUE}we1bboard stop${NC}         - Stop                         │"
  echo -e "│ ${BLUE}we1bboard restart${NC}      - Restart                      │"
  echo -e "│ ${BLUE}we1bboard status${NC}       - Status                       │"
  echo -e "│ ${BLUE}we1bboard update${NC}       - Update to latest             │"
  echo -e "│ ${BLUE}we1bboard ssl${NC}          - SSL certificate setup        │"
  echo -e "│ ${BLUE}we1bboard uninstall${NC}    - Uninstall                    │"
  echo -e "└───────────────────────────────────────────────────────┘"
}

setup_ufw() {
  echo
  echo -e "${YELLOW}Firewall (UFW):${NC}"
  echo -e "${GREEN}1.${NC} Enable UFW and open panel / sub / 80 / 443 (recommended)"
  echo -e "${GREEN}2.${NC} Skip (manage firewall yourself)"
  local choice="1"
  if [[ "${NONINTERACTIVE}" == "1" ]]; then
    case "${WE1B_UFW:-1}" in
      0|false|no|off) choice="2" ;;
      *) choice="1" ;;
    esac
  else
    read -r -p "Select [1]: " choice || true
    choice="${choice:-1}"
  fi
  if [[ "${choice}" != "1" ]]; then
    log "UFW skipped"
    export WE1B_DATA_DIR="${DATA_DIR}"
    # shellcheck disable=SC1090
    [[ -f "${ENV_FILE}" ]] && set -a && source "${ENV_FILE}" && set +a
    "${BIN_PATH}" setting set ufwEnable false 2>/dev/null || true
    return 0
  fi
  if ! command -v ufw >/dev/null 2>&1; then
    if command -v apt-get >/dev/null 2>&1; then
      apt-get install -y ufw >/dev/null 2>&1 || true
    fi
  fi
  if !command -v ufw >/dev/null 2>&1; then
    warn "ufw not installed — skip"
    return 0
  fi
  # Preserve SSH before enabling (avoid lockout)
  local ssh_port="22"
  if command -v ss >/dev/null 2>&1; then
    local detected
    detected="$(ss -lntp 2>/dev/null | awk '/sshd/ { for(i=1;i<=NF;i++) if($i ~ /:[0-9]+$/) { split($i,a,":"); print a[length(a)]; exit } }')"
    if [[ "${detected}" =~ ^[0-9]+$ ]]; then
      ssh_port="${detected}"
    fi
  fi
  ufw allow "${ssh_port}/tcp" comment 'we1bboard-ssh' >/dev/null 2>&1 || true
  ufw allow "${PANEL_PORT}/tcp" comment 'we1bboard-panel' >/dev/null 2>&1 || true
  ufw allow 80/tcp comment 'we1bboard-http' >/dev/null 2>&1 || true
  ufw allow 443/tcp comment 'we1bboard-https' >/dev/null 2>&1 || true
  local subp
  subp="$("${BIN_PATH}" setting get subPort 2>/dev/null || echo 2096)"
  if [[ -n "${subp}" && "${subp}" != "${PANEL_PORT}" ]]; then
    ufw allow "${subp}/tcp" comment 'we1bboard-sub' >/dev/null 2>&1 || true
  fi
  ufw --force enable >/dev/null 2>&1 || true
  export WE1B_DATA_DIR="${DATA_DIR}"
  # shellcheck disable=SC1090
  [[ -f "${ENV_FILE}" ]] && set -a && source "${ENV_FILE}" && set +a
  "${BIN_PATH}" setting set ufwEnable true 2>/dev/null || true
  log "UFW enabled; opened SSH:${ssh_port}, panel:${PANEL_PORT}, 80, 443${subp:+, sub:${subp}}"
}

do_install() {
  need_root
  banner
  log "Starting interactive install (like 3x-ui one-liner)"
  step 1 "Install dependencies"
  install_deps
  step 2 "Download We1BBoard binary"
  download_panel "$(detect_arch)"
  step 3 "Choose database"
  choose_database
  write_service
  step 4 "Panel access (port / path / admin)"
  configure_panel
  step 5 "SSL certificate"
  prompt_and_setup_ssl
  step 6 "Firewall (UFW)"
  setup_ufw
  step 7 "Start service"
  start_panel
  print_access
  log "Installation finished."
}

ssl_menu_only() {
  need_root
  export WE1B_DATA_DIR="${DATA_DIR}"
  # shellcheck disable=SC1090
  [[ -f "${ENV_FILE}" ]] && set -a && source "${ENV_FILE}" && set +a
  PANEL_PORT="$("${BIN_PATH}" setting get panelPort 2>/dev/null || echo 2053)"
  PANEL_PATH="$("${BIN_PATH}" setting get panelPath 2>/dev/null || echo /we1b/)"
  prompt_and_setup_ssl
  systemctl restart "${APP_NAME}" || true
  print_access
}

menu() {
  while true; do
    echo
    echo -e "${BLUE}========== We1BBoard ==========${NC}"
    echo "1. Install / Reinstall"
    echo "2. Start"
    echo "3. Stop"
    echo "4. Restart"
    echo "5. Status"
    echo "6. Reset admin"
    echo "7. Open CLI menu"
    echo "8. SSL certificate setup / renew paths"
    echo "9. Change database env (sqlite/postgres)"
    echo "10. Update to latest"
    echo "11. Install specific / legacy version"
    echo "12. Quick rollback (previous binary)"
    echo "13. Uninstall"
    echo "0. Exit"
    read -r -p "Select: " c || true
    case "${c}" in
      1) do_install ;;
      2) systemctl start "${APP_NAME}" ;;
      3) systemctl stop "${APP_NAME}" ;;
      4) systemctl restart "${APP_NAME}" ;;
      5) systemctl status "${APP_NAME}" --no-pager ;;
      6) export WE1B_DATA_DIR="${DATA_DIR}"; # shellcheck disable=SC1090
         [[ -f "${ENV_FILE}" ]] && set -a && source "${ENV_FILE}" && set +a
         "${BIN_PATH}" reset-admin ;;
      7) export WE1B_DATA_DIR="${DATA_DIR}"; # shellcheck disable=SC1090
         [[ -f "${ENV_FILE}" ]] && set -a && source "${ENV_FILE}" && set +a
         "${BIN_PATH}" menu ;;
      8) ssl_menu_only ;;
      9)
        need_root
        choose_database
        write_service
        systemctl restart "${APP_NAME}" || true
        log "Database env updated; panel restarted"
        ;;
      10)
        need_root
        bash <(curl -fsSL "https://raw.githubusercontent.com/${REPO}/main/scripts/update.sh")
        ;;
      11)
        need_root
        bash <(curl -fsSL "https://raw.githubusercontent.com/${REPO}/main/scripts/update.sh") legacy
        ;;
      12)
        need_root
        bash <(curl -fsSL "https://raw.githubusercontent.com/${REPO}/main/scripts/update.sh") rollback
        ;;
      13)
        systemctl disable --now "${APP_NAME}" || true
        rm -f "${SERVICE_PATH}" "${ENV_FILE}" "${MGMT_BIN}" /usr/bin/we1bboard-ctl /usr/local/bin/we1bboard
        rm -rf "${INSTALL_DIR}"
        systemctl daemon-reload
        log "Uninstalled (data kept at ${DATA_DIR}, certs at ${CERT_ROOT})"
        ;;
      0) exit 0 ;;
    esac
  done
}

case "${INSTALL_MODE}" in
  install) do_install ;;
  ssl) ssl_menu_only ;;
  *) menu ;;
esac
