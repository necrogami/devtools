#!/usr/bin/env bash
# entrypoint.sh — runs as the `dev` user on every container start.
#
# Responsibilities, in order:
#   1. Seed $HOME from /etc/skel if this is a fresh volume
#   2. Wire SSH agent socket (if mounted)
#   3. Wire GPG agent socket (if mounted) — public keyring comes in read-only
#   4. Kick off mise install for any project-declared runtimes (non-blocking)
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

# --- 3. GPG agent ------------------------------------------------------------
if [ -S /run/host/gpg-agent ]; then
    install -d -m 0700 "$HOME/.gnupg"
    ln -sfn /run/host/gpg-agent "$HOME/.gnupg/S.gpg-agent"
    # gpg also looks for the browser/extra sockets; symlinking the main socket
    # is enough for signing/ssh/gpg --list-keys to function.
    if [ -t 0 ]; then
        GPG_TTY="$(tty 2>/dev/null || echo)"
        export GPG_TTY
    fi
fi

# --- 4. Project runtime install (non-blocking, best-effort) ------------------
# Only runs when /code contains a mise config; keeps first-start UX snappy.
if [ -x /usr/local/bin/mise ] && { [ -f /code/.mise.toml ] || [ -f /code/.tool-versions ]; }; then
    ( cd /code && /usr/local/bin/mise install >/dev/null 2>&1 & ) || true
fi

# --- 5. Hand off -------------------------------------------------------------
exec "$@"
