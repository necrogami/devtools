// Package paths resolves the canonical locations used by the `dev` CLI.
//
// Layout assumption: the CLI is invoked with CWD somewhere under the devtools
// repo (or with --repo set). We walk up from CWD to find the repo root,
// identified by the presence of SPEC.md + base/Dockerfile.
package paths

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// projectNamePattern defines the only project names we accept.
// Lowercase + digits + hyphen, must start with letter, 1-32 chars.
var projectNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// Resolver holds resolved absolute paths for a given repo root.
type Resolver struct {
	Root     string // repo root
	Base     string // <root>/base
	Template string // <root>/template
	Projects string // <root>/projects
	Shared   string // <root>/shared
}

// Resolve finds the repo root starting from startDir (or CWD if empty) and
// returns a Resolver pointing at canonical subdirs.
func Resolve(startDir string) (*Resolver, error) {
	if startDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getwd: %w", err)
		}
		startDir = cwd
	}

	root, err := findRoot(startDir)
	if err != nil {
		return nil, err
	}

	return &Resolver{
		Root:     root,
		Base:     filepath.Join(root, "base"),
		Template: filepath.Join(root, "template"),
		Projects: filepath.Join(root, "projects"),
		Shared:   filepath.Join(root, "shared"),
	}, nil
}

// ProjectDir returns the absolute path to a project's directory after
// validating the name. Does NOT require the directory to exist.
func (r *Resolver) ProjectDir(name string) (string, error) {
	if err := ValidateProjectName(name); err != nil {
		return "", err
	}
	return filepath.Join(r.Projects, name), nil
}

// ValidateProjectName enforces the project-name regex.
func ValidateProjectName(name string) error {
	if !projectNamePattern.MatchString(name) {
		return fmt.Errorf(
			"invalid project name %q: must match %s",
			name, projectNamePattern.String(),
		)
	}
	return nil
}

// ListProjects returns the names of existing project subdirectories.
// Skips dotfiles (e.g., .gitkeep).
func (r *Resolver) ListProjects() ([]string, error) {
	entries, err := os.ReadDir(r.Projects)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read projects dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if e.Name()[0] == '.' {
			continue
		}
		names = append(names, e.Name())
	}
	return names, nil
}

// findRoot walks up from start looking for the marker files that identify
// a devtools repo. Stops at filesystem root with a helpful error.
func findRoot(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	dir := abs
	for {
		if isRepoRoot(dir) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf(
				"not inside a devtools repo (searched upward from %s); "+
					"pass --repo or cd into the repo", abs,
			)
		}
		dir = parent
	}
}

// isRepoRoot returns true when both SPEC.md and base/Dockerfile exist.
// SPEC.md alone isn't enough — common file name; the Dockerfile seals it.
func isRepoRoot(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, "SPEC.md")); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, "base", "Dockerfile")); err != nil {
		return false
	}
	return true
}
