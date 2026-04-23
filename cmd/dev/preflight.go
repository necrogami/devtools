package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// hostPathRequirement describes a host path that must exist before compose
// can bind-mount it. If kind == "file", we `touch` it empty when absent;
// if kind == "dir", we `mkdir -p` it. Sensitive dirs like .gnupg and
// .claude are created with 0700 / 0755 as appropriate.
type hostPathRequirement struct {
	rel  string // path relative to $HOME
	kind string // "file" | "dir"
	mode os.FileMode
}

// hostPathRequirements are the minimum host paths the project compose
// template needs to bind-mount successfully. We auto-create missing ones
// so `dev up` "just works" on a fresh host without manual prep.
var hostPathRequirements = []hostPathRequirement{
	{".claude", "dir", 0o755},
	{".claude/settings.json", "file", 0o644},
	{".claude/CLAUDE.md", "file", 0o644},
	{".claude/agents", "dir", 0o755},
	{".claude/skills", "dir", 0o755},
	{".claude/commands", "dir", 0o755},
	{".claude/plugins", "dir", 0o755},
	{".config", "dir", 0o755},
	{".config/gh", "dir", 0o755},
}

// ensureHostPaths creates the files/dirs the compose template expects on
// the host. Prints a one-line notice per path created; silent when nothing
// changes. Returns the number of items created.
func ensureHostPaths(w io.Writer) (int, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0, fmt.Errorf("resolve home: %w", err)
	}

	created := 0
	for _, req := range hostPathRequirements {
		abs := filepath.Join(home, req.rel)
		switch req.kind {
		case "dir":
			if _, err := os.Stat(abs); err == nil {
				continue
			}
			if err := os.MkdirAll(abs, req.mode); err != nil {
				return created, fmt.Errorf("mkdir %s: %w", abs, err)
			}
		case "file":
			if _, err := os.Stat(abs); err == nil {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
				return created, fmt.Errorf("mkdir parent of %s: %w", abs, err)
			}
			f, err := os.OpenFile(abs, os.O_CREATE|os.O_EXCL|os.O_WRONLY, req.mode)
			if err != nil {
				return created, fmt.Errorf("touch %s: %w", abs, err)
			}
			_ = f.Close()
		}
		fmt.Fprintf(w, "  + created %s\n", abs)
		created++
	}
	return created, nil
}
