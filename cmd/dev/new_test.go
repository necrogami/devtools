package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/necrogami/devtools/internal/testutil"
)

// seedTemplate writes a minimal project template into root/template/.
func seedTemplate(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"docker-compose.yml":  "name: {{.Project}}\nservices:\n  tools: {}\n",
		".env.example":        "PROJECT={{.Project}}\nDEVTOOLS_TAG={{.DevtoolsTag}}\n",
		".mise.toml.example":  "# mise config for {{.Project}}\n",
		"README.md":           "# {{.Project}}\n",
	}
	for rel, content := range files {
		p := filepath.Join(root, "template", rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNewCmdScaffoldsProject(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	seedTemplate(t, root)
	t.Chdir(root)
	repoFlag = "" // ensure auto-discovery

	_, err := testutil.RunCobraCmd(t, newNewCmd(), "myproj", "--tag", "2026-04-23")
	if err != nil {
		t.Fatalf("dev new: %v", err)
	}

	projDir := filepath.Join(root, "projects", "myproj")
	for _, rel := range []string{"docker-compose.yml", ".env", ".mise.toml", "README.md"} {
		if _, err := os.Stat(filepath.Join(projDir, rel)); err != nil {
			t.Errorf("missing generated file %s: %v", rel, err)
		}
	}

	env := testutil.MustReadFile(t, filepath.Join(projDir, ".env"))
	if !strings.Contains(env, "PROJECT=myproj") {
		t.Errorf(".env missing PROJECT=myproj:\n%s", env)
	}
	if !strings.Contains(env, "DEVTOOLS_TAG=2026-04-23") {
		t.Errorf(".env missing DEVTOOLS_TAG=2026-04-23:\n%s", env)
	}

	readme := testutil.MustReadFile(t, filepath.Join(projDir, "README.md"))
	if !strings.Contains(readme, "# myproj") {
		t.Errorf("README not rendered:\n%s", readme)
	}

	// Example suffixes stripped.
	if _, err := os.Stat(filepath.Join(projDir, ".env.example")); err == nil {
		t.Error(".env.example should have been renamed")
	}
}

func TestNewCmdDefaultTagIsLatest(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	seedTemplate(t, root)
	t.Chdir(root)
	repoFlag = ""

	_, err := testutil.RunCobraCmd(t, newNewCmd(), "foo")
	if err != nil {
		t.Fatal(err)
	}
	env := testutil.MustReadFile(t, filepath.Join(root, "projects", "foo", ".env"))
	if !strings.Contains(env, "DEVTOOLS_TAG=latest") {
		t.Errorf("default tag should be latest:\n%s", env)
	}
}

func TestNewCmdRejectsInvalidName(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	seedTemplate(t, root)
	t.Chdir(root)
	repoFlag = ""

	for _, bad := range []string{"BAD", "0starts", "has_underscore", "has space", ""} {
		t.Run(bad, func(t *testing.T) {
			_, err := testutil.RunCobraCmd(t, newNewCmd(), bad)
			if err == nil {
				t.Errorf("expected failure for name %q", bad)
			}
		})
	}
}

func TestNewCmdRefusesExisting(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	seedTemplate(t, root)
	t.Chdir(root)
	repoFlag = ""

	if _, err := testutil.RunCobraCmd(t, newNewCmd(), "dup"); err != nil {
		t.Fatal(err)
	}
	_, err := testutil.RunCobraCmd(t, newNewCmd(), "dup")
	if err == nil {
		t.Fatal("second scaffold of same name should fail")
	}
}

func TestNewCmdRequiresExactlyOneArg(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	seedTemplate(t, root)
	t.Chdir(root)
	repoFlag = ""

	// Zero args.
	if _, err := testutil.RunCobraCmd(t, newNewCmd()); err == nil {
		t.Error("zero args should fail")
	}
	// Two args.
	if _, err := testutil.RunCobraCmd(t, newNewCmd(), "a", "b"); err == nil {
		t.Error("two args should fail")
	}
}
