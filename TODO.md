# Status + Backlog

Snapshot as of 2026-04-24. Picks up where the day ended.

---

## Current state (shipped)

- **CLI**: `v0.3.2` on GitHub Releases. `dev update` idempotent; `vv` double-prefix fixed; `dev up --pull` forces registry check + `--force-recreate`; `dev bump` points at `dev up --pull <name>` as the next step (whether the tag changed or not).
- **Base image**: `ghcr.io/necrogami/devtools:latest` (image ID `290116db…`) — slim Debian + Homebrew (Linuxbrew) + Claude Code + oh-my-posh/Atomic + gh/yq/rg/fd/bat/fzf/eza/tmux/vim. No language -dev bloat. Brew installs via `/code/Brewfile` at container start.
- **Shared volumes**:
  - `devtools_brew` — Homebrew cellar (shared across every project)
  - `devtools_composer|npm|pnpm|cargo|gomod|pip` — language caches
  - `devtools_claude_plugins|skills|commands` — cross-project Claude artifacts, seeded from host `~/.claude/*` on first `dev init-shared`
- **Credential flow**: `docker-compose.yml` is host-agnostic. `dev up` writes `docker-compose.override.yml` per-run from `internal/hostenv.Discover`:
  - SSH agent socket (via `$SSH_AUTH_SOCK`)
  - GPG agent socket (via `gpgconf --list-dirs agent-socket` with XDG + `~/.gnupg` fallbacks)
  - GPG keyboxd socket + `common.conf` for gpg 2.3+ (`use-keyboxd`)
  - GPG public material: `~/.gnupg/{public-keys.d,pubring.kbx,trustdb.gpg,ownertrust}` — whichever exist
  - `~/.gitconfig`, `~/.config/gh`, `~/.claude/{settings.json,CLAUDE.md,agents}`
- **Doctor** (`dev doctor`): uses the same discovery as `dev up`; new `checkKeyboxd` catches `use-keyboxd` + no-socket mismatch as FAIL; ends with a "forwarded into container" summary mirroring the override.
- **Projects in use**: `brain`, `shilla`. Both on `:latest`, both have brew + Atomic prompt + the shared cellar populated (PHP already installed from one project is visible to the other).

---

## Known-but-not-fixed (parked, cosmetic)

### `gpg: Note: trustdb not writable`

Appears on every `gpg --list-keys` / `gpg --verify` inside the container. `~/.gnupg/trustdb.gpg` is mounted :ro from the host, and gpg wants to write update timestamps to it.

Verification and signing still produce correct results (`Good signature from …`). Pure stderr noise.

**Fix direction**: on entrypoint, if `/run/host/gpg-trustdb` is mounted (source of truth), `cp` it to `~/.gnupg/trustdb.gpg` on first start into the writable home volume instead of bind-mounting `:ro`. Diverges container's trust from host's but container gpg stops complaining; host trust db stays authoritative.

Scope: ~20 LOC in entrypoint + override renderer switch from bind-mount to copy, plus a refresh path for when the host's trustdb changes (`dev refresh-trustdb <project>`?). Not urgent.

### brain home volume still has Debian-stock `.bashrc`

`brain_home` was first initialized from a pre-0.3.0 image where the Dockerfile's `useradd -m` ran before our custom skel was copied into `/etc/skel`. The volume locked in Debian's default dotfiles and never got our custom ones.

Functionally fine — `/etc/bash.bashrc` → `/etc/profile.d/devtools-shell.sh` gives the Atomic prompt regardless. Only the user's `~/.bashrc` is stale (history settings, shell options, etc. are Debian defaults instead of ours).

**Fix**: `dev down brain --volumes` then `dev up brain` would reseed from the current image's custom skel. User data in the volume is lost (shell history, per-project home-scoped installs). Deliberately not doing this automatically.

### `~/.bashrc` inside container still references mise paths indirectly

Not directly mise, but the base/skel/.bashrc mentions "mise hook" in a comment that's now stale. Caught in a grep, not in code paths. Worth a pass.

**Fix**: update the comment in `base/skel/.bashrc` to reference Homebrew shellenv + oh-my-posh, not mise. Trivial.

---

## Backlog (design done, not yet built)

### `dev claude-copy <project>` — reuse host Claude auth in a project

