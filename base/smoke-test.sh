#!/usr/bin/env bash
# smoke-test.sh — post-build sanity check for a devtools image.
#
# Usage: ./base/smoke-test.sh [image-ref]
#        IMAGE=ghcr.io/necrogami/devtools:sha-abc ./base/smoke-test.sh
#
# Runs a throwaway container and asserts every tool the spec promises is on
# PATH and reports a version. Non-zero exit on any missing tool.
set -euo pipefail

IMAGE="${1:-${IMAGE:-ghcr.io/necrogami/devtools:latest}}"

echo ">>> smoke-testing: $IMAGE"

docker run --rm --entrypoint /bin/bash "$IMAGE" -lc '
set -e
tools=(
    bash git gh mise claude
    curl wget ssh gpg jq yq
    rg fd bat fzf eza
    oh-my-posh tmux vim
)
# Tools that do not support --version; use tool-specific flags.
declare -A version_flag=(
    [ssh]="-V"
    [tmux]="-V"
)
fail=0
echo "== tool presence =="
for t in "${tools[@]}"; do
    if command -v "$t" >/dev/null 2>&1; then
        flag=${version_flag[$t]:---version}
        ver=$("$t" "$flag" 2>&1 | head -n1 || echo present)
        printf "  ok  %-14s %s\n" "$t" "$ver"
    else
        printf "  !!  %-14s MISSING\n" "$t"
        fail=1
    fi
done
echo "== locale =="
locale 2>/dev/null | grep -E "^(LANG|LC_ALL)=" || true
echo "== user / workdir =="
id
pwd
echo "== PATH =="
echo "$PATH"
[ "$fail" -eq 0 ] || { echo "== FAILED =="; exit 1; }
echo "== all checks passed =="
'
