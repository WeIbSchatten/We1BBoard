#!/usr/bin/env bash
set -euo pipefail

# We1BBoard updater (3x-ui style): latest / pinned version / keep previous binary for rollback
# Usage:
#   bash update.sh                  # latest release
#   bash update.sh v1.0.0           # specific tag
#   WE1B_UPDATE_TAG=v1.0.0 bash update.sh

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

APP_NAME="we1bboard"
REPO="WeIbSchatten/We1BBoard"
INSTALL_DIR="/usr/local/we1bboard"
PANEL_BIN="${INSTALL_DIR}/we1bboard"
BIN_PATH="${PANEL_BIN}"
SERVICE_PATH="/etc/systemd/system/${APP_NAME}.service"
ENV_FILE="/etc/default/we1bboard"
DATA_DIR="/etc/we1bboard"
BACKUP_DIR="${INSTALL_DIR}/backups"
MGMT_SCRIPT="/usr/bin/we1bboard"

log() { echo -e "${GREEN}[update]${NC} $*"; }
warn() { echo -e "${YELLOW}[warn]${NC} $*"; }
err() { echo -e "${RED}[error]${NC} $*" >&2; }

need_root() {
  [[ ${EUID} -eq 0 ]] || { err "Run as root"; exit 1; }
}

arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) err "unsupported arch"; exit 1 ;;
  esac
}

current_version() {
  if [[ -x "${INSTALL_DIR}/we1bboard" ]]; then
    "${INSTALL_DIR}/we1bboard" version 2>/dev/null || echo "unknown"
  else
    echo "unknown"
  fi
}

fetch_latest_tag() {
  local tag=""
  tag="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
    | grep -oE '"tag_name":[[:space:]]*"[^"]+"' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')"
  if [[ -z "${tag}" ]]; then
    tag="$(curl -4 -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
      | grep -oE '"tag_name":[[:space:]]*"[^"]+"' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')"
  fi
  echo "${tag}"
}

list_recent_tags() {
  curl -fsSL "https://api.github.com/repos/${REPO}/releases?per_page=15" 2>/dev/null \
    | grep -oE '"tag_name":[[:space:]]*"[^"]+"' | sed -E 's/.*"([^"]+)".*/\1/' || true
}

download_release() {
  local tag="$1" a="$2" dest="$3"
  local url="https://github.com/${REPO}/releases/download/${tag}/we1bboard-linux-${a}.tar.gz"
  log "Downloading ${url}"
  if ! curl -fL --retry 3 --retry-delay 2 -o "${dest}" "${url}"; then
    if ! curl -4 -fL --retry 3 --retry-delay 2 -o "${dest}" "${url}"; then
      err "Failed to download release ${tag}. Publish GitHub Release assets first."
      return 1
    fi
  fi
}

backup_current() {
  mkdir -p "${BACKUP_DIR}"
  local ver
  ver="$(current_version)"
  ver="${ver//\//_}"
  local stamp
  stamp="$(date +%Y%m%d-%H%M%S)"
  if [[ -x "${INSTALL_DIR}/we1bboard" ]]; then
    cp -a "${INSTALL_DIR}/we1bboard" "${INSTALL_DIR}/we1bboard.prev"
    cp -a "${INSTALL_DIR}/we1bboard" "${BACKUP_DIR}/we1bboard-${ver}-${stamp}"
    # keep last 10 backups
    ls -1t "${BACKUP_DIR}"/we1bboard-* 2>/dev/null | tail -n +11 | xargs -r rm -f
    log "Backed up current binary → we1bboard.prev and ${BACKUP_DIR}/"
  fi
}

install_mgmt_script() {
  local src_url="https://raw.githubusercontent.com/${REPO}/main/scripts/we1bboard.sh"
  if curl -fsSL "${src_url}" -o "${MGMT_SCRIPT}.tmp" 2>/dev/null; then
    mv "${MGMT_SCRIPT}.tmp" "${MGMT_SCRIPT}"
    chmod +x "${MGMT_SCRIPT}"
    ln -sf "${MGMT_SCRIPT}" /usr/bin/we1bboard-ctl 2>/dev/null || true
    rm -f /usr/local/bin/we1bboard 2>/dev/null || true
    log "Management script updated: we1bboard"
  else
    warn "Could not refresh management script from GitHub (optional)"
    rm -f "${MGMT_SCRIPT}.tmp"
  fi
}

