package main

import (
	"strings"
	"testing"

	"github.com/necrogami/devtools/internal/testutil"
)

func TestListCmdEmpty(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	t.Chdir(root)
	repoFlag = ""

	// Fake docker so dockerStatus doesn't error out on a real daemon lookup
	// (though with zero projects we never call it).
	testutil.WithFakeDocker(t, `exit 1`)

	out, err := testutil.RunCobraCmd(t, newListCmd())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out, "no projects yet") {
		t.Errorf("expected 'no projects yet', got: %s", out)
	}
}

func TestListCmdShowsProjectsAndTags(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	makeProject(t, root, "alpha", "PROJECT=alpha\nDEVTOOLS_TAG=2026-04-23\n")
	makeProject(t, root, "bravo", "PROJECT=bravo\nDEVTOOLS_TAG=latest\n")
	t.Chdir(root)
	repoFlag = ""

	// Fake docker returns "running" for alpha-tools, not-found for bravo-tools.
	// The real invocation is `docker inspect --format ... <name>-tools`, so
	// we scan all args for the container name rather than hardcoding a position.
	testutil.WithFakeDocker(t, `
if [ "$1" = "inspect" ]; then
    for a in "$@"; do
        if [ "$a" = "alpha-tools" ]; then
            echo running
            exit 0
        fi
    done
fi
exit 1`)

	out, err := testutil.RunCobraCmd(t, newListCmd())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, want := range []string{"alpha", "bravo", "2026-04-23", "latest", "running", "stopped"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestListCmdHandlesEnvWithoutTag(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	makeProject(t, root, "nolabel", "PROJECT=nolabel\n") // no DEVTOOLS_TAG
	t.Chdir(root)
	repoFlag = ""
	testutil.WithFakeDocker(t, `exit 1`)

	out, err := testutil.RunCobraCmd(t, newListCmd())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "—") {
		t.Errorf("missing em-dash placeholder for unset tag:\n%s", out)
	}
}
