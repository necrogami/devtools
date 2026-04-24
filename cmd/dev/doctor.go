package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/necrogami/devtools/internal/hostenv"
)

// level is the severity of a doctor check outcome.
type level int

const (
	pass level = iota
	warn
	fail
)

func (l level) tag() string {
	switch l {
	case pass:
		return " OK "
	case warn:
		return "WARN"
	default:
		return "FAIL"
	}
}

type result struct {
	name string
	lvl  level
	msg  string
	fix  string
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Inspect the host for devtools prerequisites and common pitfalls",
		RunE: func(cmd *cobra.Command, _ []string) error {
			checks := []func() result{
				checkDockerDaemon,
				checkBuildx,
				checkSSHAgent,
				checkGPGAgent,
				checkKeyboxd,
				checkUID,
				checkSharedVolumes,
				checkGHConfig,
			}
			out := cmd.OutOrStdout()
			worst := pass
			for _, c := range checks {
				r := c()
				if r.lvl > worst {
					worst = r.lvl
				}
				fmt.Fprintf(out, "[%s] %-20s  %s\n", r.lvl.tag(), r.name, r.msg)
				if r.fix != "" && r.lvl != pass {
					fmt.Fprintf(out, "         fix: %s\n", r.fix)
				}
			}

			// Summarize what `dev up` would bind-mount on this host, so the
			// user sees the discovery result in one place rather than having
			// to read the generated override after an up.
			printForwardSummary(out)

			if worst == fail {
				return fmt.Errorf("one or more checks failed")
			}
			return nil
		},
	}
}

// ----- individual checks -----------------------------------------------------

func checkDockerDaemon() result {
	out, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output()
	if err != nil {
		return result{"docker daemon", fail,
			"not reachable",
			"start the docker service (e.g. `sudo systemctl start docker`)"}
	}
	return result{"docker daemon", pass,
		"server " + strings.TrimSpace(string(out)), ""}
}

func checkBuildx() result {
	if _, err := exec.Command("docker", "buildx", "version").Output(); err != nil {
		return result{"docker buildx", warn,
			"not installed",
			"install docker-buildx-plugin (apt) or Docker Desktop"}
	}
	return result{"docker buildx", pass, "available", ""}
}

func checkSSHAgent() result {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return result{"ssh-agent", warn,
			"SSH_AUTH_SOCK not set",
			"`eval $(ssh-agent)` and `ssh-add` your key"}
	}
	if _, err := os.Stat(sock); err != nil {
		return result{"ssh-agent", warn,
			"socket at $SSH_AUTH_SOCK is missing",
			"start a fresh ssh-agent and load keys"}
	}
	if out, err := exec.Command("ssh-add", "-l").CombinedOutput(); err == nil {
		return result{"ssh-agent", pass,
			strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0]), ""}
	}
	return result{"ssh-agent", warn,
		"socket present but agent has no keys",
		"`ssh-add ~/.ssh/id_ed25519` (or your preferred key)"}
}

// checkGPGAgent uses the same discovery chain as `dev up`
// (gpgconf --list-dirs → $XDG_RUNTIME_DIR/gnupg/ → ~/.gnupg/) so a PASS
// here means the same socket will be forwarded at container-up time. A
// missing socket is a WARN, not a FAIL: hosts without gpg still work,
// they just lose commit signing inside the container.
func checkGPGAgent() result {
	home, err := os.UserHomeDir()
	if err != nil {
		return result{"gpg-agent", warn, "cannot resolve $HOME", ""}
	}
	creds := hostenv.Discover(home)
	if creds.GPGAgentSock == "" {
		return result{"gpg-agent", warn,
			"no gpg-agent socket found (checked gpgconf, $XDG_RUNTIME_DIR/gnupg, ~/.gnupg)",
			"`gpgconf --launch gpg-agent`, or install gnupg if it isn't on this host"}
	}
	return result{"gpg-agent", pass, creds.GPGAgentSock, ""}
}

