#!/usr/bin/env bash
# 20-brew.sh — install Homebrew (Linuxbrew) under the dev user's ownership.
#
# Homebrew requires the user running it to own the prefix — not just
# membership in some shared group. Rather than split permissions between
# a separate `linuxbrew` user and `dev`, install everything as `dev`.
# That's also what the upstream Linuxbrew install.sh does for single-user
# setups, and it matches the container's only interactive user.
#
# The prefix stays at the canonical /home/linuxbrew/.linuxbrew so `brew
# shellenv` finds it and third-party taps that hardcode the path (rare
# but they exist) keep working. A shared `devtools_brew` docker volume
# mounted there at runtime gives every project on this host the same
# cellar, so a formula installed for one project is immediately available
# to all the others.
set -euo pipefail

# $USERNAME is passed from the Dockerfile ARG so this script stays honest
# about who owns what. Default to dev in case it's run standalone.
USERNAME="${USERNAME:-dev}"

install -d -m 0755 -o "$USERNAME" -g "$USERNAME" /home/linuxbrew
install -d -m 0755 -o "$USERNAME" -g "$USERNAME" /home/linuxbrew/.linuxbrew
install -d -m 0755 -o "$USERNAME" -g "$USERNAME" /home/linuxbrew/.linuxbrew/bin

# Shallow-clone Homebrew as the dev user so every object lands with the
# right ownership from the start. The installer's `curl | bash` route
# would need a tty and would try to sudo; a direct git clone is simpler
# and fully reproducible.
sudo -u "$USERNAME" git clone --depth=1 \
    https://github.com/Homebrew/brew \
    /home/linuxbrew/.linuxbrew/Homebrew

sudo -u "$USERNAME" ln -sf \
    /home/linuxbrew/.linuxbrew/Homebrew/bin/brew \
    /home/linuxbrew/.linuxbrew/bin/brew

# Expose brew to login shells. `brew shellenv` emits the full set of
# HOMEBREW_* vars + PATH/MANPATH/INFOPATH entries; sourcing it keeps us
# in lock-step with whatever brew itself considers authoritative.
cat > /etc/profile.d/brew.sh <<'PROFILE'
# Homebrew (Linuxbrew): prefix + PATH for every shell.
if [ -x /home/linuxbrew/.linuxbrew/bin/brew ]; then
    eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv 2>/dev/null || true)"
fi
PROFILE
chmod 0644 /etc/profile.d/brew.sh

# Pre-warm the tap as dev. Non-fatal on network hiccups — the first real
# `brew bundle` call at container start will try again if needed.
sudo -u "$USERNAME" /home/linuxbrew/.linuxbrew/bin/brew update-reset >/dev/null 2>&1 || true
sudo -u "$USERNAME" /home/linuxbrew/.linuxbrew/bin/brew --version

rm -f /tmp/20-brew.sh
