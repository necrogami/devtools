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
# Layer 1: stable system packages + compile deps for mise-built runtimes.
# Most cache-stable layer — leave it on top.
# -----------------------------------------------------------------------------
COPY base/install/10-apt.sh /tmp/10-apt.sh
RUN /tmp/10-apt.sh

# -----------------------------------------------------------------------------
# Layer 2: mise (runtime version manager). Runtimes themselves are NOT
# installed at build time — they land in the shared devtools_mise volume
# at first `dev up` per project.
# -----------------------------------------------------------------------------
COPY base/install/20-mise.sh /tmp/20-mise.sh
RUN /tmp/20-mise.sh

# -----------------------------------------------------------------------------
# Layer 3: modern CLI tools from apt + upstream binaries (yq, oh-my-posh, gh).
# -----------------------------------------------------------------------------
COPY base/install/30-cli-tools.sh /tmp/30-cli-tools.sh
RUN /tmp/30-cli-tools.sh

# -----------------------------------------------------------------------------
# Layer 4: AI tooling (Claude Code). Churns most on upstream releases —
# put last so lower layers stay cached across rebuilds.
# -----------------------------------------------------------------------------
COPY base/install/40-ai-tools.sh /tmp/40-ai-tools.sh
RUN /tmp/40-ai-tools.sh

# -----------------------------------------------------------------------------
# Non-root user + sudo-NOPASSWD for adhoc installs inside the container.
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
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.local/bin \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.local/share \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /home/${USERNAME}/.local/share/mise \
 && install -d -m 0755 -o ${USERNAME} -g ${GID} /run/host

COPY --chown=${UID}:${GID} base/skel/ /etc/skel/
COPY base/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod 0755 /usr/local/bin/entrypoint.sh

ENV HOME=/home/${USERNAME} \
    MISE_DATA_DIR=/home/${USERNAME}/.local/share/mise \
    PATH=/home/${USERNAME}/.local/share/mise/shims:/home/${USERNAME}/.local/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin

USER ${USERNAME}
WORKDIR /code

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["bash", "-l"]
