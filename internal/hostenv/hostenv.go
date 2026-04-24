// Package hostenv discovers the credentials, agent sockets, and config
// files available on the host that can be forwarded into a devtools project
// container. Discovery is pure: nothing is started or mutated, and missing
// items simply produce empty fields on the returned HostCreds. The compose
// override renderer then emits bind-mounts only for non-empty fields, which
// gives graceful degradation across platforms (Linux with/without systemd,
// macOS, BSD, WSL, CI runners) and agent implementations (OpenSSH,
// gnome-keyring, 1Password, KeePassXC, GnuPG 2.1+ keybox vs. legacy).
package hostenv

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// HostCreds holds the host paths that the container will bind-mount.
// Every field is an absolute host path, or "" if the source is unavailable
// on this host. Consumers must treat empty fields as "don't mount".
type HostCreds struct {
	// Agent sockets.
	SSHAgentSock string // → /run/host/ssh-agent
	GPGAgentSock string // → /run/host/gpg-agent
	KeyboxdSock  string // → /run/host/keyboxd  (gpg 2.3+ with use-keyboxd)

	// GPG public material (read-only from the host).
	GPGKeyboxDir  string // ~/.gnupg/public-keys.d (gpg 2.1+)
	GPGPubringKbx string // ~/.gnupg/pubring.kbx (legacy / keybox compat stub)
	GPGTrustdb    string // ~/.gnupg/trustdb.gpg
	GPGOwnerTrust string // ~/.gnupg/ownertrust (optional)
	GPGCommonConf string // ~/.gnupg/common.conf (enables `use-keyboxd`, etc.)

	// Git + GitHub CLI.
	GitConfig string // ~/.gitconfig
	GHConfig  string // ~/.config/gh (dir)

	// Claude personalization (usually created by preflight, listed here for
	// completeness so the override is self-describing).
	ClaudeSettings string // ~/.claude/settings.json
	ClaudeMd       string // ~/.claude/CLAUDE.md
	ClaudeAgents   string // ~/.claude/agents (dir)
}

// Discover inspects the host under the given home directory and returns a
// HostCreds populated with the subset of sources that exist right now. Never
// returns an error; unavailable bits become empty fields.
//
// The caller is expected to pass os.UserHomeDir() in production. Tests pass
// a temp dir.
func Discover(home string) HostCreds {
	c := HostCreds{
		SSHAgentSock: discoverSSHAgent(),
		GPGAgentSock: discoverGPGAgent(home),
		KeyboxdSock:  discoverKeyboxd(home),
	}

	// GPG public material — we mount every piece that exists. On gpg 2.1+
	// keys live in public-keys.d (the keybox dir); on legacy / compat
	// setups pubring.kbx holds them. Mounting both when both exist costs
	// nothing and covers mixed setups.
	if p := existingDir(filepath.Join(home, ".gnupg", "public-keys.d")); p != "" {
		c.GPGKeyboxDir = p
	}
	if p := existingFile(filepath.Join(home, ".gnupg", "pubring.kbx")); p != "" {
		c.GPGPubringKbx = p
	}
	if p := existingFile(filepath.Join(home, ".gnupg", "trustdb.gpg")); p != "" {
		c.GPGTrustdb = p
	}
	if p := existingFile(filepath.Join(home, ".gnupg", "ownertrust")); p != "" {
		c.GPGOwnerTrust = p
	}
	if p := existingFile(filepath.Join(home, ".gnupg", "common.conf")); p != "" {
		c.GPGCommonConf = p
	}

	// Git + gh.
	if p := existingFile(filepath.Join(home, ".gitconfig")); p != "" {
		c.GitConfig = p
	}
	if p := existingDir(filepath.Join(home, ".config", "gh")); p != "" {
		c.GHConfig = p
	}

	// Claude.
	if p := existingFile(filepath.Join(home, ".claude", "settings.json")); p != "" {
		c.ClaudeSettings = p
	}
	if p := existingFile(filepath.Join(home, ".claude", "CLAUDE.md")); p != "" {
		c.ClaudeMd = p
	}
	if p := existingDir(filepath.Join(home, ".claude", "agents")); p != "" {
		c.ClaudeAgents = p
	}

	return c
}