do_update() {
  need_root
  local tag="${1:-${WE1B_UPDATE_TAG:-}}"
  local cur
  cur="$(current_version)"
  log "Current version: ${cur}"

  if [[ -z "${tag}" ]]; then
    tag="$(fetch_latest_tag)"
  fi
  if [[ -z "${tag}" ]]; then
    err "Could not resolve release tag from GitHub"
    exit 1
  fi
  # allow both v1.0.0 and 1.0.0
  if [[ "${tag}" != v* ]]; then
    tag="v${tag}"
  fi
  log "Target version: ${tag}"

  local a tmp archive
  a="$(arch)"
  tmp="$(mktemp -d)"
  archive="${tmp}/we1bboard-linux-${a}.tar.gz"
  download_release "${tag}" "${a}" "${archive}"

  systemctl stop "${APP_NAME}" >/dev/null 2>&1 || true
  backup_current

  tar -xzf "${archive}" -C "${tmp}"
  if [[ ! -f "${tmp}/we1bboard" ]]; then
    # sometimes nested folder
    if [[ -f "${tmp}/we1bboard/we1bboard" ]]; then
      mv "${tmp}/we1bboard/we1bboard" "${tmp}/we1bboard.bin"
    else
      err "Archive missing we1bboard binary"
      systemctl start "${APP_NAME}" >/dev/null 2>&1 || true
      rm -rf "${tmp}"
      exit 1
    fi
  else
    mv "${tmp}/we1bboard" "${tmp}/we1bboard.bin"
  fi

  mkdir -p "${INSTALL_DIR}/bin" "${DATA_DIR}"
  install -m 755 "${tmp}/we1bboard.bin" "${INSTALL_DIR}/we1bboard"
  rm -f /usr/local/bin/we1bboard 2>/dev/null || true

  # refresh systemd unit if shipped later; keep existing env
  if [[ ! -f "${SERVICE_PATH}" ]]; then
    cat > "${SERVICE_PATH}" <<EOF
[Unit]
Description=We1BBoard Xray Panel
After=network.target

[Service]
Type=simple
EnvironmentFile=-${ENV_FILE}
Environment=WE1B_DATA_DIR=${DATA_DIR}
Environment=WE1B_BIN_DIR=${INSTALL_DIR}/bin
ExecStart=${INSTALL_DIR}/we1bboard run
Restart=on-failure
RestartSec=5
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    systemctl enable "${APP_NAME}" >/dev/null 2>&1 || true
  fi

  install_mgmt_script
  rm -rf "${tmp}"

  systemctl start "${APP_NAME}"
  local newv
  newv="$(current_version)"
  log "Updated ${cur} → ${newv} (${tag})"
  log "Quick rollback: we1bboard rollback   OR   bash update.sh rollback"
}

do_rollback_prev() {
  need_root
  if [[ ! -x "${INSTALL_DIR}/we1bboard.prev" ]]; then
    err "No previous binary at ${INSTALL_DIR}/we1bboard.prev"
    err "Use: we1bboard legacy   to install a specific release tag"
    exit 1
  fi
  systemctl stop "${APP_NAME}" >/dev/null 2>&1 || true
  # swap
  if [[ -x "${INSTALL_DIR}/we1bboard" ]]; then
    cp -a "${INSTALL_DIR}/we1bboard" "${INSTALL_DIR}/we1bboard.rolled-forward"
  fi
  cp -a "${INSTALL_DIR}/we1bboard.prev" "${INSTALL_DIR}/we1bboard"
  chmod +x "${INSTALL_DIR}/we1bboard"
  systemctl start "${APP_NAME}"
  log "Rolled back to previous binary: $(current_version)"
}

do_legacy() {
  need_root
  local tag="${1:-}"
  if [[ -z "${tag}" ]]; then
    echo "Recent releases:"
    list_recent_tags | sed 's/^/  /'
    read -r -p "Enter version tag (e.g. 1.0.0 or v1.0.0): " tag
  fi
  [[ -n "${tag}" ]] || { err "version empty"; exit 1; }
  do_update "${tag}"
}

case "${1:-}" in
  rollback|prev)
    do_rollback_prev
    ;;
  legacy)
    shift || true
    do_legacy "${1:-}"
    ;;
  list)
    list_recent_tags
    ;;
  *)
    do_update "${1:-}"
    ;;
esac
