#!/usr/bin/env bash
# 10-apt.sh — system packages + mise compile deps + db clients.
set -euo pipefail

apt-get update

# Split into groups for readability / easier auditing.
BASE_PKGS=(
    ca-certificates curl wget gnupg openssh-client
    git unzip less sudo locales tzdata
    bash-completion tmux vim-nox nano
    procps iproute2 dnsutils htop file
    jq xz-utils zstd
)

# CLI tools available in trixie apt (prefer over GH releases when version is fresh).
APT_CLI_PKGS=(
    ripgrep fd-find bat fzf eza
)

# Compile deps so `mise install <lang>@<ver>` can build from source when no
# prebuilt is available (happens for uncommon PHP/Python/Ruby versions).
BUILD_DEPS=(
    build-essential pkg-config autoconf bison
    libssl-dev libicu-dev libxml2-dev libzip-dev libonig-dev
    libffi-dev liblzma-dev libyaml-dev libsqlite3-dev libpq-dev
    libcurl4-openssl-dev libreadline-dev libbz2-dev libncurses-dev
    libxslt1-dev libpng-dev libjpeg-dev zlib1g-dev libgmp-dev
)

# Database clients — convenience for connecting into service containers.
DB_CLIENTS=(
    mariadb-client postgresql-client redis-tools sqlite3
)

apt-get install -y --no-install-recommends \
    "${BASE_PKGS[@]}" \
    "${APT_CLI_PKGS[@]}" \
    "${BUILD_DEPS[@]}" \
    "${DB_CLIENTS[@]}"

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