// discoverSSHAgent returns a stat-able ssh-agent socket path, or "".
//
// The only source of truth is $SSH_AUTH_SOCK: it's what the user's shell
// environment already points at, and it covers OpenSSH, gnome-keyring,
// 1Password ($HOME/.1password/agent.sock), KeePassXC, Windows SSH Agent on
// WSL, macOS launchd, etc. We deliberately do NOT probe common fallback
// paths — silently picking up the "wrong" agent is worse than exposing a
// clear empty state that the user can fix.
func discoverSSHAgent() string {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return ""
	}
	if _, err := os.Stat(sock); err != nil {
		return ""
	}
	return sock
}

// discoverGPGAgent returns a stat-able gpg-agent socket path, or "".
//
// Preferred source: `gpgconf --list-dirs agent-socket`. This is the only
// cross-platform way to find the right socket (Linux systemd vs. macOS
// Homebrew vs. BSD /tmp vs. custom --homedir setups). If gpgconf is
// unavailable, fall back to the two most common locations: the systemd
// per-user runtime dir and ~/.gnupg itself.
func discoverGPGAgent(home string) string {
	if sock := gpgconfAgentSocket(); sock != "" {
		if _, err := os.Stat(sock); err == nil {
			return sock
		}
	}
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		cand := filepath.Join(xdg, "gnupg", "S.gpg-agent")
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	cand := filepath.Join(home, ".gnupg", "S.gpg-agent")
	if _, err := os.Stat(cand); err == nil {
		return cand
	}
	return ""
}

// gpgconfAgentSocket returns whatever gpgconf reports as the agent socket,
// without statting it. Callers stat the result. Empty string if gpgconf
// isn't on PATH or exits nonzero.
func gpgconfAgentSocket() string {
	return gpgconfDir("agent-socket")
}

// gpgconfKeyboxdSocket returns whatever gpgconf reports as the keyboxd
// socket (the key-storage daemon introduced in gpg 2.3, used when
// `common.conf` contains `use-keyboxd`). Empty string if gpgconf is
// unavailable or reports no socket.
func gpgconfKeyboxdSocket() string {
	return gpgconfDir("keyboxd-socket")
}

// gpgconfDir shells out to `gpgconf --list-dirs <key>` and returns the
// trimmed output, or "" if gpgconf isn't on PATH / exits nonzero.
func gpgconfDir(key string) string {
	bin, err := exec.LookPath("gpgconf")
	if err != nil {
		return ""
	}
	out, err := exec.Command(bin, "--list-dirs", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// discoverKeyboxd returns a stat-able keyboxd socket path, or "".
//
// Same probe order as discoverGPGAgent: authoritative gpgconf output
// first, then the standard XDG runtime dir, then ~/.gnupg for BSDs and
// non-systemd setups.
func discoverKeyboxd(home string) string {
	if sock := gpgconfKeyboxdSocket(); sock != "" {
		if _, err := os.Stat(sock); err == nil {
			return sock
		}
	}
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		cand := filepath.Join(xdg, "gnupg", "S.keyboxd")
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	cand := filepath.Join(home, ".gnupg", "S.keyboxd")
	if _, err := os.Stat(cand); err == nil {
		return cand
	}
	return ""
}

// ----- small helpers ---------------------------------------------------------

func existingFile(p string) string {
	info, err := os.Stat(p)
	if err != nil || info.IsDir() {
		return ""
	}
	return p
}

func existingDir(p string) string {
	info, err := os.Stat(p)
	if err != nil || !info.IsDir() {
		return ""
	}
	return p
}
