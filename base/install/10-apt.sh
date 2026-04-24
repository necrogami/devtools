#!/usr/bin/env bash
# 10-apt.sh — minimal system packages for the devtools base image.
#
# This layer is intentionally slim: everything runtime/library-related that
# projects actually need (PHP extensions, Python build deps, DB client libs,
# etc.) is installed at container-start time via Homebrew (see 20-brew.sh
# and the entrypoint's `brew bundle install` step). Keeping this layer tiny
# means a faster `docker pull` and a smaller delta when Debian base updates.
set -euo pipefail

apt-get update

# Things every interactive dev shell wants, and the pieces brew itself needs
# to bootstrap (curl, git, build-essential for source-only formulae).
BASE_PKGS=(
    ca-certificates curl wget gnupg openssh-client
    git build-essential file
    unzip xz-utils zstd
    less sudo locales tzdata
    bash-completion tmux vim-nox nano
    procps iproute2 dnsutils htop
    jq
)

# CLI tools available in trixie apt where the packaged version is fresh
# enough. Everything else (ripgrep, fd, bat, fzf, eza, yq, gh, oh-my-posh)
# can be `brew install`ed per-project if a project needs newer.
# We keep the apt versions of these in the base because they're used by
# interactive shells and should work even if a project has no Brewfile.
APT_CLI_PKGS=(
    ripgrep fd-find bat fzf eza
)

apt-get install -y --no-install-recommends \
    "${BASE_PKGS[@]}" \
    "${APT_CLI_PKGS[@]}"

# Locale generation — oh-my-posh and many tools render better with UTF-8.
sed -i '/^# *en_US.UTF-8 UTF-8/s/^# *//' /etc/locale.gen
locale-gen en_US.UTF-8
update-locale LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8

# Debian ships `fd-find` and `bat` with renamed binaries (`fdfind`, `batcat`)
# to avoid namespace clashes. Symlink so muscle memory works.
install -d /usr/local/bin
[ -e /usr/local/bin/fd ]  || ln -s /usr/bin/fdfind /usr/local/bin/fd
[ -e /usr/local/bin/bat ] || ln -s /usr/bin/batcat /usr/local/bin/bat

# Cleanup.
apt-get clean
rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/* /tmp/10-apt.sh