// checkKeyboxd verifies that if the host enables gpg's keyboxd daemon
// (use-keyboxd in common.conf, gpg 2.3+), the socket is actually
// reachable. Without this, in-container gpg with common.conf mounted
// will look for a socket the container can't find and report no keys.
// Missing use-keyboxd is not a problem — gpg just reads pubring.kbx
// directly — so we only flag the inconsistent case.
func checkKeyboxd() result {
	home, err := os.UserHomeDir()
	if err != nil {
		return result{"gpg keyboxd", warn, "cannot resolve $HOME", ""}
	}
	creds := hostenv.Discover(home)

	// Does common.conf say to use keyboxd?
	var useKeyboxd bool
	if creds.GPGCommonConf != "" {
		if data, err := os.ReadFile(creds.GPGCommonConf); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "#") || trimmed == "" {
					continue
				}
				if strings.HasPrefix(trimmed, "use-keyboxd") {
					useKeyboxd = true
					break
				}
			}
		}
	}

	switch {
	case useKeyboxd && creds.KeyboxdSock != "":
		return result{"gpg keyboxd", pass,
			"use-keyboxd + socket " + creds.KeyboxdSock, ""}
	case useKeyboxd && creds.KeyboxdSock == "":
		return result{"gpg keyboxd", fail,
			"common.conf enables use-keyboxd but no S.keyboxd socket is reachable",
			"`gpgconf --launch keyboxd` (or remove `use-keyboxd` from common.conf)"}
	case !useKeyboxd && creds.KeyboxdSock != "":
		return result{"gpg keyboxd", pass,
			"socket available (not enabled in common.conf)", ""}
	default:
		return result{"gpg keyboxd", pass, "not in use on this host", ""}
	}
}

// printForwardSummary writes a one-block summary of what `dev up` will
// bind-mount into the container, based on the same hostenv discovery
// the override renderer uses. Non-fatal — purely informational output.
// It answers the question "what credentials will I actually have inside
// the container?" without requiring the user to bring a stack up first.
func printForwardSummary(w io.Writer) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	c := hostenv.Discover(home)

	// (label, host path, missing-explanation) — missing-explanation is
	// only used when the host path is empty. Keep order aligned with
	// override.buildMounts so the two are easy to diff mentally.
	rows := []struct {
		label   string
		path    string
		missing string
	}{
		{"ssh-agent socket", c.SSHAgentSock, "no agent on host"},
		{"gpg-agent socket", c.GPGAgentSock, "gpg not installed / agent not running"},
		{"keyboxd socket", c.KeyboxdSock, "not used on this host"},
		{"gpg common.conf", c.GPGCommonConf, "no ~/.gnupg/common.conf"},
		{"gpg keybox dir", c.GPGKeyboxDir, "no ~/.gnupg/public-keys.d (legacy host?)"},
		{"gpg pubring.kbx", c.GPGPubringKbx, "no ~/.gnupg/pubring.kbx (gpg 2.1+ is fine)"},
		{"gpg trustdb", c.GPGTrustdb, "no ~/.gnupg/trustdb.gpg"},
		{"gitconfig", c.GitConfig, "no ~/.gitconfig"},
		{"gh config", c.GHConfig, "no ~/.config/gh"},
		{"claude settings", c.ClaudeSettings, "no ~/.claude/settings.json (preflight creates this)"},
		{"claude CLAUDE.md", c.ClaudeMd, "no ~/.claude/CLAUDE.md (preflight creates this)"},
		{"claude agents", c.ClaudeAgents, "no ~/.claude/agents (preflight creates this)"},
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "forwarded into container (`dev up` override):")
	for _, r := range rows {
		if r.path != "" {
			fmt.Fprintf(w, "  + %-18s %s\n", r.label, r.path)
		} else {
			fmt.Fprintf(w, "  - %-18s (%s)\n", r.label, r.missing)
		}
	}
}

func checkUID() result {
	u, err := user.Current()
	if err != nil {
		return result{"host uid", warn, "cannot read current user", ""}
	}
	if u.Uid == "1000" {
		return result{"host uid", pass, "uid=1000 matches image default", ""}
	}
	return result{"host uid", warn,
		fmt.Sprintf("uid=%s differs from image default 1000", u.Uid),
		"rebuild with `dev build` after setting build args UID/GID " +
			"(wire-up planned; manual `docker build --build-arg UID=$(id -u) ...` works today)"}
}

func checkSharedVolumes() result {
	all := sharedVolumes()
	var missing []string
	for _, v := range all {
		ok, _ := volumeExists(v)
		if !ok {
			missing = append(missing, v)
		}
	}
	if len(missing) == 0 {
		return result{"shared volumes", pass,
			fmt.Sprintf("all %d present", len(all)), ""}
	}
	return result{"shared volumes", warn,
		fmt.Sprintf("%d missing: %s", len(missing), strings.Join(missing, ", ")),
		"run `dev init-shared`"}
}

func checkGHConfig() result {
	home, err := os.UserHomeDir()
	if err != nil {
		return result{"gh config", warn, "cannot resolve $HOME", ""}
	}
	p := filepath.Join(home, ".config", "gh", "hosts.yml")
	if _, err := os.Stat(p); err != nil {
		return result{"gh config", warn,
			"~/.config/gh/hosts.yml not found",
			"`gh auth login` on the host"}
	}
	return result{"gh config", pass, p, ""}
}
