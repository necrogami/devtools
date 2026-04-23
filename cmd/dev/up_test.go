package main

import (
	"os"
	"strings"
	"testing"

	"github.com/necrogami/devtools/internal/testutil"
)

// dockerStubLogsCompose records all `docker compose ...` invocations from
// the generated project dir. Body writes one line per call to $DOCKER_LOG.
const dockerStubLogsCompose = `
echo "pwd=$PWD args=$*" >> "$DOCKER_LOG"
exit 0`

func TestUpCmdInvokesComposeUpDetached(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	projDir := makeProject(t, root, "p", "PROJECT=p\n")
	t.Chdir(root)
	repoFlag = ""

	home := testutil.WithFakeHome(t)
	log := testutil.WithFakeDocker(t, dockerStubLogsCompose)

	_, err := testutil.RunCobraCmd(t, newUpCmd(), "p")
	if err != nil {
		t.Fatalf("up: %v", err)
	}

	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.Contains(got, "pwd="+projDir) {
		t.Errorf("compose should run in project dir %s:\n%s", projDir, got)
	}
	if !strings.Contains(got, "args=compose up -d") {
		t.Errorf("expected 'compose up -d', got:\n%s", got)
	}

	// Pre-flight should have populated home/.claude.
	if _, err := os.Stat(home + "/.claude/CLAUDE.md"); err != nil {
		t.Errorf("pre-flight didn't create ~/.claude/CLAUDE.md: %v", err)
	}
}

func TestUpCmdNoDetach(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	_ = makeProject(t, root, "p", "PROJECT=p\n")
	t.Chdir(root)
	repoFlag = ""
	testutil.WithFakeHome(t)
	log := testutil.WithFakeDocker(t, dockerStubLogsCompose)

	_, err := testutil.RunCobraCmd(t, newUpCmd(), "p", "--detach=false")
	if err != nil {
		t.Fatalf("up: %v", err)
	}
	data, _ := os.ReadFile(log)
	if strings.Contains(string(data), "compose up -d") {
		t.Errorf("--detach=false should not emit -d; got:\n%s", string(data))
	}
	if !strings.Contains(string(data), "compose up") {
		t.Errorf("compose up missing:\n%s", string(data))
	}
}

func TestDownCmd(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	_ = makeProject(t, root, "p", "PROJECT=p\n")
	t.Chdir(root)
	repoFlag = ""
	log := testutil.WithFakeDocker(t, dockerStubLogsCompose)

	_, err := testutil.RunCobraCmd(t, newDownCmd(), "p")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(log)
	if !strings.Contains(string(data), "compose down") {
		t.Errorf("expected 'compose down', got:\n%s", string(data))
	}
	if strings.Contains(string(data), "--volumes") {
		t.Errorf("--volumes shouldn't be set without --volumes flag")
	}
}

func TestDownCmdWithVolumes(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	_ = makeProject(t, root, "p", "PROJECT=p\n")
	t.Chdir(root)
	repoFlag = ""
	log := testutil.WithFakeDocker(t, dockerStubLogsCompose)

	_, err := testutil.RunCobraCmd(t, newDownCmd(), "p", "--volumes")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(log)
	if !strings.Contains(string(data), "--volumes") {
		t.Errorf("--volumes flag should be forwarded, got:\n%s", string(data))
	}
}

func TestLogsCmdForwardsFlags(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	_ = makeProject(t, root, "p", "PROJECT=p\n")
	t.Chdir(root)
	repoFlag = ""
	log := testutil.WithFakeDocker(t, dockerStubLogsCompose)

	_, err := testutil.RunCobraCmd(t, newLogsCmd(), "p", "--follow", "db")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(log)
	got := string(data)
	for _, want := range []string{"compose logs", "--tail=200", "--follow", "db"} {
		if !strings.Contains(got, want) {
			t.Errorf("logs missing %q in:\n%s", want, got)
		}
	}
}

func TestPsCmdAllProjects(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	t.Chdir(root)
	repoFlag = ""
	log := testutil.WithFakeDocker(t, dockerStubLogsCompose)

	_, err := testutil.RunCobraCmd(t, newPsCmd())
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(log)
	if !strings.Contains(string(data), "--filter label=com.docker.compose.project") {
		t.Errorf("ps should filter by compose.project label:\n%s", string(data))
	}
}

func TestPsCmdSingleProject(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	projDir := makeProject(t, root, "p", "PROJECT=p\n")
	t.Chdir(root)
	repoFlag = ""
	log := testutil.WithFakeDocker(t, dockerStubLogsCompose)

	_, err := testutil.RunCobraCmd(t, newPsCmd(), "p")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(log)
	if !strings.Contains(string(data), "pwd="+projDir) {
		t.Errorf("single-project ps should run in project dir:\n%s", string(data))
	}
	if !strings.Contains(string(data), "args=compose ps") {
		t.Errorf("single-project ps should call compose ps:\n%s", string(data))
	}
}

func TestExecCmdForwardsRemainder(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	_ = makeProject(t, root, "p", "PROJECT=p\n")
	t.Chdir(root)
	repoFlag = ""
	log := testutil.WithFakeDocker(t, dockerStubLogsCompose)

	// cobra's SetInterspersed(false) passes "--" through as a positional arg;
	// docker exec itself accepts "--" as an end-of-flags terminator.
	_, err := testutil.RunCobraCmd(t, newExecCmd(), "p", "--", "php", "-v")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(log)
	if !strings.Contains(string(data), "exec -it p-tools") {
		t.Errorf("expected `exec -it p-tools` prefix, got:\n%s", string(data))
	}
	if !strings.Contains(string(data), "php -v") {
		t.Errorf("expected `php -v` forwarded, got:\n%s", string(data))
	}
}

func TestCommandsRequireKnownProject(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	t.Chdir(root)
	repoFlag = ""
	testutil.WithFakeDocker(t, `exit 0`)

	for _, tc := range []struct {
		name string
		cmd  func() error
	}{
		{"up", func() error {
			_, err := testutil.RunCobraCmd(t, newUpCmd(), "ghost")
			return err
		}},
		{"down", func() error {
			_, err := testutil.RunCobraCmd(t, newDownCmd(), "ghost")
			return err
		}},
		{"logs", func() error {
			_, err := testutil.RunCobraCmd(t, newLogsCmd(), "ghost")
			return err
		}},
		{"exec", func() error {
			_, err := testutil.RunCobraCmd(t, newExecCmd(), "ghost", "--", "echo")
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cmd(); err == nil {
				t.Fatal("expected error for missing project")
			}
		})
	}
}
