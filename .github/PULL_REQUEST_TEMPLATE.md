<!--
Thanks for contributing! Filling this out makes review faster.
If your change is trivial (typo, whitespace), feel free to delete sections that don't apply.
-->

## Summary

<!-- One or two sentences: what does this change and why? -->

## Related issues

<!--
Link to issue(s) this addresses. Use "Closes #N" / "Fixes #N" / "Refs #N".
If there's no issue, explain why a direct PR is the right path.
-->

## Type of change

<!-- Check all that apply. -->

- [ ] `type: bug` — fixes broken behavior
- [ ] `type: feature` — adds a new capability
- [ ] `type: enhancement` — improves an existing capability
- [ ] `type: docs` — documentation only
- [ ] `type: security` — security-relevant change
- [ ] `type: test` — test coverage only
- [ ] `type: chore` — maintenance / tooling / CI

## Area

- [ ] `area: cli` (cmd/dev, internal/)
- [ ] `area: image` (base/)
- [ ] `area: compose` (template/)
- [ ] `area: ci` (.github/workflows/, .goreleaser.yml)
- [ ] `area: docs` (README, SPEC)

## Test plan

<!-- How did you verify this works? Paste commands + output. -->

- [ ] `go test -race ./...` passes locally
- [ ] `go vet ./...` is clean
- [ ] Manually verified the changed behavior
- [ ] Updated tests for the changed code path
- [ ] (image changes) Ran `./base/smoke-test.sh devtools:local` on a local build

## Checklist

- [ ] README / SPEC updated if the behavior or surface changed
- [ ] No new commented-out code, no unrelated refactors
- [ ] Commit messages follow the existing style (imperative mood, explain the _why_)
- [ ] No secrets, tokens, or personal paths in the diff

## Breaking changes?

<!--
If yes, describe the migration path. If no, delete this section.
Remember: compose template changes break anyone who did `dev new <proj>` previously.
-->
