package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateProjectName(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
	}{
		{"ezfleet", false},
		{"my-project", false},
		{"a", false},
		{"a0", false},
		{"project-with-dashes", false},
		{"", true},           // empty
		{"0starts-digit", true},
		{"has_underscore", true},
		{"HAS-UPPER", true},
		{"has.dot", true},
		{"has space", true},
		{"-leading-dash", true},
		{"too-long-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateProjectName(c.name)
			if (err != nil) != c.wantErr {
				t.Fatalf("ValidateProjectName(%q) err=%v wantErr=%v",
					c.name, err, c.wantErr)
			}
		})
	}
}

func TestResolveFindsRoot(t *testing.T) {
	root := makeFakeRepo(t)
	deep := filepath.Join(root, "cmd", "dev")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}

	r, err := Resolve(deep)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if r.Root != root {
		t.Fatalf("Root = %q, want %q", r.Root, root)
	}
	if r.Projects != filepath.Join(root, "projects") {
		t.Fatalf("Projects = %q", r.Projects)
	}
}

func TestResolveFailsOutsideRepo(t *testing.T) {
	dir := t.TempDir() // not a devtools repo
	if _, err := Resolve(dir); err == nil {
		t.Fatal("Resolve outside repo unexpectedly succeeded")
	}
}

func TestProjectDir(t *testing.T) {
	root := makeFakeRepo(t)
	r, err := Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	p, err := r.ProjectDir("valid-name")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "projects", "valid-name")
	if p != want {
		t.Fatalf("ProjectDir = %q, want %q", p, want)
	}
	if _, err := r.ProjectDir("BAD NAME"); err == nil {
		t.Fatal("ProjectDir accepted invalid name")
	}
}

func TestListProjects(t *testing.T) {
	root := makeFakeRepo(t)
	for _, n := range []string{"one", "two", ".gitkeep-dir-shouldbe-excluded"} {
		if err := os.MkdirAll(filepath.Join(root, "projects", n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	r, err := Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	// Order of os.ReadDir is lexical; dot-prefixed excluded.
	want := []string{"one", "two"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("ListProjects = %v, want %v", got, want)
	}
}

// makeFakeRepo creates a temp dir containing the two marker files that
// identify a devtools repo root.
func makeFakeRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "SPEC.md"), []byte("# fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "base"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "base", "Dockerfile"), []byte("FROM scratch"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}
