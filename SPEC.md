# devtools — Design Spec

**Status:** Approved design, pre-implementation
**Date:** 2026-04-23
**Repo:** `github.com/necrogami/devtools`
**Image:** `ghcr.io/necrogami/devtools`
**CLI binary:** `dev`

## 1. Goal

A personal "developer-host" system that lets a single remote server (or laptop) run many mutually-isolated project stacks from one shared, pre-baked toolbox image. SSH into the host, pick a project, `docker exec` into its tools container, and everything you need (claude, git, gh, shell tooling) is already there. Language runtimes and project-specific libraries install on demand via `Brewfile` (Homebrew), with a shared cellar across projects. Cross-project code visibility is prevented; cross-project downloads are cached.

## 2. Non-Goals

- Not a multi-tenant dev platform for other users
- Not a CI/CD platform (separate concern)
- Not a replacement for production deploy tooling
- No Kubernetes-in-container / k3d-style orchestration in v1
- No VS Code / Cursor Remote integration hooks in v1 (the model is SSH + `docker exec`, not Remote-Containers)

## 3. Usage Pattern (reference)

```
laptop ──ssh──► remote host
                 ├── ssh-agent (keys loaded here, only)
                 ├── gpg-agent (keys loaded here, only)
                 ├── docker engine
                 │
                 ├── project: ezfleet
                 │    ├── ezfleet-tools  (from devtools image)
                 │    ├── ezfleet-db     (mariadb)
                 │    └── volumes: code, home, shared caches
                 │
                 ├── project: leadsrx
                 │    ├── leadsrx-tools
                 │    ├── leadsrx-db
                 │    └── ...isolated...
                 │
                 └── shared volumes: devtools_brew, devtools_composer,
                                     devtools_npm, devtools_pnpm,
                                     devtools_cargo, devtools_gomod,
                                     devtools_pip
```

## 4. Key Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Usage model | Ride-inside dev box (ssh + docker exec) | Remote-first, isolation is the goal |
| Isolation unit | Per-project container + per-project volumes | Strong security boundary between divergent projects |
| Code storage | Named Docker volume, not host bind-mount | Reduces container→host filesystem escape surface |
| Credential model | Agent socket forwarding (SSH & GPG) | Private keys never enter any container |
| Runtime + library mgmt | Homebrew (Linuxbrew) with per-project `Brewfile` | Precompiled formulae, brew owns both runtime + its library deps |
| Base OS | `debian:trixie-slim` (Debian 13, current stable) | glibc, broadest PHP/Rust/Node compat, small |
| Architectures | `linux/amd64` + `linux/arm64` | Mac (M-series) + amd64 remote servers |
| UID/GID strategy | Build-arg `UID`/`GID` (default 1000) | Match host agent-socket ownership |
| Shared caches | External named volumes for pkg-mgr caches AND Claude plugins/skills/commands | Fast project spin-up; install-once plugins/skills across projects |
| Management CLI | Go (Cobra + fang) | Native Docker SDK fit, multi-arch trivial, tiny binary |
| CLI deferred deps | `lipgloss`, `bubbletea` added when features need them | YAGNI; keep startup fast and binary small |

## 5. Repo Layout

```
devtools/
├── base/                       # The main image (tracked)
│   ├── Dockerfile              # FROM debian:trixie-slim
│   ├── entrypoint.sh
│   ├── smoke-test.sh           # used by CI after build
│   ├── skel/                   # default dotfiles, copied into $HOME on first start
│   │   ├── .bashrc
│   │   ├── .bash_aliases
│   │   ├── .inputrc
│   │   ├── .tmux.conf
│   │   └── .config/oh-my-posh/atomic.omp.json   # pinned copy; fallback if $POSH_THEMES_PATH absent
│   └── install/                # split-stage install scripts
│       ├── 10-apt.sh
│       ├── 20-brew.sh
│       ├── 30-cli-tools.sh
│       └── 40-ai-tools.sh
├── template/                   # copied to `projects/<name>/` by `dev new`
│   ├── docker-compose.yml
│   ├── .env.example
│   ├── Brewfile.example
│   └── README.md
├── projects/                   # gitignored — your real project stacks
│   └── .gitkeep
├── shared/
│   └── init-volumes.sh         # redundant but kept for non-CLI recovery
├── cmd/
│   └── dev/
│       ├── main.go
│       ├── root.go             # cobra root + fang wiring
│       ├── build.go
│       ├── push.go
│       ├── new.go
│       ├── up.go               # up / down / shell / exec / logs / ps
│       ├── bump.go
│       ├── init_shared.go
│       ├── list.go
│       └── doctor.go
├── internal/
│   ├── compose/                # YAML manipulation (bump tags, validate) + override renderer
│   ├── hostenv/                # host credential / agent discovery for `dev up`
│   ├── dockerclient/           # thin wrapper over docker/client
│   ├── tmpl/                   # `dev new` templating
│   ├── paths/                  # canonical path resolution (projects/, template/)
│   └── host/                   # host inspection for `doctor`
├── go.mod
├── go.sum
├── .goreleaser.yml
├── install.sh                  # fetches latest `dev` for host arch from GH Releases
├── .github/
│   ├── workflows/
│   │   ├── build-image.yml     # base image → GHCR (push to base/** + weekly)
│   │   ├── build-cli.yml       # dev binary → GitHub Releases (on tag v*)
│   │   └── test.yml            # vet + race tests + cross-compile matrix
│   ├── ISSUE_TEMPLATE/
│   │   ├── config.yml          # disables blank issues, pins helpful links
│   │   ├── bug_report.yml      # requires `dev doctor`/`dev version` output
│   │   ├── feature_request.yml # problem-first; contributor-willingness
│   │   └── docs.yml
│   └── PULL_REQUEST_TEMPLATE.md # summary, type/area, test plan, breaking-change prompt
├── .gitignore                  # projects/* except .gitkeep
├── LICENSE                     # MIT
├── SPEC.md                     # this file
└── README.md
```

