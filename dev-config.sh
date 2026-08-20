#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VENV_DIR="${REPO_ROOT}/.venv"

if [ ! -d "${VENV_DIR}" ]; then
  python3 -m venv "${VENV_DIR}"
fi

# shellcheck disable=SC1091
source "${VENV_DIR}/bin/activate"

python -m pip install --upgrade pip
python -m pip install vacuum

cat > "${REPO_ROOT}/.git/info/exclude" <<'EOF'
.venv/
venv/
server/.bin/
**/.bin/
**/vacuum
*.bin
vacuum-report-*.json
EOF

echo "Virtual environment ready at ${VENV_DIR}"
echo "Local tool artifacts are excluded from git via .git/info/exclude"
