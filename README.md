# devtools

> Pre-baked Docker toolbox image + per-project isolated compose stacks, orchestrated by a Go CLI (`dev`).

A personal **developer-host** system: run one SSH connection to a remote box, `docker exec` into any project's tools container, and find every tool you need already configured — bash + oh-my-posh, mise-managed runtimes (PHP/Go/Node/Rust/Python/...), Claude Code, git, gh, db clients, and more. Projects stay isolated from each other by design; download caches stay shared so spinning up a new project is fast.

[![build-image](https://github.com/necrogami/devtools/actions/workflows/build-image.yml/badge.svg)](https://github.com/necrogami/devtools/actions/workflows/build-image.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](./LICENSE)

---

## Why

Running many unrelated projects on one host gets messy: Node version conflicts, global Composer caches with cross-project dependencies, PHP extension drift, and the creeping anxiety that one project's tooling can observe another's code.

`devtools` gives each project its own sealed world:

- **Separate code volume**, separate home volume, separate network — a compromised project container can't see the others' source, shell history, or running services.
- **Shared download caches** (mise, composer, npm, pnpm, cargo, go-mod, pip) mean first-run doesn't re-download every runtime and dependency.
- **Agent-socket credential forwarding** — your SSH and GPG keys stay on the host; containers get sockets, never keyfiles.
- **One image, one CLI** — update the toolbox once; every project picks it up by pinning a date tag.

---

## Quick start

### 1. Install the `dev` CLI

```bash
# Option A: prebuilt binary (recommended for end-users)
curl -sSL https://raw.githubusercontent.com/necrogami/devtools/main/install.sh | bash

# Option B: from source (Go 1.25+ required)
go install github.com/necrogami/devtools/cmd/dev@latest
```

Add `~/.local/bin` to your `$PATH` if it isn't already.

### 2. Clone this repo and bootstrap shared volumes

```bash
git clone git@github.com:necrogami/devtools.git ~/devtools
cd ~/devtools
dev init-shared    # creates 10 named volumes (7 caches + 3 Claude), seeds Claude from ~/.claude
```

### 3. Pull (or build) the base image

```bash
# Pull the latest published image:
docker pull ghcr.io/necrogami/devtools:latest

# Or build it yourself:
dev build --tag local
```

### 4. Create your first project

```bash
dev new myproject              # scaffolds projects/myproject/
dev up myproject               # compose up -d
dev shell myproject            # docker exec -it myproject-tools bash -l
# inside the container:
claude login                   # one-time auth per project (work vs personal accounts possible)
```

That's it. Your new project has its own `/code` volume, its own shell history, and every tool from the base image on `PATH`.

---

## Commands

```text
dev new <name>                        Scaffold projects/<name>/ from template/
dev up <name>                         Start the project's compose stack (-d)
dev down <name> [--volumes]           Stop; optionally wipe volumes
dev shell <name>                      Login shell inside the tools container
dev exec <name> -- <cmd...>           One-shot command inside the tools container
dev logs <name> [service] [--follow]  Tail compose logs
dev ps [name]                         Show running containers (all projects or one)
dev list                              Table of known projects + pinned tag + status
dev bump <name> [--tag X]             Update DEVTOOLS_TAG in projects/<name>/.env
dev init-shared [--no-seed]           Create (and seed) shared volumes on host
dev build [--push] [--platform X,Y]   Build the base image via buildx
dev push [--tag X]                    Alias for build --push
dev update [--check] [--tag X] [--image]
                                      Self-update the CLI from the latest
                                      GitHub release; verifies SHA-256
                                      checksum before atomic replacement.
                                      Pass --image to also pull the latest
                                      base image locally.
dev doctor                            Host health check (docker, agents, volumes, UID)
dev version                           Build info
```

Everything has `--help`; `dev completion {bash,zsh,fish}` prints completion scripts.

---

## Project layout

```
devtools/
├── base/                      The main image
│   ├── Dockerfile             debian:trixie-slim, multi-arch
│   ├── entrypoint.sh          Seeds $HOME from /etc/skel, wires agent sockets,
│   │                          kicks off mise install for project runtimes
│   ├── skel/                  Default dotfiles copied into first-start $HOME
│   │   ├── .bashrc
│   │   ├── .bash_aliases
│   │   ├── .inputrc
│   │   ├── .tmux.conf
│   │   └── .config/mise/config.toml
│   ├── install/               Split-stage install scripts (apt → mise → CLI → AI)
│   └── smoke-test.sh          CI post-build sanity check (19 tools verified)
├── template/                  Scaffold for `dev new`
│   ├── docker-compose.yml
│   ├── .env.example           Rendered → projects/<name>/.env
│   ├── .mise.toml.example     Rendered → projects/<name>/.mise.toml
│   └── README.md
├── projects/                  Your real project stacks (gitignored)
├── shared/init-volumes.sh     Portable fallback for creating shared volumes
├── cmd/dev/                   The `dev` CLI (Go + Cobra + fang)
├── internal/                  Pure-Go helpers (compose, paths, tmpl) + tests
├── install.sh                 End-user installer: fetches matching release asset
├── .goreleaser.yml            Multi-arch release config
└── .github/workflows/         CI: image build (weekly + on change); CLI release on tag
```

See [`SPEC.md`](./SPEC.md) for the full design document.

---

## How it works

### Image layers

| Layer | Contents | Cache behavior |
|---|---|---|
| 1 | apt packages (shell, build deps, db clients, vim-nox, tmux) | Rarely changes |
| 2 | `mise` runtime manager | Monthly-ish |
| 3 | Modern CLI tools (yq, oh-my-posh, gh) | Weekly-ish |
| 4 | Claude Code native binary | Most churn — stays on top |

Rebuilding because Claude Code ships a new version only touches ~30MB of the 456MiB image.

### Per-project isolation

Each project gets its own compose stack with its own networks and volumes:

- `code:/code` — per-project, never shared
- `home:/home/dev` — per-project shell history, mise project-level config, installed plugins
- `devtools_mise:/home/dev/.local/share/mise` — **shared across projects**
- `devtools_{composer,npm,pnpm,cargo,gomod,pip}` — **shared download caches**
- `devtools_claude_{plugins,skills,commands}` — **shared "install once, use everywhere"**

Volume isolation means a compromised project container can't enumerate siblings, read their code, or trigger their shell history. Shared caches only contain **public package artifacts**, not secrets.

### Credential model

Private keys **never** enter any container:

| Credential | Host surface (discovered) | Container surface |
|---|---|---|
| SSH private keys | `~/.ssh/` | *not mounted* — agent socket only |
| GPG private keys | `~/.gnupg/private-keys-v1.d/` | *not mounted* — agent socket only |
| SSH agent socket | `$SSH_AUTH_SOCK` | `/run/host/ssh-agent` |
| GPG agent socket | `gpgconf --list-dirs agent-socket`, falling back to `$XDG_RUNTIME_DIR/gnupg/S.gpg-agent` → `~/.gnupg/S.gpg-agent` | `/run/host/gpg-agent` |
| GPG public material | `~/.gnupg/public-keys.d/` (gpg 2.1+), `pubring.kbx`, `trustdb.gpg`, `ownertrust` — whichever exist | read-only binds |
| gh token | `~/.config/gh/` | read-only bind |
| Git identity | `~/.gitconfig` | read-only bind |
| Claude config | `~/.claude/settings.json`, `CLAUDE.md`, `agents/` | read-only bind |
| Claude auth | *(never shared)* | `claude login` inside each project; token lives in per-project home volume |

A compromised container can *use* your agent during its lifetime (mitigated by `ssh-add -t 3600` and fine-grained gh PATs), but can't exfiltrate the key material itself.

### Host discovery + auto-generated override

The static `docker-compose.yml` in each project is host-agnostic — it defines the tools container and the shared/per-project volumes only. All credential and agent mounts are host-specific and are written out to `docker-compose.override.yml` (alongside the base file) every time `dev up` runs. Compose auto-loads the override, so `docker compose` commands against the project dir pick them up without extra flags.

`dev up` does two things before calling compose:

1. **`ensureHostPaths`** — touches missing `~/.claude/*` and `~/.config/gh/` so those bind sources always exist on a fresh host. Only devtools-specific paths are auto-created; credential paths (`.gnupg`, `.gitconfig`) are left alone and simply not mounted when absent.
2. **`internal/hostenv.Discover`** — probes the host for each credential source in turn (ssh-agent, gpg-agent via `gpgconf`, gpg keybox/pubring, gitconfig, gh dir, claude personalization) and returns only the paths that exist right now. The override renderer (`internal/compose.WriteOverride`) emits a bind-mount for every non-empty field.

That means `dev up` works on hosts without an ssh-agent, without gpg installed, on gpg 2.1+ (keybox dir) and legacy (pubring.kbx) alike, on macOS (Homebrew gpg socket paths) and Linux (systemd per-user runtime dir) — each host gets the subset that applies, and nothing fails when a piece is missing. The override file is `.gitignored` and can be deleted at any time; the next `dev up` rewrites it.

---

## Customization

### Adding runtimes to a project

Drop a `.mise.toml` inside `/code` in the container (or commit it to the project's git repo):

```toml
[tools]
php  = "8.3"
node = "22"
go   = "1.25"
```

The entrypoint runs `mise install` in the background on every container start. Subsequent shells pick the runtimes up via `$PATH`.

### Adding services

`projects/<name>/docker-compose.yml` has stubbed stanzas for MariaDB, Redis, and Mailpit. Uncomment, customize, `dev up <name>` again.

### Pinning a tool version

```bash
dev bump myproject --tag 2026-04-23  # pin to a specific dated image
dev up   myproject                   # pick up the new tag
```

To roll forward when you're ready:

```bash
dev bump myproject                   # defaults to today's date
dev up   myproject
```

### Updating the `dev` CLI itself

```bash
dev update --check          # report if a newer release is available
dev update                  # download, verify SHA-256, atomic replace
dev update --tag v0.2.0     # pin to a specific release
dev update --image          # also `docker pull` the latest base image
```

### Multi-account Claude Code

Because each project has its own `home` volume and does its own `claude login`, you can authenticate with your work account in `work-project` and your personal account in `side-project` without juggling.

---

## Development

Requires Go 1.25+ and Docker (with buildx).

```bash
# Build the CLI locally
go build -o dev ./cmd/dev

# Run unit tests
go test ./...

# Build the base image locally
./dev build --tag local

# Run the post-build smoke test
./base/smoke-test.sh devtools:local
```

### Release flow

- **Image**: push to `main` touching `base/**` triggers a multi-arch build → `ghcr.io/necrogami/devtools:{latest,YYYY-MM-DD,sha-<short>}`.
- **CLI**: tag `v*.*.*` → GoReleaser builds linux/darwin × amd64/arm64 binaries into a GitHub Release.

---

## Status

First stable scaffold landed. Working on real dog-food migration of existing projects, CI verification, and incremental polish. See [`SPEC.md`](./SPEC.md) §14 for the implementation order.

## Contributing

Issues and pull requests are welcome. See [`CONTRIBUTING.md`](./CONTRIBUTING.md) for development setup, the test/release flow, and PR expectations. Community interactions are governed by the [`CODE_OF_CONDUCT.md`](./CODE_OF_CONDUCT.md).

## Security

Report vulnerabilities privately per [`SECURITY.md`](./SECURITY.md) — do not open public issues for security reports.

## License

[MIT](./LICENSE)
