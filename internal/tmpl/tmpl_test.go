package tmpl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderSubstitutesVars(t *testing.T) {
	srcDir := t.TempDir()
	writeSrc(t, srcDir, "README.md", "# {{.Project}}\n\ntag: {{.DevtoolsTag}}")
	writeSrc(t, srcDir, ".env.example", "PROJECT={{.Project}}\nDEVTOOLS_TAG={{.DevtoolsTag}}\n")

	dstDir := filepath.Join(t.TempDir(), "dst")
	err := Render(srcDir, dstDir, Vars{Project: "demo", DevtoolsTag: "2026-04-23"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	readme := readFile(t, filepath.Join(dstDir, "README.md"))
	if !strings.Contains(readme, "# demo") || !strings.Contains(readme, "tag: 2026-04-23") {
		t.Fatalf("README unexpected:\n%s", readme)
	}

	env := readFile(t, filepath.Join(dstDir, ".env"))
	if !strings.Contains(env, "PROJECT=demo") || !strings.Contains(env, "DEVTOOLS_TAG=2026-04-23") {
		t.Fatalf(".env unexpected:\n%s", env)
	}
}

func TestRenderStripsExampleSuffix(t *testing.T) {
	srcDir := t.TempDir()
	writeSrc(t, srcDir, "Brewfile.example", "# brew deps")

	dstDir := filepath.Join(t.TempDir(), "dst")
	if err := Render(srcDir, dstDir, Vars{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, "Brewfile")); err != nil {
		t.Fatal("expected Brewfile (suffix stripped), but it's missing")
	}
	if _, err := os.Stat(filepath.Join(dstDir, "Brewfile.example")); err == nil {
		t.Fatal("unstripped Brewfile.example should not exist")
	}
}

func TestRenderRefusesOverwrite(t *testing.T) {
	srcDir := t.TempDir()
	writeSrc(t, srcDir, "README.md", "# x")

	dstDir := filepath.Join(t.TempDir(), "dst")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatal(err)
	}

	err := Render(srcDir, dstDir, Vars{})
	if err == nil {
		t.Fatal("Render should refuse existing dstDir")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error message should mention overwrite: %v", err)
	}
}

func TestRenderFailsOnMissingVar(t *testing.T) {
	srcDir := t.TempDir()
	writeSrc(t, srcDir, "README.md", "{{.Nonexistent}}")

	dstDir := filepath.Join(t.TempDir(), "dst")
	err := Render(srcDir, dstDir, Vars{Project: "x"})
	if err == nil {
		t.Fatal("Render should error on unknown template variable")
	}
}

func TestRenderFailsOnMalformedTemplate(t *testing.T) {
	srcDir := t.TempDir()
	writeSrc(t, srcDir, "README.md", "{{.Project")

	dstDir := filepath.Join(t.TempDir(), "dst")
	err := Render(srcDir, dstDir, Vars{Project: "x"})
	if err == nil {
		t.Fatal("Render should error on malformed template syntax")
	}
}

func TestRenderPreservesDirStructure(t *testing.T) {
	srcDir := t.TempDir()
	writeSrc(t, srcDir, filepath.Join(".config", "gh", "config.yml"), "hosts: {}\n")

	dstDir := filepath.Join(t.TempDir(), "dst")
	if err := Render(srcDir, dstDir, Vars{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, ".config", "gh", "config.yml")); err != nil {
		t.Fatalf("expected nested file preserved: %v", err)
	}
}

func TestRenderPreservesFilePermissions(t *testing.T) {
	srcDir := t.TempDir()
	exec := filepath.Join(srcDir, "setup.sh")
	if err := os.WriteFile(exec, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	nonExec := filepath.Join(srcDir, "notes.md")
	if err := os.WriteFile(nonExec, []byte("# notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dstDir := filepath.Join(t.TempDir(), "dst")
	if err := Render(srcDir, dstDir, Vars{}); err != nil {
		t.Fatal(err)
	}

	info, _ := os.Stat(filepath.Join(dstDir, "setup.sh"))
	if info.Mode().Perm() != 0o755 {
		t.Errorf("exec bit not preserved: %o", info.Mode().Perm())
	}
	info, _ = os.Stat(filepath.Join(dstDir, "notes.md"))
	if info.Mode().Perm() != 0o644 {
		t.Errorf("non-exec perms not preserved: %o", info.Mode().Perm())
	}
}

func TestStripExampleSuffix(t *testing.T) {
	cases := map[string]string{
		".env.example":      ".env",
		"Brewfile.example":  "Brewfile",
		"README.md":         "README.md", // unchanged
		"plain":             "plain",     // unchanged
		"foo.bar.example":   "foo.bar",
		"ends-with.example": "ends-with",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			if got := stripExampleSuffix(in); got != want {
				t.Errorf("stripExampleSuffix(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

func TestRenderEmptySrcDir(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := filepath.Join(t.TempDir(), "dst")
	if err := Render(srcDir, dstDir, Vars{}); err != nil {
		t.Fatalf("Render empty: %v", err)
	}
	// Destination should exist and be empty.
	entries, err := os.ReadDir(dstDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty dst, got %v", entries)
	}
}

func writeSrc(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
