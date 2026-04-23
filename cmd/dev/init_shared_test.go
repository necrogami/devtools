package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/necrogami/devtools/internal/testutil"
)

func TestVolumeExistsTrueAndFalse(t *testing.T) {
	testutil.WithFakeDocker(t, `
if [ "$1" = "volume" ] && [ "$2" = "inspect" ] && [ "$3" = "present" ]; then
    exit 0
fi
exit 1`)

	ok, err := volumeExists("present")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected present=true")
	}
	ok, err = volumeExists("absent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected absent=false")
	}
}

func TestVolumeExistsReportsMissingDocker(t *testing.T) {
	// Point PATH somewhere with no docker at all.
	dir := t.TempDir()
	t.Setenv("PATH", dir)

	_, err := volumeExists("anything")
	if err == nil {
		t.Fatal("expected error when docker binary is absent")
	}
}

func TestEnsureVolumeCreatesWhenMissing(t *testing.T) {
	log := testutil.WithFakeDocker(t, `
case "$1 $2" in
    "volume inspect")
        # pretend nothing exists
        exit 1 ;;
    "volume create")
        echo "$3" >> "$DOCKER_LOG.created"
        exit 0 ;;
    *) exit 0 ;;
esac`)

	created, err := ensureVolume("devtools_x")
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Error("expected created=true")
	}
	data, _ := os.ReadFile(log + ".created")
	if !strings.Contains(string(data), "devtools_x") {
		t.Errorf("expected docker volume create invocation; log: %s", string(data))
	}
}

func TestEnsureVolumeSkipsWhenPresent(t *testing.T) {
	testutil.WithFakeDocker(t, `exit 0`) // volume inspect succeeds → exists
	created, err := ensureVolume("devtools_x")
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Error("expected created=false for existing volume")
	}
}

func TestInitSharedCreatesAllTenVolumes(t *testing.T) {
	log := testutil.WithFakeDocker(t, `
case "$1 $2" in
    "volume inspect")
        # treat everything as missing so ensureVolume creates all.
        exit 1 ;;
    "volume create")
        echo "$3" >> "$DOCKER_LOG.created"
        exit 0 ;;
    *)
        echo "unexpected docker call: $*" >&2
        exit 2 ;;
esac`)

	// Empty $HOME → no seeding.
	testutil.WithFakeHome(t)

	out, err := testutil.RunCobraCmd(t, newInitSharedCmd(), "--no-seed")
	if err != nil {
		t.Fatalf("init-shared: %v", err)
	}

	if !strings.Contains(out, "10 created") {
		t.Errorf("expected '10 created' summary, got:\n%s", out)
	}

	data, _ := os.ReadFile(log + ".created")
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 10 {
		t.Errorf("expected 10 volume-create calls, got %d:\n%s",
			len(lines), string(data))
	}

	// All expected volume names appear.
	for _, v := range sharedVolumes() {
		if !strings.Contains(string(data), v) {
			t.Errorf("volume %s was never created", v)
		}
	}
}

func TestInitSharedIdempotent(t *testing.T) {
	testutil.WithFakeDocker(t, `
case "$1 $2" in
    "volume inspect") exit 0 ;;
    *) exit 0 ;;
esac`)
	testutil.WithFakeHome(t)

	out, err := testutil.RunCobraCmd(t, newInitSharedCmd(), "--no-seed")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "0 created") {
		t.Errorf("expected '0 created' when all volumes exist, got:\n%s", out)
	}
}

func TestSharedVolumesListIncludesClaude(t *testing.T) {
	names := sharedVolumes()
	// Should include all 7 cache volumes + 3 claude volumes = 10.
	if len(names) != 10 {
		t.Fatalf("expected 10 total volumes, got %d: %v", len(names), names)
	}
	haveClaudeSkills := false
	for _, n := range names {
		if n == "devtools_claude_skills" {
			haveClaudeSkills = true
		}
	}
	if !haveClaudeSkills {
		t.Error("sharedVolumes() missing devtools_claude_skills")
	}
}

func TestInitSharedSeedsFromHomeWhenDirExists(t *testing.T) {
	log := testutil.WithFakeDocker(t, `
case "$1 $2" in
    "volume inspect") exit 1 ;;          # nothing exists yet
    "volume create")  exit 0 ;;           # creation succeeds
    "run --rm")
        # Capture args so we can verify seed was called with --user 1000:1000.
        echo "run $*" >> "$DOCKER_LOG.runs"
        exit 0 ;;
    *) exit 0 ;;
esac`)

	home := testutil.WithFakeHome(t)

	// Seed content on fake host ~/.claude/skills.
	skillsDir := filepath.Join(home, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "foo.md"), []byte("# skill"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := testutil.RunCobraCmd(t, newInitSharedCmd())
	if err != nil {
		t.Fatal(err)
	}

	runs, err := os.ReadFile(log + ".runs")
	if err != nil {
		t.Fatalf("seed container invocation log not produced: %v", err)
	}
	if !strings.Contains(string(runs), "--user 1000:1000") {
		t.Errorf("seed container didn't set --user 1000:1000:\n%s", string(runs))
	}
	if !strings.Contains(string(runs), "devtools_claude_skills") {
		t.Errorf("skills volume wasn't seeded:\n%s", string(runs))
	}
}
