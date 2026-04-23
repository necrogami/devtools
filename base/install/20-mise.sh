#!/usr/bin/env bash
# 20-mise.sh — install mise (runtime version manager) system-wide.
#
# No language runtimes are installed here; mise-managed runtimes live
# in the shared `devtools_mise` Docker volume mounted at first `dev up`.
set -euo pipefail

# Official installer honors MISE_INSTALL_PATH.
curl -fsSL https://mise.run | MISE_INSTALL_PATH=/usr/local/bin/mise sh

# Verify.
/usr/local/bin/mise --version

# System-wide activation for interactive login shells.
install -d /etc/profile.d
cat > /etc/profile.d/mise.sh <<'PROFILE'
# mise: runtime version manager
if [ -x /usr/local/bin/mise ] && [ -n "${BASH_VERSION:-}" ]; then
    eval "$(/usr/local/bin/mise activate bash)"
fi
PROFILE
chmod 0644 /etc/profile.d/mise.sh

rm -f /tmp/20-mise.sh
