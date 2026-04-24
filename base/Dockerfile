# syntax=docker/dockerfile:1.7

FROM debian:trixie-slim

ARG USERNAME=dev
ARG UID=1000
ARG GID=1000

ENV DEBIAN_FRONTEND=noninteractive \
    LANG=en_US.UTF-8 \
    LC_ALL=en_US.UTF-8 \
    LANGUAGE=en_US:en \
    TZ=UTC \
    TERM=xterm-256color

# -----------------------------------------------------------------------------
# Layer 1: minimum system packages. No language -dev libs here — brew handles
# that at container-start via Brewfile.
# -----------------------------------------------------------------------------
COPY base/install/10-apt.sh /tmp/10-apt.sh
RUN /tmp/10-apt.sh

# -----------------------------------------------------------------------------
# Layer 2: interactive-shell CLI tools — oh-my-posh prompt (with Atomic
# theme), yq, gh. Runs before the skel copy because 30-cli-tools.sh
# populates /etc/skel/.config/oh-my-posh/ itself; putting it before the
# user copy means the final /etc/skel contains both our custom dotfiles
# and the upstream-installed theme, and useradd -m seeds both.
# -----------------------------------------------------------------------------
COPY base/install/30-cli-tools.sh /tmp/30-cli-tools.sh
RUN /tmp/30-cli-tools.sh

# -----------------------------------------------------------------------------
# Custom /etc/skel — MUST be in place BEFORE `useradd -m` so the new user's
# $HOME is populated from *our* skel (our .bashrc, .inputrc, etc.) rather
# than Debian's stock defaults. A named volume mounted over /home/dev at
# runtime is initialized from the image's /home/dev on first mount, so
# getting this right here is what makes the prompt and completions work in
# every freshly-provisioned project without a reseed dance.
# -----------------------------------------------------------------------------
COPY --chown=${UID}:${GID} base/skel/ /etc/skel/

# -----------------------------------------------------------------------------
# Non-root user + sudo-NOPASSWD for adhoc installs inside the container.
# Must happen BEFORE the brew install layer so the prefix can be owned by
# ${USERNAME} from the start — brew requires the running user to own its
# prefix, not just have group access.
# -----------------------------------------------------------------------------
RUN groupadd -g ${GID} ${USERNAME} \
 && useradd -m -u ${UID} -g ${GID} -s /bin/bash ${USERNAME} \
 && install -d -m 0750 /etc/sudoers.d \
 && echo "${USERNAME} ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/${USERNAME} \
 && chmod 0440 /etc/sudoers.d/${USERNAME} \
 && install -d -m 0700 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.gnupg \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.config \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.config/gh \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.claude \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.claude/agents \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.claude/skills \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.claude/commands \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.claude/plugins \
 && touch                                       /home/${USERNAME}/.claude/settings.json \
 && touch                                       /home/${USERNAME}/.claude/CLAUDE.md \
 && chown ${USERNAME}:${GID}                    /home/${USERNAME}/.claude/settings.json /home/${USERNAME}/.claude/CLAUDE.md \
 # `install -d -o X -g Y path/a/b/c` chowns only the leaf (c); intermediates
 # (path, path/a, path/a/b) get default root:root. Create each level we care
 # about explicitly so runtime tools can actually write inside their XDG dirs.
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.local \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.local/bin \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.local/share \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.local/share/pnpm \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.local/share/pnpm/store \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.local/state \
 # Shared-volume mount parents. Pre-create these as ${USERNAME}-owned so that
 # when compose mounts e.g. devtools_pip at /home/dev/.cache/pip, Docker's
 # auto-created /home/dev/.cache parent inherits our ownership instead of
 # defaulting to root.
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.cache \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.cache/pip \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.cargo \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.cargo/registry \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.composer \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.composer/cache \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.npm \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/go \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/go/pkg \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/go/pkg/mod \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /run/host

# -----------------------------------------------------------------------------
# Layer 4: Homebrew (Linuxbrew). Installs into /home/linuxbrew/.linuxbrew
# owned by ${USERNAME}, so `brew install`/`brew bundle install` inside the
# container Just Works™ as the dev user. The shared devtools_brew volume
# mounted at /home/linuxbrew/.linuxbrew at runtime dedupes installs across
# every project on this host.
# -----------------------------------------------------------------------------
COPY base/install/20-brew.sh /tmp/20-brew.sh
RUN USERNAME=${USERNAME} /tmp/20-brew.sh

# -----------------------------------------------------------------------------
# Layer 5: AI tooling (Claude Code). Churns most on upstream releases —
# put last so lower layers stay cached across rebuilds.
# -----------------------------------------------------------------------------
COPY base/install/40-ai-tools.sh /tmp/40-ai-tools.sh
RUN /tmp/40-ai-tools.sh

COPY base/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod 0755 /usr/local/bin/entrypoint.sh

ENV HOME=/home/${USERNAME} \
    HOMEBREW_PREFIX=/home/linuxbrew/.linuxbrew \
    HOMEBREW_CELLAR=/home/linuxbrew/.linuxbrew/Cellar \
    HOMEBREW_REPOSITORY=/home/linuxbrew/.linuxbrew/Homebrew \
    PATH=/home/linuxbrew/.linuxbrew/bin:/home/linuxbrew/.linuxbrew/sbin:/home/${USERNAME}/.local/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin

USER ${USERNAME}
WORKDIR /code

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["bash", "-l"]
