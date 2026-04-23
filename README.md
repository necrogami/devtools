# devtools

Personal developer-host system: one pre-baked toolbox image + per-project
isolated Compose stacks, orchestrated by a small Go CLI called `dev`.

See [`SPEC.md`](./SPEC.md) for the full design.

## What this gives you

- **One image** (`ghcr.io/necrogami/devtools`) with every tool you need:
  bash + oh-my-posh (Atomic), mise-managed runtimes on demand, Claude Code,
  git, gh, rg/fd/bat/fzf/eza, jq/yq, tmux, and DB clients.
- **Per-project containers** with their own `/code` and `/home` volumes,
  their own networks, and their own service sidecars (DB, redis, etc.).
- **Shared credential model**: SSH & GPG come in as agent sockets —
  private keys never enter any container.
- **Shared caches**: mise / composer / npm / pnpm / cargo / go-mod / pip
  live in cross-project named volumes so new projects spin up fast.

## Layout (high level)

```
base/        # Dockerfile + install scripts + skel dotfiles + entrypoint
template/    # scaffold for `dev new <project>`
projects/    # your actual project stacks (gitignored)
cmd/dev/     # the `dev` Go CLI
internal/    # CLI internals (compose mgmt, docker client, templating, doctor)
```

## Status

Spec locked, implementation in progress. See `SPEC.md` §14 for build order.