## 6. Base Image

### 6.1 Dockerfile

```dockerfile
FROM debian:trixie-slim
ARG USERNAME=dev
ARG UID=1000
ARG GID=1000
ENV DEBIAN_FRONTEND=noninteractive \
    LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 TZ=UTC

COPY base/install/10-apt.sh /tmp/
RUN /tmp/10-apt.sh

COPY base/install/20-brew.sh /tmp/
RUN /tmp/20-brew.sh

COPY base/install/30-cli-tools.sh /tmp/
RUN /tmp/30-cli-tools.sh

COPY base/install/40-ai-tools.sh /tmp/
RUN /tmp/40-ai-tools.sh

RUN groupadd -g ${GID} ${USERNAME} \
 && useradd -m -u ${UID} -g ${GID} -s /bin/bash ${USERNAME} \
 && echo "${USERNAME} ALL=(ALL) NOPASSWD:ALL" > /etc/sudoers.d/${USERNAME}

COPY --chown=${UID}:${GID} base/skel/ /etc/skel/
COPY base/entrypoint.sh /usr/local/bin/entrypoint.sh

USER ${USERNAME}
WORKDIR /code
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["bash", "-l"]
```

### 6.2 Install scripts (summary)

- **`10-apt.sh`** — the minimum to bootstrap an interactive shell and Homebrew: ca-certificates, curl, wget, gnupg, openssh-client, git, build-essential, file, unzip/xz-utils/zstd, less, sudo, locales, tzdata, bash-completion, tmux, vim-nox, nano, procps/iproute2/dnsutils/htop, jq, plus the apt-packaged interactive tools ripgrep/fd-find/bat/fzf/eza. Locale gen en_US.UTF-8. Deliberately **no** language -dev packages — Homebrew handles those per-project via Brewfile.
- **`20-brew.sh`** — installs Homebrew (Linuxbrew) into `/home/linuxbrew/.linuxbrew`, owned by a dedicated `linuxbrew` system user. Writes `/etc/profile.d/brew.sh` that sources `brew shellenv` for every login shell. Pre-warms the tap. Runtime installs happen at container-start via the entrypoint's `brew bundle install` against `/code/Brewfile`; the shared `devtools_brew` volume mounts over the prefix so formulae are reused across every project on the host.
- **`30-cli-tools.sh`** — installs from upstream: yq (mikefarah/yq GitHub release), **oh-my-posh** (via `https://ohmyposh.dev/install.sh`, exposes `POSH_THEMES_PATH` at `/usr/local/share/oh-my-posh/themes`), gh (via official apt repo). Also writes `/etc/profile.d/devtools-shell.sh` — the system-wide interactive-shell init that activates oh-my-posh (with `--print`), sets `XDG_CACHE_HOME`, and wires fzf keybindings. Appends a one-line `source` of it to `/etc/bash.bashrc` so non-login interactive shells (`docker exec -it … bash`) pick it up too.
- **`40-ai-tools.sh`** — installs Claude Code via Anthropic's official installer (bundles its own runtime; no global node needed in the image). Verifies `claude --version` at end. Codex install left commented as opt-in.

### 6.3 Entrypoint

