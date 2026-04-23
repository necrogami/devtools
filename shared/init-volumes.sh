#!/usr/bin/env bash
# shared/init-volumes.sh — create devtools shared cache volumes.
#
# Idempotent: skips volumes that already exist. `dev init-shared` also
# creates these via the Docker SDK; this script is the portable fallback
# for bootstrapping before the CLI is installed, or for recovery.
set -euo pipefail

VOLUMES=(
    # Package-manager download caches.
    devtools_mise              # mise installs + runtimes
    devtools_composer          # composer global cache
    devtools_npm               # npm global cache
    devtools_pnpm              # pnpm content-addressable store
    devtools_cargo             # cargo registry cache
    devtools_gomod             # go module cache
    devtools_pip               # pip wheel/download cache
    # Claude install-once-use-everywhere.
    devtools_claude_plugins    # Claude plugin cache
    devtools_claude_skills     # Claude user skills
    devtools_claude_commands   # Claude user commands
)

created=0
skipped=0
for v in "${VOLUMES[@]}"; do
    if docker volume inspect "$v" >/dev/null 2>&1; then
        printf "  = %s (exists)\n" "$v"
        skipped=$((skipped + 1))
    else
        docker volume create "$v" >/dev/null
        printf "  + %s\n" "$v"
        created=$((created + 1))
    fi
done

printf "\n%d created, %d already existed.\n" "$created" "$skipped"
