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
	writeSrc(t, srcDir, ".mise.toml.example", "# mise config")

	dstDir := filepath.Join(t.TempDir(), "dst")
	if err := Render(srcDir, dstDir, Vars{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, ".mise.toml")); err != nil {
		t.Fatal("expected .mise.toml (suffix stripped), but it's missing")
	}
	if _, err := os.Stat(filepath.Join(dstDir, ".mise.toml.example")); err == nil {
		t.Fatal("unstripped .mise.toml.example should not exist")
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

func TestRenderPreservesDirStructure(t *testing.T) {
	srcDir := t.TempDir()
	writeSrc(t, srcDir, filepath.Join(".config", "mise", "config.toml"), "[settings]\n")

	dstDir := filepath.Join(t.TempDir(), "dst")
	if err := Render(srcDir, dstDir, Vars{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dstDir, ".config", "mise", "config.toml")); err != nil {
		t.Fatalf("expected nested file preserved: %v", err)
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
