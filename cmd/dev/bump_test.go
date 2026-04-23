package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/necrogami/devtools/internal/testutil"
)

func makeProject(t *testing.T, root, name, envContent string) string {
	t.Helper()
	dir := filepath.Join(root, "projects", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if envContent != "" {
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(envContent), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestBumpCmdUpdatesTag(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	projDir := makeProject(t, root, "myproj", "PROJECT=myproj\nDEVTOOLS_TAG=old\n")
	t.Chdir(root)
	repoFlag = ""

	out, err := testutil.RunCobraCmd(t, newBumpCmd(), "myproj", "--tag", "2026-04-23")
	if err != nil {
		t.Fatalf("bump: %v", err)
	}
	if !strings.Contains(out, "old → 2026-04-23") {
		t.Errorf("expected transition message, got: %s", out)
	}

	env := testutil.MustReadFile(t, filepath.Join(projDir, ".env"))
	if !strings.Contains(env, "DEVTOOLS_TAG=2026-04-23") {
		t.Fatalf("tag not updated:\n%s", env)
	}
}

func TestBumpCmdDefaultsToToday(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	projDir := makeProject(t, root, "p", "PROJECT=p\nDEVTOOLS_TAG=old\n")
	t.Chdir(root)
	repoFlag = ""

	if _, err := testutil.RunCobraCmd(t, newBumpCmd(), "p"); err != nil {
		t.Fatal(err)
	}
	env := testutil.MustReadFile(t, filepath.Join(projDir, ".env"))
	today := time.Now().UTC().Format("2006-01-02")
	want := "DEVTOOLS_TAG=" + today
	if !strings.Contains(env, want) {
		t.Fatalf("expected %s in .env:\n%s", want, env)
	}
}

func TestBumpCmdNoopWhenUnchanged(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	_ = makeProject(t, root, "p", "DEVTOOLS_TAG=same\n")
	t.Chdir(root)
	repoFlag = ""

	out, err := testutil.RunCobraCmd(t, newBumpCmd(), "p", "--tag", "same")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "already pinned") {
		t.Errorf("expected 'already pinned' message, got: %s", out)
	}
}

func TestBumpCmdRejectsMissingProject(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	t.Chdir(root)
	repoFlag = ""

	out, err := testutil.RunCobraCmd(t, newBumpCmd(), "nonexistent")
	if err == nil {
		t.Fatalf("expected error for missing project, got output: %s", out)
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error should mention missing project: %v", err)
	}
}

func TestDisplayTagHelper(t *testing.T) {
	if got := displayTag(""); got != "(unset)" {
		t.Errorf("displayTag(\"\") = %q, want (unset)", got)
	}
	if got := displayTag("v1"); got != "v1" {
		t.Errorf("displayTag(\"v1\") = %q, want v1", got)
	}
}