```bash
#!/usr/bin/env bash
set -euo pipefail

# 1. Seed $HOME from /etc/skel on first start (empty volume)
if [ -z "$(ls -A "$HOME" 2>/dev/null)" ]; then
    cp -rT /etc/skel "$HOME"
fi

# 2. SSH agent wiring
if [ -S /run/host/ssh-agent ]; then
    export SSH_AUTH_SOCK=/run/host/ssh-agent
fi

# 3. GPG agent wiring
if [ -S /run/host/gpg-agent ]; then
    mkdir -p "$HOME/.gnupg"
    chmod 700 "$HOME/.gnupg"
    ln -sf /run/host/gpg-agent "$HOME/.gnupg/S.gpg-agent"
    export GPG_TTY
    GPG_TTY=$(tty 2>/dev/null || echo /dev/console)
fi

# 4. Brewfile bundle install (non-blocking, best-effort)
if [ -f /code/Brewfile ] && command -v brew >/dev/null 2>&1; then
    ( brew bundle install --file=/code/Brewfile >/tmp/brew-bundle.log 2>&1 & ) || true
fi

exec "$@"
```

## 7. Credential Flow

Credential and agent mounts are **not** part of the static `docker-compose.yml`. They live in a runtime-generated `docker-compose.override.yml` that `dev up` writes next to the base compose file on every invocation, based on what `internal/hostenv.Discover` finds on the host right now. Compose auto-loads the override. The override file is `.gitignored` and is safe to delete — `dev up` rewrites it.

Host-side discovery (each source optional; if absent, the corresponding bind-mount is simply omitted):
- `ssh-agent` — socket at `$SSH_AUTH_SOCK`
- `gpg-agent` — socket path from `gpgconf --list-dirs agent-socket`, falling back to `$XDG_RUNTIME_DIR/gnupg/S.gpg-agent`, then `~/.gnupg/S.gpg-agent`
- GPG public material: whichever of `~/.gnupg/{public-keys.d,pubring.kbx,trustdb.gpg,ownertrust}` exist
- `~/.config/gh/` — gh CLI config + token (recommendation: fine-grained PAT)
- `~/.gitconfig` — commit-signing / identity config
- `~/.claude/{settings.json,CLAUDE.md,agents/}` — Claude personalization

Container-side destinations (read-only except sockets):
- Socket: host ssh-agent socket → `/run/host/ssh-agent` (and `SSH_AUTH_SOCK=/run/host/ssh-agent` is set only when the mount is present)
- Socket: host gpg-agent socket → `/run/host/gpg-agent`
- Dir (ro): `~/.gnupg/public-keys.d` → `/home/dev/.gnupg/public-keys.d`
- File (ro): `~/.gnupg/pubring.kbx` → `/home/dev/.gnupg/pubring.kbx`
- File (ro): `~/.gnupg/trustdb.gpg` → `/home/dev/.gnupg/trustdb.gpg`
- File (ro): `~/.gnupg/ownertrust` → `/home/dev/.gnupg/ownertrust`
- Dir (ro): `~/.config/gh` → `/home/dev/.config/gh`
- File (ro): `~/.gitconfig` → `/home/dev/.gitconfig`
- File (ro): `~/.claude/settings.json` → `/home/dev/.claude/settings.json`
- File (ro): `~/.claude/CLAUDE.md` → `/home/dev/.claude/CLAUDE.md`
- Dir (ro): `~/.claude/agents` → `/home/dev/.claude/agents`

Claude Code artifacts shared across projects via external named volumes
(seeded from host ~/.claude on first `dev init-shared`):
- Volume (rw): `devtools_claude_plugins` → `/home/dev/.claude/plugins`
- Volume (rw): `devtools_claude_skills` → `/home/dev/.claude/skills`
- Volume (rw): `devtools_claude_commands` → `/home/dev/.claude/commands`

Claude auth (`~/.claude/credentials.*`) and per-project state (`~/.claude/projects/`)
are deliberately NOT shared: each project container does its own
`claude login` (stored in its isolated `home` volume) and maintains its own
conversation/memory. This preserves project isolation even for Claude state,
and lets you log in with different accounts per project (e.g., work vs personal).

Private keyfiles never enter any container. UID match (container user == host agent socket owner) is required; enforced by build-arg + `dev doctor`.

## 8. Project Compose Template

`template/docker-compose.yml` (copied by `dev new`; project supplies `.env` with `PROJECT=<name>` and optionally `DEVTOOLS_TAG=<tag>`).

