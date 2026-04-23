package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureHostPathsCreatesMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var buf bytes.Buffer
	created, err := ensureHostPaths(&buf)
	if err != nil {
		t.Fatalf("ensureHostPaths: %v", err)
	}
	if created == 0 {
		t.Fatal("expected at least one creation on empty home")
	}

	for _, req := range hostPathRequirements {
		abs := filepath.Join(home, req.rel)
		info, err := os.Stat(abs)
		if err != nil {
			t.Errorf("missing %s after pre-flight: %v", abs, err)
			continue
		}
		if req.kind == "dir" && !info.IsDir() {
			t.Errorf("%s should be a dir", abs)
		}
		if req.kind == "file" && info.IsDir() {
			t.Errorf("%s should be a file", abs)
		}
	}
}

func TestEnsureHostPathsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var buf bytes.Buffer
	if _, err := ensureHostPaths(&buf); err != nil {
		t.Fatal(err)
	}
	// Snapshot the timestamps after first run.
	before := snapshotTimes(t, home)

	buf.Reset()
	created, err := ensureHostPaths(&buf)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}
	if created != 0 {
		t.Fatalf("second call created %d new paths (want 0)", created)
	}
	if buf.Len() != 0 {
		t.Errorf("second call had output: %q", buf.String())
	}

	// Confirm nothing got touched.
	after := snapshotTimes(t, home)
	for k, v := range before {
		if after[k] != v {
			t.Errorf("%s mtime changed across idempotent calls", k)
		}
	}
}

func TestEnsureHostPathsPreservesExistingContent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// User already has content.
	claudeMd := filepath.Join(home, ".claude", "CLAUDE.md")
	if err := os.MkdirAll(filepath.Dir(claudeMd), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(claudeMd, []byte("# my memory"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := ensureHostPaths(&buf); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(claudeMd)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "# my memory" {
		t.Fatalf("pre-flight overwrote existing CLAUDE.md: %q", string(content))
	}
}

func TestEnsureHostPathsDirModes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var buf bytes.Buffer
	if _, err := ensureHostPaths(&buf); err != nil {
		t.Fatal(err)
	}
	// Sanity: .claude/ is 0755, not wide open, not overly locked.
	info, err := os.Stat(filepath.Join(home, ".claude"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 == 0 && info.Mode().Perm()&0o400 == 0 {
		t.Errorf(".claude mode looks wrong: %o", info.Mode().Perm())
	}
}

// snapshotTimes returns a map of relative path → mtime-epoch-nanoseconds
// for everything under root, for idempotence checks.
func snapshotTimes(t *testing.T, root string) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		out[rel] = info.ModTime().UnixNano()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
