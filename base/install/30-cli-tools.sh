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
# System-wide interactive-shell init.
#
# Debian's bash sources /etc/bash.bashrc for interactive non-login shells
# (what you get from `docker exec -it <container> bash`), and /etc/profile
# → /etc/profile.d/*.sh for login shells. Putting our mise + oh-my-posh
# activation in a dedicated /etc/profile.d file and sourcing it from
# /etc/bash.bashrc covers both, and — crucially — makes the prompt work
# even when the user's home volume was seeded from an old image whose
# ~/.bashrc predates these blocks. Without this, stale home volumes keep
# whatever dotfiles they first got forever.
# -----------------------------------------------------------------------------
cat > /etc/profile.d/devtools-shell.sh <<'PROFILE'
# devtools: interactive-shell init. Sourced by login shells via /etc/profile
# and by non-login interactive bash via /etc/bash.bashrc. Safe to source
# multiple times — every step is idempotent.
if [ -z "${BASH_VERSION:-}" ]; then
    return 0
fi
case $- in
    *i*) ;;
      *) return 0 ;;
esac

# Ensure XDG_CACHE_HOME exists and points at ~/.cache. Without this
# oh-my-posh (v29) drops its runtime caches (bash.UUID.omp.cache, omp.cache)
# into $PWD, polluting every user project directory. Exporting here is
# early enough that OMP's init below picks it up.
: "${XDG_CACHE_HOME:=$HOME/.cache}"
export XDG_CACHE_HOME
mkdir -p "$XDG_CACHE_HOME" 2>/dev/null || true

# mise — runtime manager. Activate hooks + shims.
if [ -x /usr/local/bin/mise ]; then
    eval "$(/usr/local/bin/mise activate bash)"
fi

# oh-my-posh — Atomic prompt theme. `--print` dumps the full init script
# inline; without it OMP emits `source $'init.HASH.sh'` with a relative
# path, which makes bash source a temp file that OMP had to write to the
# user's cwd. Inlining avoids the temp file entirely.
if command -v oh-my-posh >/dev/null 2>&1; then
    __omp_theme="${POSH_THEMES_PATH:-/usr/local/share/oh-my-posh/themes}/atomic.omp.json"
    [ -f "$__omp_theme" ] || __omp_theme="$HOME/.config/oh-my-posh/atomic.omp.json"
    [ -f "$__omp_theme" ] || __omp_theme="/etc/skel/.config/oh-my-posh/atomic.omp.json"
    if [ -f "$__omp_theme" ]; then
        eval "$(oh-my-posh init bash --print --config "$__omp_theme")"
    fi
    unset __omp_theme
fi

# fzf — keybindings + completions (Debian package).
[ -f /usr/share/doc/fzf/examples/key-bindings.bash ] && . /usr/share/doc/fzf/examples/key-bindings.bash
[ -f /usr/share/bash-completion/completions/fzf ]    && . /usr/share/bash-completion/completions/fzf
PROFILE
chmod 0644 /etc/profile.d/devtools-shell.sh

# Ensure /etc/bash.bashrc sources it for non-login interactive shells.
# Idempotent: only appended once (guarded by a marker comment).
if ! grep -q "devtools-shell.sh" /etc/bash.bashrc 2>/dev/null; then
    cat >> /etc/bash.bashrc <<'BBRC'

# devtools: interactive-shell init (mise activation, oh-my-posh prompt, fzf).
# This runs *before* ~/.bashrc, so stale home volumes still get the prompt.
if [ -r /etc/profile.d/devtools-shell.sh ]; then
    . /etc/profile.d/devtools-shell.sh
fi
BBRC
fi

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
