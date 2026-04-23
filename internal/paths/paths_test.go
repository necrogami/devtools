package paths

import (
	"os"
	"path/filepath"
	"strings"
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
		{"a" + strings.Repeat("b", 31), false}, // exactly 32 chars → allowed
		{"", true},
		{"0starts-digit", true},
		{"has_underscore", true},
		{"HAS-UPPER", true},
		{"has.dot", true},
		{"has space", true},
		{"-leading-dash", true},
		{"a" + strings.Repeat("b", 32), true}, // 33 chars → rejected
		{"trailing-", false},                  // regex allows trailing dash
		{"ünicøde", true},
		{"has/slash", true},
		{"has\\backslash", true},
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

func TestResolveFindsRootFromDeepDir(t *testing.T) {
	root := makeFakeRepo(t)
	deep := filepath.Join(root, "cmd", "dev", "foo", "bar")
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
	wantSub := map[string]string{
		"Base":     filepath.Join(root, "base"),
		"Template": filepath.Join(root, "template"),
		"Projects": filepath.Join(root, "projects"),
		"Shared":   filepath.Join(root, "shared"),
	}
	got := map[string]string{
		"Base": r.Base, "Template": r.Template,
		"Projects": r.Projects, "Shared": r.Shared,
	}
	for k, v := range wantSub {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
}

func TestResolveUsesCwdWhenEmpty(t *testing.T) {
	root := makeFakeRepo(t)
	t.Chdir(root)

	r, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve(\"\"): %v", err)
	}
	if r.Root != root {
		t.Fatalf("Root = %q, want %q", r.Root, root)
	}
}

func TestResolveFailsOutsideRepo(t *testing.T) {
	dir := t.TempDir() // not a devtools repo
	_, err := Resolve(dir)
	if err == nil {
		t.Fatal("Resolve outside repo unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "not inside a devtools repo") {
		t.Errorf("error missing guidance: %v", err)
	}
}

func TestResolveRequiresBothMarkers(t *testing.T) {
	cases := []struct {
		name  string
		setup func(root string)
	}{
		{"SPEC.md only", func(r string) {
			_ = os.WriteFile(filepath.Join(r, "SPEC.md"), []byte("x"), 0o644)
		}},
		{"Dockerfile only", func(r string) {
			_ = os.MkdirAll(filepath.Join(r, "base"), 0o755)
			_ = os.WriteFile(filepath.Join(r, "base", "Dockerfile"), []byte("x"), 0o644)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			c.setup(dir)
			if _, err := Resolve(dir); err == nil {
				t.Fatal("expected Resolve to fail with partial markers")
			}
		})
	}
}

func TestProjectDir(t *testing.T) {
	root := makeFakeRepo(t)
	r, err := Resolve(root)
	if err != nil {
		t.Fatal(err)
	}

	got, err := r.ProjectDir("valid-name")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "projects", "valid-name")
	if got != want {
		t.Fatalf("ProjectDir = %q, want %q", got, want)
	}

	if _, err := r.ProjectDir("BAD NAME"); err == nil {
		t.Fatal("ProjectDir accepted invalid name")
	}
	if _, err := r.ProjectDir(""); err == nil {
		t.Fatal("ProjectDir accepted empty name")
	}
}

func TestListProjects(t *testing.T) {
	root := makeFakeRepo(t)
	for _, n := range []string{"one", "two", ".gitkeep-dir"} {
		if err := os.MkdirAll(filepath.Join(root, "projects", n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Also drop a dotfile and a regular file — neither should appear.
	if err := os.WriteFile(filepath.Join(root, "projects", ".gitkeep"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "projects", "NOTES.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"one", "two"}
	if len(got) != len(want) {
		t.Fatalf("ListProjects len = %d (%v), want %d (%v)", len(got), got, len(want), want)
	}
	for i, n := range want {
		if got[i] != n {
			t.Errorf("[%d] = %q, want %q", i, got[i], n)
		}
	}
}

func TestListProjectsHandlesMissingDir(t *testing.T) {
	root := t.TempDir()
	// Fake repo markers but no projects/ dir.
	_ = os.WriteFile(filepath.Join(root, "SPEC.md"), []byte("x"), 0o644)
	_ = os.MkdirAll(filepath.Join(root, "base"), 0o755)
	_ = os.WriteFile(filepath.Join(root, "base", "Dockerfile"), []byte("x"), 0o644)

	r, err := Resolve(root)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects with missing dir: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil slice for missing dir, got %v", got)
	}
}

// makeFakeRepo creates a temp dir containing the two marker files that
// identify a devtools repo root. Kept local (small) so paths_test doesn't
// import the testutil package cyclically.
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
