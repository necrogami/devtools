# Contributing to devtools

Thanks for your interest in helping out. This is a personal project first, but pull requests and issues are welcome — especially thoughtful ones.

Before anything else: please read the [Code of Conduct](./CODE_OF_CONDUCT.md). Participation in this project requires that you agree to abide by it.

## Ways to help

| Kind of help | How |
|---|---|
| Report a bug | [File a bug report](https://github.com/necrogami/devtools/issues/new?template=bug_report.yml). Include `dev doctor` and `dev version` output — the templates walk you through it. |
| Propose a feature | [File a feature request](https://github.com/necrogami/devtools/issues/new?template=feature_request.yml). Lead with the problem, not the solution. |
| Fix typos / clarify docs | [File a docs issue](https://github.com/necrogami/devtools/issues/new?template=docs.yml) or open a PR directly for tiny changes. |
| Weigh in on Windows support | [Issue #1](https://github.com/necrogami/devtools/issues/1). React or comment with your use case. |
| Write code | Read the sections below, then open a PR. |

## Before opening a PR

1. **Check if an issue exists.** For anything non-trivial (> 10-line change or touches behavior), open an issue first so we can agree on the approach before you invest time.
2. **Read [`SPEC.md`](./SPEC.md).** It captures the design intent and explicit non-goals. PRs that contradict the spec need a spec update as part of the PR.
3. **Match the existing style.** Read a nearby file before you start — this project has conventions (split-stage install scripts, prefix-grouped labels, etc.).

## Development setup

Requirements:
- Go 1.25 or newer
- Docker with buildx

```bash
# Clone
git clone git@github.com:necrogami/devtools.git
cd devtools

# Build the CLI
go build -o dev ./cmd/dev
./dev --help

# Run the full test suite
go test -race ./...

# Build the base image locally (takes 5-10 min on first run)
./dev build --tag local

# Smoke-test a locally-built image
./base/smoke-test.sh devtools:local
```

## Running tests

```bash
go test ./...                    # all packages
go test -race ./...              # with race detector (CI runs this)
go test -cover ./...             # with coverage summary
go test -v ./cmd/dev -run TestX  # single test, verbose
```

Integration tests are gated behind a build tag:

```bash
DEVTOOLS_INTEGRATION=1 go test -tags integration ./cmd/dev
```

## Code style

- **Go**: `go fmt` + `go vet` must pass. CI enforces both.
- **Naming**: favor clarity over brevity. `ensureHostPaths` beats `ehp`.
- **Comments**: explain the *why*, not the *what*. Good: `// Mount home first so specific caches can override it.` Bad: `// Mount home.`
- **Commits**: imperative mood, explain the motivation in the body. Match the existing log style (`git log --oneline`).
- **Tests**: prefer table-driven tests for repetitive patterns. Use `t.TempDir()` and `t.Setenv` for isolation. Integration-level concerns go behind the `integration` build tag.

## Pull request process

1. Fork and branch. Name branches `fix/<short-topic>` or `feat/<short-topic>`.
2. Write tests for any behavior change.
3. Run the full verification:
   ```bash
   go vet ./...
   go test -race ./...
   ./dev doctor    # if your change affects the CLI
   ```
4. Update docs:
   - `README.md` if user-facing behavior changed
   - `SPEC.md` if architecture / model / key decisions changed
   - Inline comments for any non-obvious code
5. Open a PR. The PR template will guide you through summary, type/area labels, test plan, and breaking-change disclosure.
6. Expect code review. Squash-merge is the default.

## Adding a new `dev` command

1. Create `cmd/dev/<cmd>.go` exposing `new<Cmd>Cmd() *cobra.Command`.
2. Register it in `cmd/dev/root.go`.
3. Add tests in `cmd/dev/<cmd>_test.go`. Use `internal/testutil` helpers.
4. Document it in the `README.md` command table and `SPEC.md §9.1`.
5. If the command shells out to Docker, use the existing `runCompose`/`runDocker`/`runDockerIn` helpers so tests can intercept via the fake-docker-on-PATH pattern.

## Modifying the base image

1. Edit `base/Dockerfile` and/or `base/install/*.sh`.
2. Rebuild locally: `./dev build --tag local`.
3. Run `./base/smoke-test.sh devtools:local`.
4. Push the change — CI rebuilds multi-arch and pushes to GHCR.

Keep layer discipline in mind: put slow-churn things (apt, Homebrew prefix) first; fast-churn things (Claude Code) last.

## Modifying the project template

1. Edit `template/docker-compose.yml` or `template/.env.example`.
2. Render a fresh project and inspect: `./dev new test-render && cat projects/test-render/docker-compose.yml`.
3. Clean up: `rm -rf projects/test-render`.
4. Remember that changes to the template only affect *new* projects. Existing projects keep their generated copy until regenerated.

## Release process (maintainers)

```bash
# Verify main is green and ready.
gh run list -R necrogami/devtools --limit 5

# Tag with an annotated release message.
git tag -a vX.Y.Z -m "vX.Y.Z — short summary ..."
git push origin vX.Y.Z

# The `build-cli` workflow picks up the tag, runs GoReleaser, and publishes
# a GitHub Release with multi-arch binaries + checksums.txt.
```

Image releases are continuous — every push to `main` touching `base/**` rebuilds and retags `ghcr.io/necrogami/devtools:{latest,YYYY-MM-DD,sha-<short>}`.

## Questions

If something isn't covered here, open a [question issue](https://github.com/necrogami/devtools/issues/new?labels=question). Feedback on this document is also welcome.