**Motivation**: `claude login` per-project means 50 logins for 50 projects. For projects using the main cred (personal work), user wants a one-shot copy from host. Work projects with separate orgs stay per-container.

**What to copy** (confirmed against the host tonight):

| Host path | Purpose | Copy destination in container |
|---|---|---|
| `~/.claude/.credentials.json` | OAuth tokens (the actual auth) | `~/.claude/.credentials.json` (0600) |
| `~/.claude.json` | top-level user state | `~/.claude.json` |
| `~/.claude/projects/` | per-project session history on host | NOT copied (scoped to the host's Claude sessions, not useful inside a container) |
| `~/.claude/settings.json` | already mounted :ro via override | no change |

**Design**:
- `dev claude-copy <project>` (explicit, per-project) — runs a one-shot container that mounts `<project>_home` + host's `~/.claude/.credentials.json` and `~/.claude.json` read-only, copies in, chowns to `dev`. Idempotent; warns if a live credential is about to be overwritten.
- `dev claude-copy --all` convenience — loops over every `projects/<name>/`.
- `dev claude-copy --clear <project>` — wipes the copied creds (for when you want to switch back to a per-project login, e.g. moving a project from personal to work).
- Credentials never written to disk in the devtools repo; never logged.
- Security note in doc: copying the creds means a compromised container has full use of the host's Claude session for its lifetime.

**Scope**: ~100 LOC in a new `cmd/dev/claude_copy.go` + SPEC entry + README section. Tests via `WithFakeHome` and `WithFakeDocker` following the existing init-shared pattern.

### Publish `:<date>` tag alongside `:latest`

CI's `build-image.yml` currently pushes `:latest` and `:sha-<abbr>`. Also pushing `:YYYY-MM-DD` would let `dev bump <name> --tag 2026-04-24` pin to today's dated image — the feature already documented in SPEC but not wired to the workflow.

**Fix**: one line in the `docker/metadata-action` tags list.

### `dev bundle` (tiny) — deprioritized

Earlier today I drafted a full bundle system (curated Brewfiles shipped with the image, per-project declarative enablement, CLI `dev bundle add/list/info/remove`). You correctly pushed back that this was overthinking it — per-project `/code/Brewfile` that the entrypoint runs is simpler and does the job. The bundle CLI surface is not needed. Parked indefinitely unless you find a real case for curated shared sets.

### `dev infer` — scan `/code` and draft a Brewfile

Like `dev bundle`, this was proposed before the brew switch. Would scan `/code` for ecosystem manifests (`package.json` → `brew "node"`, `go.mod` → `brew "go"`, `Gemfile` → `brew "ruby"`, etc.) and emit a starter `Brewfile`. Nice-to-have; not urgent since writing `Brewfile` by hand is trivial.

### Consider bottling the image itself

Current `:latest` is 1.92 GB. Most of that is the Debian base + Claude's bundled runtime + Homebrew's initial clone. Could probably shave 300-500 MB by:
- Dropping `vim-nox` → vim-tiny (saves ~30 MB but user loses `+clipboard` etc. — user call)
- Trimming apt caches more aggressively (already done)
- Homebrew tap: clone `--depth=1` is already set; could `rm -rf .git/` after clone (breaks `brew update` inside container, but that's fine for image baseline — the shared volume takes over at runtime)

Leave as-is unless pull time becomes annoying.

---

## Loose ends from the session

- **`build-image` workflow currently fires only on `base/**` pushes**. 0.3.1 and 0.3.2 didn't touch `base/**`, so the image wasn't rebuilt — correct behavior since nothing changed about the image. Worth keeping in mind when reading CI logs.
- **goreleaser config is fine** — injects `main.version` / `commit` / `date` via -ldflags. `dev version` now produces clean output for both release and source-tree builds.
- **`projects/brain/Brewfile.example`, `projects/shilla/Brewfile.example`** — already in place. Rename to `Brewfile` and commit with the project whenever you want persistent dep declaration.

---

## Pick-up order (when you resume)

1. `dev claude-copy` (biggest UX win, clear scope).
2. Date-tag in `build-image` (one-line change).
3. Stale-bashrc mise-comment cleanup (trivial, drop on next small PR).
4. Trustdb writeable copy (if the warning starts bothering you).
