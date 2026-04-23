package compose

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAssignment(t *testing.T) {
	cases := []struct {
		in       string
		k, v     string
		ok       bool
	}{
		{"KEY=value", "KEY", "value", true},
		{"  SPACED =  trimmed ", "SPACED", "  trimmed ", true},
		{`QUOTED="with spaces"`, "QUOTED", "with spaces", true},
		{`SINGLE='sq value'`, "SINGLE", "sq value", true},
		{"EMPTY=", "EMPTY", "", true},
		{"WITH_EQUALS_IN_VAL=a=b=c", "WITH_EQUALS_IN_VAL", "a=b=c", true},
		{"MIXED_QUOTES=\"ends'different\"", "MIXED_QUOTES", "ends'different", true},
		{"# comment", "", "", false},
		{"  # indented comment", "", "", false},
		{"", "", "", false},
		{"   ", "", "", false},
		{"=no_key", "", "", false},
		{"novaluejustword", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			k, v, ok := parseAssignment(c.in)
			if ok != c.ok {
				t.Fatalf("ok=%v want %v", ok, c.ok)
			}
			if ok && (k != c.k || v != c.v) {
				t.Fatalf("got (%q,%q) want (%q,%q)", k, v, c.k, c.v)
			}
		})
	}
}

func TestLoadEnvAndSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	original := `# leading comment
PROJECT=demo
DEVTOOLS_TAG=latest

# trailing comment
EMPTY=
QUOTED="hello world"
`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	e, err := LoadEnv(path)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []struct{ k, v string }{
		{"PROJECT", "demo"},
		{"DEVTOOLS_TAG", "latest"},
		{"EMPTY", ""},
		{"QUOTED", "hello world"},
	} {
		got, ok := e.Get(want.k)
		if !ok || got != want.v {
			t.Fatalf("Get(%q) = (%q,%v) want (%q,true)", want.k, got, ok, want.v)
		}
	}

	if err := e.Save(); err != nil {
		t.Fatal(err)
	}
	roundtrip, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"# leading comment", "# trailing comment", "PROJECT=demo"} {
		if !strings.Contains(string(roundtrip), marker) {
			t.Fatalf("roundtrip missing %q:\n%s", marker, string(roundtrip))
		}
	}
}

func TestLoadEnvMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.env")

	e, err := LoadEnv(path)
	if err != nil {
		t.Fatalf("LoadEnv of missing file should succeed: %v", err)
	}
	if _, ok := e.Get("ANYTHING"); ok {
		t.Fatal("empty file shouldn't have any keys")
	}

	// Setting + saving should create the file.
	e.Set("NEW", "val")
	if err := e.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("Save didn't create the file")
	}
}

func TestSetInPlaceAndAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("A=1\nB=2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := LoadEnv(path)
	if err != nil {
		t.Fatal(err)
	}

	e.Set("B", "two")   // update existing
	e.Set("C", "three") // append new

	if err := e.Save(); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(path)
	got := string(content)

	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	want := []string{"A=1", "B=two", "C=three"}
	for i := range want {
		if i >= len(lines) || lines[i] != want[i] {
			t.Fatalf("line[%d] = %q, want %q (full: %v)", i, safeGet(lines, i), want[i], lines)
		}
	}
}

func TestSetPreservesCommentsAndBlanks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	original := "# header\n\nA=1\n# inline for B\nB=2\n\n# footer\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := LoadEnv(path)
	if err != nil {
		t.Fatal(err)
	}
	e.Set("A", "one")
	if err := e.Save(); err != nil {
		t.Fatal(err)
	}
	roundtrip, _ := os.ReadFile(path)
	got := string(roundtrip)
	for _, want := range []string{"# header", "# inline for B", "# footer", "A=one", "B=2"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestBumpTagCreatesFile(t *testing.T) {
	dir := t.TempDir()
	prev, err := BumpTag(dir, "2026-04-23")
	if err != nil {
		t.Fatal(err)
	}
	if prev != "" {
		t.Fatalf("prev = %q, want empty", prev)
	}
	content, _ := os.ReadFile(filepath.Join(dir, ".env"))
	if !strings.Contains(string(content), "DEVTOOLS_TAG=2026-04-23") {
		t.Fatalf("env missing DEVTOOLS_TAG:\n%s", string(content))
	}
}

func TestBumpTagReturnsPrevious(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	_ = os.WriteFile(path, []byte("DEVTOOLS_TAG=2026-04-01\nPROJECT=x\n"), 0o644)

	prev, err := BumpTag(dir, "2026-04-23")
	if err != nil {
		t.Fatal(err)
	}
	if prev != "2026-04-01" {
		t.Fatalf("prev = %q, want 2026-04-01", prev)
	}
}

func TestBumpTagNoopWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	_ = os.WriteFile(path, []byte("DEVTOOLS_TAG=same\n"), 0o644)

	prev, err := BumpTag(dir, "same")
	if err != nil {
		t.Fatal(err)
	}
	if prev != "same" {
		t.Fatalf("prev = %q, want same", prev)
	}
}

func TestEnsureRequired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	if err := os.WriteFile(path, []byte("DEVTOOLS_TAG=latest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := EnsureRequired(dir)
	if err == nil {
		t.Fatal("expected missing PROJECT to error")
	}
	if !strings.Contains(err.Error(), "PROJECT") {
		t.Errorf("error should mention PROJECT: %v", err)
	}

	if err := os.WriteFile(path, []byte("PROJECT=demo\nDEVTOOLS_TAG=latest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureRequired(dir); err != nil {
		t.Fatalf("EnsureRequired on valid .env: %v", err)
	}

	// Empty value also counts as missing.
	if err := os.WriteFile(path, []byte("PROJECT=\nDEVTOOLS_TAG=latest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureRequired(dir); err == nil {
		t.Fatal("empty PROJECT value should fail EnsureRequired")
	}
}

func TestEnvFileAndComposeFilePaths(t *testing.T) {
	dir := "/tmp/foo"
	if got := EnvFile(dir); got != "/tmp/foo/.env" {
		t.Errorf("EnvFile = %q", got)
	}
	if got := ComposeFile(dir); got != "/tmp/foo/docker-compose.yml" {
		t.Errorf("ComposeFile = %q", got)
	}
}

func safeGet(ss []string, i int) string {
	if i < 0 || i >= len(ss) {
		return "<out-of-range>"
	}
	return ss[i]
}
