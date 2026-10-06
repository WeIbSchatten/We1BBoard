#!/usr/bin/env bash
# We1BBoard — one-command installer (same UX as 3x-ui)
#
#   bash <(curl -Ls https://raw.githubusercontent.com/WeIbSchatten/We1BBoard/main/install.sh)
#   bash <(curl -Ls https://raw.githubusercontent.com/WeIbSchatten/We1BBoard/main/install.sh) v1.0.0
#
# After install: run `we1bboard` for the management menu.
set -euo pipefail

REPO="WeIbSchatten/We1BBoard"
RAW_BASE="https://raw.githubusercontent.com/${REPO}/main"

# Prefer local scripts/ when this file is run from a git checkout
HERE=""
if [[ -n "${BASH_SOURCE[0]:-}" ]]; then
  HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd || true)"
fi

if [[ -n "${HERE}" && -f "${HERE}/scripts/install.sh" ]]; then
  exec bash "${HERE}/scripts/install.sh" "$@"
fi

# Remote one-liner / process substitution: fetch full installer and run it
tmp="$(mktemp)"
trap 'rm -f "${tmp}"' EXIT
if ! curl -fsSL "${RAW_BASE}/scripts/install.sh" -o "${tmp}"; then
  echo "Failed to download installer from ${RAW_BASE}/scripts/install.sh" >&2
  exit 1
fi
bash "${tmp}" "$@"
