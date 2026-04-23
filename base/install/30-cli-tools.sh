#!/usr/bin/env bash
# 30-cli-tools.sh — modern CLI tools not in apt (or with stale apt versions).
#
# Installs: yq (Go mikefarah), oh-my-posh + Atomic theme, gh (GitHub CLI).
# rg / fd / bat / fzf / eza are installed via apt in 10-apt.sh.
set -euo pipefail

ARCH="$(dpkg --print-architecture)"  # amd64 | arm64
case "$ARCH" in
    amd64) GOARCH=amd64 ;;
    arm64) GOARCH=arm64 ;;
    *) echo "Unsupported arch: $ARCH" >&2; exit 1 ;;
esac

# -----------------------------------------------------------------------------
# yq (mikefarah/yq) — YAML processor. `dev` uses the Go library directly,
# but the CLI is still handy inside containers.
# -----------------------------------------------------------------------------
YQ_VERSION="${YQ_VERSION:-v4.45.1}"
curl -fsSL -o /usr/local/bin/yq \
    "https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/yq_linux_${GOARCH}"
chmod 0755 /usr/local/bin/yq
/usr/local/bin/yq --version

# -----------------------------------------------------------------------------
# oh-my-posh + Atomic theme.
# Installer places the binary in /usr/local/bin/oh-my-posh and (on recent
# versions) drops themes into /usr/local/share/oh-my-posh/themes.
# We also download the Atomic theme explicitly to guarantee its presence
# and mirror it into /etc/skel as a belt-and-braces fallback.
# -----------------------------------------------------------------------------
curl -fsSL https://ohmyposh.dev/install.sh | bash -s -- -d /usr/local/bin
/usr/local/bin/oh-my-posh version

THEMES_DIR=/usr/local/share/oh-my-posh/themes
install -d "$THEMES_DIR"
curl -fsSL -o "$THEMES_DIR/atomic.omp.json" \
    https://raw.githubusercontent.com/JanDeDobbeleer/oh-my-posh/main/themes/atomic.omp.json

# Expose themes path to all shells.
cat > /etc/profile.d/oh-my-posh.sh <<'PROFILE'
# oh-my-posh: themes directory for config discovery
export POSH_THEMES_PATH="/usr/local/share/oh-my-posh/themes"
PROFILE
chmod 0644 /etc/profile.d/oh-my-posh.sh

# Mirror the theme into /etc/skel so first-start home volumes always have it.
install -d /etc/skel/.config/oh-my-posh
cp "$THEMES_DIR/atomic.omp.json" /etc/skel/.config/oh-my-posh/atomic.omp.json

# -----------------------------------------------------------------------------
# gh (GitHub CLI) — official apt repo.
# -----------------------------------------------------------------------------
install -d -m 0755 /etc/apt/keyrings
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
    -o /etc/apt/keyrings/githubcli-archive-keyring.gpg
chmod 0644 /etc/apt/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=${ARCH} signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
    > /etc/apt/sources.list.d/github-cli.list
apt-get update
apt-get install -y --no-install-recommends gh
apt-get clean
rm -rf /var/lib/apt/lists/*
/usr/bin/gh --version

rm -f /tmp/30-cli-tools.sh
