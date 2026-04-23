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
		{"# comment", "", "", false},
		{"", "", "", false},
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

	// Save without modification should preserve comments.
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

	e.Set("B", "two")  // update existing
	e.Set("C", "three") // append new

	if err := e.Save(); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(path)
	got := string(content)
	if !strings.Contains(got, "A=1") ||
		!strings.Contains(got, "B=two") ||
		!strings.Contains(got, "C=three") {
		t.Fatalf("unexpected save output:\n%s", got)
	}
	// Check order: A first, B in its original slot (2nd line), C appended.
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if lines[0] != "A=1" || lines[1] != "B=two" || lines[2] != "C=three" {
		t.Fatalf("lines out of order: %v", lines)
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

func TestEnsureRequired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	if err := os.WriteFile(path, []byte("DEVTOOLS_TAG=latest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureRequired(dir); err == nil {
		t.Fatal("expected missing PROJECT to error")
	}

	if err := os.WriteFile(path, []byte("PROJECT=demo\nDEVTOOLS_TAG=latest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureRequired(dir); err != nil {
		t.Fatalf("EnsureRequired on valid .env: %v", err)
	}
}
