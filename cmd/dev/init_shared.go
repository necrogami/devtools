package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// sharedCacheVolumes are package-manager download caches shared across
// projects to avoid re-downloading runtimes/deps per project.
var sharedCacheVolumes = []string{
	"devtools_mise",
	"devtools_composer",
	"devtools_npm",
	"devtools_pnpm",
	"devtools_cargo",
	"devtools_gomod",
	"devtools_pip",
}

// claudeSharedVolume is a cross-project Claude Code artifact volume that
// gets first-run seeded from a host directory. Write access is intentional
// so containers can install plugins/skills/commands.
type claudeSharedVolume struct {
	name      string // docker volume name
	hostSub   string // path under $HOME to seed from (empty = no seed)
	mountPath string // path inside the tools container (for reference)
}

var claudeSharedVolumes = []claudeSharedVolume{
	{"devtools_claude_plugins", ".claude/plugins", "/home/dev/.claude/plugins"},
	{"devtools_claude_skills", ".claude/skills", "/home/dev/.claude/skills"},
	{"devtools_claude_commands", ".claude/commands", "/home/dev/.claude/commands"},
}

// sharedVolumes returns every volume `dev init-shared` manages, for use by
// doctor and other subcommands that just need the names.
func sharedVolumes() []string {
	all := make([]string, 0, len(sharedCacheVolumes)+len(claudeSharedVolumes))
	all = append(all, sharedCacheVolumes...)
	for _, c := range claudeSharedVolumes {
		all = append(all, c.name)
	}
	return all
}

func newInitSharedCmd() *cobra.Command {
	var skipSeed bool
	cmd := &cobra.Command{
		Use:   "init-shared",
		Short: "Create shared cache & Claude volumes (idempotent; seeds from ~/.claude on first run)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			created, skipped := 0, 0

			// Cache volumes — no seeding, they're download caches.
			for _, v := range sharedCacheVolumes {
				c, err := ensureVolume(v)
				if err != nil {
					return err
				}
				logVolume(out, v, c, "")
				if c {
					created++
				} else {
					skipped++
				}
			}

			// Claude volumes — seed from host if empty and host dir exists.
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("cannot resolve $HOME: %w", err)
			}
			for _, cv := range claudeSharedVolumes {
				c, err := ensureVolume(cv.name)
				if err != nil {
					return err
				}
				note := ""
				if c && !skipSeed {
					hostPath := filepath.Join(home, cv.hostSub)
					if s, err := os.Stat(hostPath); err == nil && s.IsDir() {
						if err := seedVolume(cv.name, hostPath); err != nil {
							return fmt.Errorf("seed %s from %s: %w", cv.name, hostPath, err)
						}
						note = "seeded from " + hostPath
					} else {
						note = "host dir absent, skipped seed"
					}
				}
				logVolume(out, cv.name, c, note)
				if c {
					created++
				} else {
					skipped++
				}
			}

			fmt.Fprintf(out, "\n%d created, %d already existed.\n", created, skipped)
			return nil
		},
	}
	cmd.Flags().BoolVar(&skipSeed, "no-seed", false,
		"create Claude volumes empty instead of seeding from host ~/.claude")
	return cmd
}

// ensureVolume returns (created, error). created=true means this call made it.
func ensureVolume(name string) (bool, error) {
	ok, err := volumeExists(name)
	if err != nil {
		return false, err
	}
	if ok {
		return false, nil
	}
	if err := exec.Command("docker", "volume", "create", name).Run(); err != nil {
		return false, fmt.Errorf("create %s: %w", name, err)
	}
	return true, nil
}

// logVolume prints a single row for the init-shared report.
func logVolume(w io.Writer, name string, created bool, note string) {
	prefix := "="
	label := "(exists)"
	if created {
		prefix = "+"
		label = "(created)"
	}
	if note != "" {
		label = label + " — " + note
	}
	fmt.Fprintf(w, "  %s %-28s %s\n", prefix, name, label)
}

// seedVolume mounts the volume and the host path inside a throwaway container
// and copies host → volume. Runs as UID/GID 1000:1000 so files land with
// correct ownership for the `dev` user in the tools container.
func seedVolume(volumeName, hostPath string) error {
	script := `set -e
if [ -z "$(ls -A /dst 2>/dev/null)" ]; then
    cp -R /src/. /dst/ 2>/dev/null || true
fi`
	out, err := exec.Command("docker", "run", "--rm",
		"--user", "1000:1000",
		"-v", volumeName+":/dst",
		"-v", hostPath+":/src:ro",
		"busybox:stable",
		"sh", "-c", script,
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker run seed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func volumeExists(name string) (bool, error) {
	cmd := exec.Command("docker", "volume", "inspect", name)
	cmd.Stdout = nil
	cmd.Stderr = nil
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if _, ok := err.(*exec.ExitError); ok {
		return false, nil
	}
	if strings.Contains(err.Error(), "executable file not found") {
		return false, err
	}
	return false, nil
}