Key properties:
- `image: ghcr.io/necrogami/devtools:${DEVTOOLS_TAG:-latest}`
- `container_name: ${PROJECT}-tools`
- Named per-project volumes: `code`, `home`
- External shared volumes: `devtools_brew` (Homebrew cellar), `devtools_composer`, `devtools_npm`, `devtools_pnpm`, `devtools_cargo`, `devtools_gomod`, `devtools_pip`
- External shared Claude volumes: `devtools_claude_plugins`, `devtools_claude_skills`, `devtools_claude_commands`
- Credential mounts as in §7
- Service stubs commented out: `db` (mariadb), `redis`, `queue` (beanstalkd/rabbit), `mail` (mailpit)

## 9. `dev` CLI

### 9.1 Commands

| Command | Purpose |
|---|---|
| `dev build [--push] [--platform amd64,arm64]` | Build the base image locally via buildx |
| `dev push [--tag X]` | Push to GHCR; if no `--tag`, push `latest` + today's date |
| `dev new <name>` | Scaffold `projects/<name>/` from `template/` |
| `dev up <name>` | `docker compose up -d` for a project |
| `dev down <name> [--volumes]` | `docker compose down` (optional volume wipe) |
| `dev shell <name>` | `docker exec -it <name>-tools bash -l` |
| `dev exec <name> -- <cmd...>` | One-shot command in `<name>-tools` |
| `dev logs <name> [--follow] [service]` | Tail compose logs |
| `dev ps` | Table of all known projects + running state |
| `dev list` | Same as `ps` but offline (no Docker queries) |
| `dev bump <name> [--tag X]` | Rewrite `projects/<name>/.env`'s `DEVTOOLS_TAG` |
| `dev init-shared [--no-seed]` | Idempotent creation of all 10 shared volumes (7 cache + 3 Claude). Seeds Claude volumes from host `~/.claude/{plugins,skills,commands}` on first creation unless `--no-seed` |
| `dev update [--check] [--tag X] [--force] [--image]` | Self-update the CLI from the latest GitHub release. SHA-256 verification against `checksums.txt`; atomic rename-over-self. `--image` also `docker pull`s the latest base image |
| `dev doctor` | Host health check (see §9.2) |
| `dev version` | Build info + image info |

### 9.2 `dev doctor` checks

Each reports PASS / WARN / FAIL with a remediation hint:
1. Docker daemon reachable; Engine API ≥ 1.44
2. `docker buildx` plugin installed
3. `SSH_AUTH_SOCK` set + socket exists + `ssh-add -l` lists ≥1 key
4. `gpg-agent` socket present at `$XDG_RUNTIME_DIR/gnupg/S.gpg-agent`
5. Host user UID/GID == image's baked UID/GID (1000/1000 default)
6. GHCR auth works (manifest inspect on `latest`)
7. All 10 shared volumes exist (7 cache + 3 Claude) — offer `dev init-shared` if not
8. `~/.config/gh/hosts.yml` present + token lint (warn if looks like classic PAT)

### 9.3 Internal packages

- `internal/compose` — reads/writes `projects/<name>/{.env,docker-compose.yml}`; exposes `BumpImageTag(project, tag)`, `Validate(path)`, `ProjectDir(name)`. Also renders the runtime `docker-compose.override.yml` (`RenderOverride`, `WriteOverride`) from a `hostenv.HostCreds` value. Uses `gopkg.in/yaml.v3` for base compose manipulation and plain string building for the override (so conditional mounts stay readable).
- `internal/hostenv` — pure-Go discovery of host credentials / agent sockets / personalization files. `Discover(home) HostCreds` probes `$SSH_AUTH_SOCK`, `gpgconf --list-dirs agent-socket` (with `$XDG_RUNTIME_DIR/gnupg/` and `~/.gnupg/` fallbacks), `~/.gnupg/{public-keys.d,pubring.kbx,trustdb.gpg,ownertrust}`, `~/.gitconfig`, `~/.config/gh`, and `~/.claude/{settings.json,CLAUDE.md,agents}`. Empty fields mean "not available — don't mount."
- `internal/dockerclient` — wraps `github.com/docker/docker/client`. v1 methods: `PingDaemon()`, `VolumeExists(name)`, `VolumeCreate(name)`, `ContainerInspect(name)`. Used by `doctor`, `init-shared`, `ps`.
- `internal/tmpl` — loads `template/` via `os.DirFS`, substitutes variables (`{{.Project}}`, `{{.DevtoolsTag}}`), writes to destination.
- `internal/paths` — resolves `repoRoot`, `templateDir`, `projectDir(name)` with proper validation (name is `^[a-z][a-z0-9-]{0,30}$`).
- `internal/host` — `doctor` implementations (each check is a `Check` struct returning `Status{Level, Message, Fix}`).

