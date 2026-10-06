#!/usr/bin/env bash
set -euo pipefail

# We1BBoard control menu (analog of x-ui.sh)
# Installed as /usr/bin/we1bboard  (we1bboard-ctl is a symlink)

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m'

APP_NAME="we1bboard"
REPO="WeIbSchatten/We1BBoard"
INSTALL_DIR="/usr/local/we1bboard"
BIN="${INSTALL_DIR}/we1bboard"
DATA_DIR="/etc/we1bboard"
ENV_FILE="/etc/default/we1bboard"
RAW_BASE="https://raw.githubusercontent.com/${REPO}/main"

log() { echo -e "${GREEN}[*]${NC} $*"; }
err() { echo -e "${RED}[!]${NC} $*" >&2; }

load_env() {
  export WE1B_DATA_DIR="${DATA_DIR}"
  # shellcheck disable=SC1090
  [[ -f "${ENV_FILE}" ]] && set -a && source "${ENV_FILE}" && set +a
}

need_root() {
  [[ ${EUID} -eq 0 ]] || { err "Run as root"; exit 1; }
}

show_status() {
  load_env
  echo -e "${BLUE}--- We1BBoard status ---${NC}"
  if [[ -x "${BIN}" ]]; then
    echo "Version: $("${BIN}" version 2>/dev/null || echo unknown)"
  fi
  systemctl is-active --quiet "${APP_NAME}" && echo "Service: active" || echo "Service: inactive"
  systemctl status "${APP_NAME}" --no-pager -l 2>/dev/null | head -n 12 || true
}

update_latest() {
  need_root
  bash <(curl -fsSL "${RAW_BASE}/scripts/update.sh")
}

update_legacy() {
  need_root
  bash <(curl -fsSL "${RAW_BASE}/scripts/update.sh") legacy
}

rollback() {
  need_root
  bash <(curl -fsSL "${RAW_BASE}/scripts/update.sh") rollback
}

update_ctl_script() {
  need_root
  curl -fsSL "${RAW_BASE}/scripts/we1bboard.sh" -o /usr/bin/we1bboard
  chmod +x /usr/bin/we1bboard
  ln -sf /usr/bin/we1bboard /usr/bin/we1bboard-ctl
  log "Control script updated. Re-run: we1bboard"
}

ssl_setup() {
  need_root
  bash <(curl -fsSL "${RAW_BASE}/install.sh") ssl
}

reinstall() {
  need_root
  bash <(curl -fsSL "${RAW_BASE}/install.sh")
}

uninstall() {
  need_root
  read -r -p "Uninstall panel binary/service? Data in ${DATA_DIR} kept. [y/N]: " y
  [[ "${y}" == "y" || "${y}" == "Y" ]] || return 0
  systemctl disable --now "${APP_NAME}" 2>/dev/null || true
  rm -f /etc/systemd/system/${APP_NAME}.service /usr/local/bin/${APP_NAME} /usr/bin/we1bboard /usr/bin/we1bboard-ctl /usr/bin/we1bboard-menu
  rm -rf "${INSTALL_DIR}"
  systemctl daemon-reload
  log "Uninstalled (data kept at ${DATA_DIR})"
}

menu() {
  while true; do
    echo
    echo -e "${BLUE}========== We1BBoard Control ==========${NC}"
    echo "0. Exit"
    echo "1. Start"
    echo "2. Stop"
    echo "3. Restart"
    echo "4. Status"
    echo "5. Update to latest"
    echo "6. Install legacy / specific version (rollback by tag)"
    echo "7. Quick rollback to previous binary"
    echo "8. SSL certificate setup"
    echo "9. Reset admin password"
    echo "10. Panel CLI menu"
    echo "11. Update this control script"
    echo "12. Reinstall"
    echo "13. Uninstall"
    read -r -p "Select: " c || true
    case "${c}" in
      0) exit 0 ;;
      1) need_root; systemctl start "${APP_NAME}" ;;
      2) need_root; systemctl stop "${APP_NAME}" ;;
      3) need_root; systemctl restart "${APP_NAME}" ;;
      4) show_status ;;
      5) update_latest ;;
      6) update_legacy ;;
      7) rollback ;;
      8) ssl_setup ;;
      9) need_root; load_env; "${BIN}" reset-admin ;;
      10) load_env; "${BIN}" menu ;;
      11) update_ctl_script; exit 0 ;;
      12) reinstall ;;
      13) uninstall ;;
      *) err "invalid option" ;;
    esac
  done
}

case "${1:-}" in
  start) need_root; systemctl start "${APP_NAME}" ;;
  stop) need_root; systemctl stop "${APP_NAME}" ;;
  restart) need_root; systemctl restart "${APP_NAME}" ;;
  status) show_status ;;
  update) update_latest ;;
  legacy) update_legacy ;;
  rollback) rollback ;;
  ssl) ssl_setup ;;
  update-ctl) update_ctl_script ;;
  uninstall) uninstall ;;
  install) reinstall ;;
  menu|"") menu ;;
  *)
    echo "Usage: we1bboard {menu|start|stop|restart|status|update|legacy|rollback|ssl|install|update-ctl|uninstall}"
    exit 1
    ;;
esac
