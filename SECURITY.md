# Security Policy

## Reporting a vulnerability

Please report security vulnerabilities **privately** to **a@c4.io**. Do **not** open a public GitHub issue, pull request, or discussion for security reports — coordinated disclosure protects users who haven't patched yet.

### What to include

- A description of the issue and the impact you see
- Steps to reproduce, or a proof-of-concept
- The version of the `dev` CLI (`dev version` output) and/or the image tag you tested against
- Your disclosure preference — public credit, anonymous, or embargo

### What you can expect

- Acknowledgement within **3 business days**
- An initial severity assessment within **7 days**
- A patch release and coordinated disclosure timeline once triaged

If a fix requires advance notice to deployers, we'll coordinate an embargo with you before public disclosure.

## Supported versions

While the project is pre-1.0, **only the latest release tag receives security patches**. Older tags remain downloadable but will not get backports unless a vulnerability is being actively exploited in the wild.

After 1.0, this policy will formalize into minor-version support windows.

## Scope

In scope:

- The `dev` CLI source code (`cmd/dev/`, `internal/`)
- The base image Dockerfile and install scripts (`base/`)
- The project compose template (`template/`)
- Release-artifact integrity (CLI binaries on GitHub Releases, image on GHCR)
- Build pipeline (`.github/workflows/*.yml`, `install.sh`)

Out of scope:

- Vulnerabilities in third-party tools baked into the image (PHP, mise runtimes, Claude Code, etc.) — please file those with the respective upstream projects.
- Social engineering of project maintainers.
- Denial-of-service issues that require pre-existing privileged local access.

## Credit

Researchers who responsibly disclose will be credited in the release notes, unless they prefer to remain anonymous.