## 10. Build / Publish / Release

### 10.0 Test workflow (`.github/workflows/test.yml`)

- Trigger: every push to any branch, every PR
- Steps: `go vet ./...` → `go test -race -coverprofile=coverage.out ./...` → coverage summary → cross-compile matrix (`linux|darwin × amd64|arm64`)
- Uploads `coverage.out` as a run artifact
- Required-status for merging (manual setting on the repo)

### 10.1 Image workflow (`.github/workflows/build-image.yml`)

- Trigger: push to `main` affecting `base/**`; manual dispatch; weekly cron (Sundays 06:00 UTC)
- Steps: checkout → QEMU → buildx → build+push multi-arch → run `smoke-test.sh` in the just-pushed image per arch → tag `latest`, `YYYY-MM-DD`, `sha-<short>`
- Registry: GHCR (`ghcr.io/necrogami/devtools`)
- Optional later: cosign signing + provenance attestation

### 10.2 CLI workflow (`.github/workflows/build-cli.yml`)

- Trigger: git tag matching `v*.*.*`
- Tool: GoReleaser (4-line `.goreleaser.yml`)
- Matrix: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`
- Artifacts: `dev_<os>_<arch>` tarballs attached to GitHub Release
- Install paths for end users (documented in README):
  - `curl -sSL https://raw.githubusercontent.com/necrogami/devtools/main/install.sh | bash` (detects arch, downloads matching release asset into `~/.local/bin`)
  - `go install github.com/necrogami/devtools/cmd/dev@latest` (Go toolchain on host)
- No binaries are committed to the repo

### 10.3 Project bump

`dev bump ezfleet --tag 2026-04-23` does this atomically:
1. Read `projects/ezfleet/.env`
2. Replace `DEVTOOLS_TAG=...` (insert if missing)
3. Write with matching perms
4. Print: "Bumped ezfleet to 2026-04-23. Run `dev up ezfleet` to apply."

## 11. Testing

- **CLI unit tests** (`go test ./...`): `internal/compose` (bump/validate roundtrips on fixture files), `internal/tmpl` (scaffold output matches golden files), `internal/paths` (name validation, edge cases)
- **CLI integration tests** (`go test -tags integration`): gated behind `DEVTOOLS_INTEGRATION=1`; exercise `dev new` → `dev up` → `dev exec echo hi` → `dev down` against real Docker daemon; cleanup uses `t.Cleanup` with `docker compose down -v`
- **Image smoke test** (`base/smoke-test.sh`): invoked in CI after image push; `docker run --rm <image> bash -c 'for t in bash git gh brew claude curl ssh gpg jq yq rg fd bat fzf eza oh-my-posh tmux vim; do command -v "$t" >/dev/null || exit 1; done'` plus version banner for each tool
- **Doctor self-test**: `dev doctor --dry-run` runs each check against a synthetic host state to verify the remediation hints render correctly

## 12. Security Posture

- Non-root default user (`dev`, UID 1000) inside container
- Per-project volumes prevent cross-project code visibility
- Agent-socket-only credential model; keys never leave host
- `.env` files always gitignored; `.env.example` tracked
- Compose secrets used for service credentials (DB root pw, etc.)
- Image signed by cosign (phase 2)
- GHCR pulls authenticated via `docker login ghcr.io` on host
- Sudoers NOPASSWD for `dev` user — accepted risk: container is ephemeral, attack surface is already high (full toolchain + claude-code inside)
- Known accepted risk: a compromised container *can* use the forwarded SSH/GPG agent during its lifetime. Mitigations: short-lived agent entries (`ssh-add -t 3600`), fine-grained gh PAT, signing-only GPG subkey instead of full master key.

## 13. Out-of-Scope (future)

- Multi-user support on same host
- VS Code Remote-Containers / devcontainer.json wiring
- Automatic project template variants (PHP-only, Go-only, etc.)
- Volume backup/restore commands
- Tailscale/WireGuard sidecar for zero-trust remote access
- Automatic dependency-update PRs for base image

## 14. Implementation Order

1. `base/` Dockerfile + install scripts + entrypoint + skel dotfiles + smoke-test
2. `cmd/dev` + `internal/paths` + `internal/compose`: `new`, `up`, `down`, `shell`, `exec`, `logs`, `ps`
3. `internal/dockerclient` + `init-shared` + `doctor`
4. `bump`, `list`, `version`
5. GHA: `build-image.yml` (ship first to unblock real use)
6. GoReleaser + `build-cli.yml`
7. Documentation pass (`README.md`, per-command help text polish)
8. First real project migrated to the new system (likely `ezfleet`) as dog-food shakedown
