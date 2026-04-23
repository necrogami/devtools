// Package testutil provides shared helpers for tests across devtools
// packages. Deliberately internal so it isn't part of the public API.
package testutil

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

// MakeFakeRepo creates a minimal devtools repo in a new temp dir containing
// just enough structure to satisfy paths.Resolve (SPEC.md + base/Dockerfile
// + an empty projects/ dir). Returns the absolute root path.
//
// If templateFiles is non-nil, each entry (relative path → content) is
// also written under template/.
func MakeFakeRepo(t *testing.T, templateFiles map[string]string) string {
	t.Helper()
	root := t.TempDir()

	write(t, filepath.Join(root, "SPEC.md"), "# fake")
	mkdir(t, filepath.Join(root, "base"))
	write(t, filepath.Join(root, "base", "Dockerfile"), "FROM scratch")
	mkdir(t, filepath.Join(root, "projects"))
	mkdir(t, filepath.Join(root, "template"))

	for rel, content := range templateFiles {
		write(t, filepath.Join(root, "template", rel), content)
	}

	return root
}

// WithFakeHome points $HOME at a fresh temp dir for the duration of t.
// Returns the fake home path.
func WithFakeHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

// WithFakeDocker installs a shell stub named `docker` at the front of $PATH.
// The stub's body is the provided `body` (bash), which has access to $1, $2
// etc. for the arguments passed. Tests can assert on stub-written state.
//
// Example:
//
//	log := WithFakeDocker(t, `
//	    echo "$@" >> "$DOCKER_LOG"
//	    case "$1 $2" in
//	        "volume inspect") exit 1 ;;
//	        *) exit 0 ;;
//	    esac`)
//
// Returns the path to a log file the stub appends invocations to.
func WithFakeDocker(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "docker.log")

	// Ensure log exists (body appends).
	if err := os.WriteFile(logPath, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	script := fmt.Sprintf(`#!/usr/bin/env bash
export DOCKER_LOG=%q
%s
`, logPath, body)

	stubPath := filepath.Join(dir, "docker")
	if err := os.WriteFile(stubPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("DOCKER_LOG", logPath)
	return logPath
}

// RunCobraCmd invokes cmd with args and captures combined stdout + stderr.
// Returns the captured output and any error returned from Execute.
func RunCobraCmd(t *testing.T, cmd *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	// Reset any flags set by a previous invocation.
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	err := cmd.Execute()
	return buf.String(), err
}

// MustReadFile reads path or t.Fatals. Returns content as string.
func MustReadFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// CaptureStream is a trivial capture helper when Cobra's SetOut isn't
// what you want (e.g., testing functions that take an io.Writer directly).
type CaptureStream struct{ bytes.Buffer }

// Write implements io.Writer.
func (c *CaptureStream) Write(p []byte) (int, error) { return c.Buffer.Write(p) }

// Read returns accumulated output and resets the buffer.
func (c *CaptureStream) Read() string {
	out := c.Buffer.String()
	c.Buffer.Reset()
	return out
}

var _ io.Writer = (*CaptureStream)(nil)

// ----- small helpers ---------------------------------------------------------

func mkdir(t *testing.T, p string) {
	t.Helper()
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", p, err)
	}
}

func write(t *testing.T, p, content string) {
	t.Helper()
	mkdir(t, filepath.Dir(p))
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
}
