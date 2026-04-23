package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
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

func checkGPGAgent() result {
	xdg := os.Getenv("XDG_RUNTIME_DIR")
	if xdg == "" {
		return result{"gpg-agent", warn,
			"XDG_RUNTIME_DIR not set",
			"ensure you're logged in via a systemd --user session"}
	}
	sock := filepath.Join(xdg, "gnupg", "S.gpg-agent")
	if _, err := os.Stat(sock); err != nil {
		return result{"gpg-agent", warn,
			"socket " + sock + " not found",
			"`gpgconf --launch gpg-agent`"}
	}
	return result{"gpg-agent", pass, sock, ""}
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
