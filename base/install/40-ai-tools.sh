#!/usr/bin/env bash
# 40-ai-tools.sh — AI CLI tooling. Put LAST in the layer order because it
# upgrades most often; keeps layers 10/20/30 cache-warm across rebuilds.
#
# Primary: Claude Code (Anthropic). Uses the official standalone installer
# which ships a self-contained binary (no global node required).
set -euo pipefail

# The installer respects the current HOME. Running as root during image
# build, it places the binary at /root/.local/bin/claude and (on recent
# versions) may drop runtime assets under /root/.claude/. We then relocate
# to a system-wide location so every user-scoped HOME sees `claude` on
# PATH — critical because our per-project home volumes start empty.
curl -fsSL https://claude.ai/install.sh | bash

# ----- locate the installed binary -------------------------------------------
CLAUDE_BIN=""
for candidate in \
    /root/.local/bin/claude \
    /root/.claude/local/claude \
    /root/.anthropic/claude
do
    if [ -x "$candidate" ]; then
        CLAUDE_BIN="$candidate"
        break
    fi
done

if [ -z "$CLAUDE_BIN" ]; then
    echo "Claude installer succeeded but no binary found. /root/ tree:" >&2
    find /root -maxdepth 4 -name 'claude*' -printf '  %p\n' >&2 || true
    exit 1
fi

# ----- relocate into /opt/claude-code + link into /usr/local/bin -------------
install -d /opt/claude-code

# If a runtime directory exists alongside the binary (common for the
# native-build install), move its contents before the binary so the
# installer's expected layout is preserved under /opt/claude-code.
if [ -d /root/.claude/local ]; then
    # Copy rather than move so a symlinked binary inside keeps working.
    cp -a /root/.claude/local/. /opt/claude-code/ 2>/dev/null || true
fi

# Place the launcher binary. Use `install` (copy + chmod) so the original
# source can be anything — symlink, wrapper script, or native ELF.
install -m 0755 "$CLAUDE_BIN" /opt/claude-code/claude

# System-wide PATH entry.
ln -sfn /opt/claude-code/claude /usr/local/bin/claude

# Clean up root's HOME so the image doesn't carry stray install artifacts.
rm -rf /root/.local/bin/claude /root/.claude /root/.anthropic || true

/usr/local/bin/claude --version

rm -f /tmp/40-ai-tools.sh
