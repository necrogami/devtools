package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/necrogami/devtools/internal/paths"
)

// resolvePaths resolves the repo using repoFlag or CWD.
func resolvePaths() (*paths.Resolver, error) {
	return paths.Resolve(repoFlag)
}

// resolveProject resolves the repo and validates+returns the project dir.
// Returns an error if the project doesn't exist on disk.
func resolveProject(name string) (*paths.Resolver, string, error) {
	r, err := resolvePaths()
	if err != nil {
		return nil, "", err
	}
	dir, err := r.ProjectDir(name)
	if err != nil {
		return nil, "", err
	}
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("project %q does not exist at %s (try `dev new %s`)",
				name, dir, name)
		}
		return nil, "", err
	}
	return r, dir, nil
}

// runCompose runs `docker compose` in the given project directory with the
// given extra args. Stdio is inherited so compose output streams live.
func runCompose(projectDir string, args ...string) error {
	full := append([]string{"compose"}, args...)
	cmd := exec.Command("docker", full...)
	cmd.Dir = projectDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runDocker runs `docker <args...>` with stdio inherited.
func runDocker(args ...string) error {
	return runDockerIn("", args...)
}

// runDockerIn runs `docker <args...>` with CWD=dir (if non-empty) and
// stdio inherited.
func runDockerIn(dir string, args ...string) error {
	cmd := exec.Command("docker", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
