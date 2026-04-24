#!/usr/bin/env bash
# 20-brew.sh — install Homebrew (Linuxbrew) system-wide.
#
# Homebrew handles both runtimes and their library dependencies (including
# precompiled formulae), which is what the devtools base image delegates
# package management to. No language -dev apt packages need to live in the
# base image as a result: a project that needs PHP drops a Brewfile with
# `brew "php"` and the entrypoint's `brew bundle install` pulls everything
# including libsqlite3, libiconv, libsodium, and friends as brew deps.
#
# Install target is /home/linuxbrew/.linuxbrew (the distribution-blessed
# prefix). At runtime we mount a shared `devtools_brew` named volume there
# so formula installs are shared across every project on this host.
set -euo pipefail

# Homebrew requires a non-root user to run. Create `linuxbrew` as a system
# user with its own home at the expected prefix.
groupadd -r linuxbrew || true
useradd -r -m -s /bin/bash -g linuxbrew -d /home/linuxbrew linuxbrew

# `useradd -m` creates /home/linuxbrew as 0700 by default, which blocks
# the `dev` user from traversing into the brew prefix. Open the home
# (and the prefix) so group members can read and exec, while everything
# below stays brew-owned.
chmod 0755 /home/linuxbrew
install -d -o linuxbrew -g linuxbrew -m 0755 /home/linuxbrew/.linuxbrew

# Non-interactive install. The official installer would `curl | bash` and
# prompt; we inline its git clone + env bootstrap so the image build is
# reproducible without stdin tricks.
sudo -u linuxbrew git clone --depth=1 \
    https://github.com/Homebrew/brew \
    /home/linuxbrew/.linuxbrew/Homebrew

install -d -o linuxbrew -g linuxbrew /home/linuxbrew/.linuxbrew/bin
ln -sf /home/linuxbrew/.linuxbrew/Homebrew/bin/brew /home/linuxbrew/.linuxbrew/bin/brew

# Expose brew on PATH system-wide for every shell. Also set up shellenv
# (HOMEBREW_PREFIX, MANPATH, INFOPATH) — brew's own init does this, we just
# source it at login.
cat > /etc/profile.d/brew.sh <<'PROFILE'
# Homebrew (Linuxbrew): set HOMEBREW_PREFIX and prepend brew's bin to PATH.
if [ -x /home/linuxbrew/.linuxbrew/bin/brew ]; then
    eval "$(/home/linuxbrew/.linuxbrew/bin/brew shellenv 2>/dev/null || true)"
fi
PROFILE
chmod 0644 /etc/profile.d/brew.sh

# Run a one-shot `brew update-reset` as the linuxbrew user to pre-warm the
# tap and verify the install succeeded. Non-fatal on network hiccups; the
# first real `brew bundle` call will try again anyway.
sudo -u linuxbrew /home/linuxbrew/.linuxbrew/bin/brew update-reset >/dev/null 2>&1 || true
sudo -u linuxbrew /home/linuxbrew/.linuxbrew/bin/brew --version

rm -f /tmp/20-brew.sh
