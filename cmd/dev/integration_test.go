//go:build integration

// Package main integration tests.
//
// Gated behind the `integration` build tag + DEVTOOLS_INTEGRATION=1 env var.
// Run with:
//     DEVTOOLS_INTEGRATION=1 go test -tags integration ./cmd/dev
//
// These exercise the real docker daemon. Skipped by default so plain
// `go test ./...` in CI never requires docker.

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/necrogami/devtools/internal/testutil"
)

func requireIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("DEVTOOLS_INTEGRATION") != "1" {
		t.Skip("skipped: set DEVTOOLS_INTEGRATION=1 to run docker-dependent tests")
	}
}

// TestIntegrationInitSharedCreatesRealVolumes verifies `dev init-shared` end
// to end against the real docker daemon. Cleans up volumes on exit.
func TestIntegrationInitSharedCreatesRealVolumes(t *testing.T) {
	requireIntegration(t)

	// Use a prefix so we don't collide with real devtools volumes.
	t.Setenv("DEVTOOLS_TEST_PREFIX", "devtools_it_")

	root := testutil.MakeFakeRepo(t, nil)
	t.Chdir(root)
	repoFlag = ""

	out, err := testutil.RunCobraCmd(t, newInitSharedCmd(), "--no-seed")
	if err != nil {
		t.Fatalf("init-shared: %v\noutput: %s", err, out)
	}
	if !strings.Contains(out, "10 created") && !strings.Contains(out, "already existed") {
		t.Errorf("unexpected init-shared output:\n%s", out)
	}

	// Cleanup: best-effort remove.
	t.Cleanup(func() {
		for _, v := range sharedVolumes() {
			_ = runDocker("volume", "rm", "-f", v)
		}
	})
}

// TestIntegrationFullProjectCycle is a placeholder for the full
// `dev new → up → exec → down` loop. Requires the devtools image to be
// already loaded (`dev build --tag local` beforehand) plus real SSH/GPG
// agents. Skipped unless explicitly invoked.
func TestIntegrationFullProjectCycle(t *testing.T) {
	requireIntegration(t)
	if os.Getenv("DEVTOOLS_IT_FULL_CYCLE") != "1" {
		t.Skip("skipped: full-cycle test requires DEVTOOLS_IT_FULL_CYCLE=1 and a pre-built image")
	}

	// TODO: full cycle — not yet wired because it needs a test-specific
	// compose template that doesn't require agent sockets / ~/.claude.
	t.Skip("full cycle not yet implemented")
}
