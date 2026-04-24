#!/usr/bin/env bash
# entrypoint.sh — runs as the `dev` user on every container start.
#
# Responsibilities, in order:
#   1. Seed $HOME from /etc/skel if this is a fresh volume
#   2. Wire SSH agent socket (if mounted)
#   3. Wire GPG agent + keyboxd sockets (if mounted)
#   4. If /code/Brewfile exists, run `brew bundle install` (non-blocking)
#   5. Hand off to the requested command (default: bash -l)
set -euo pipefail

# --- 1. First-start home seed ------------------------------------------------
if [ -z "$(ls -A "$HOME" 2>/dev/null)" ]; then
    cp -rT /etc/skel "$HOME" || true
fi

# --- 2. SSH agent ------------------------------------------------------------
if [ -S /run/host/ssh-agent ]; then
    export SSH_AUTH_SOCK=/run/host/ssh-agent
fi

# --- 3. GPG agent + keyboxd --------------------------------------------------
if [ -S /run/host/gpg-agent ] || [ -S /run/host/keyboxd ]; then
    install -d -m 0700 "$HOME/.gnupg"
fi
if [ -S /run/host/gpg-agent ]; then
    ln -sfn /run/host/gpg-agent "$HOME/.gnupg/S.gpg-agent"
    if [ -t 0 ]; then
        GPG_TTY="$(tty 2>/dev/null || echo)"
        export GPG_TTY
    fi
fi
# Wire the keyboxd socket so gpg >= 2.3 with `use-keyboxd` in common.conf
# can reach the host's key-storage daemon. Without this the container's gpg
# auto-creates an empty pubring.kbx and reports "no keys" even though
# public-keys.d is bind-mounted in.
if [ -S /run/host/keyboxd ]; then
    ln -sfn /run/host/keyboxd "$HOME/.gnupg/S.keyboxd"
fi

# --- 4. Project Brewfile (non-blocking, best-effort) -------------------------
# If the mounted project has a Brewfile at /code/Brewfile, kick off
# `brew bundle install` in the background. Brew handles its own
# idempotence (formulae already present are no-ops), so re-running is
# cheap, and running in the background keeps first-shell latency low.
# The shared devtools_brew volume (mounted at $HOMEBREW_PREFIX) means any
# formulae installed here benefit every other project on this host too.
if [ -f /code/Brewfile ] && command -v brew >/dev/null 2>&1; then
    ( brew bundle install --file=/code/Brewfile >/tmp/brew-bundle.log 2>&1 \
        && echo "devtools: brew bundle install complete" >> /tmp/brew-bundle.log \
        || echo "devtools: brew bundle install FAILED — see /tmp/brew-bundle.log" >&2
    ) &
fi

# --- 5. Hand off -------------------------------------------------------------
exec "$@"
