package main

import (
	"os"
	"path/filepath"
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

	// Override file must have been rendered alongside the static compose.
	override := filepath.Join(projDir, "docker-compose.override.yml")
	if _, err := os.Stat(override); err != nil {
		t.Errorf("dev up should render docker-compose.override.yml: %v", err)
	}
}

func TestUpCmdRendersOverrideFromDiscovery(t *testing.T) {
	root := testutil.MakeFakeRepo(t, nil)
	projDir := makeProject(t, root, "p", "PROJECT=p\n")
	t.Chdir(root)
	repoFlag = ""

	home := testutil.WithFakeHome(t)

	// Pre-populate host state so discovery finds gitconfig + ssh-agent.
	sshSock := filepath.Join(home, "ssh.sock")
	if err := os.WriteFile(sshSock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SSH_AUTH_SOCK", sshSock)
	t.Setenv("XDG_RUNTIME_DIR", "")
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	testutil.WithFakeDocker(t, dockerStubLogsCompose)

	if _, err := testutil.RunCobraCmd(t, newUpCmd(), "p"); err != nil {
		t.Fatal(err)
	}

	override := filepath.Join(projDir, "docker-compose.override.yml")
	data, err := os.ReadFile(override)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, sshSock+":/run/host/ssh-agent") {
		t.Errorf("override missing ssh-agent mount for %q:\n%s", sshSock, s)
	}
	if !strings.Contains(s, filepath.Join(home, ".gitconfig")+":/home/dev/.gitconfig:ro") {
		t.Errorf("override missing gitconfig mount:\n%s", s)
	}
	// We can't assert the absence of gpg mounts here — the dev box
	// running the tests may have a real gpg-agent socket that
	// discoverGPGAgent will find via `gpgconf` on PATH. Unit tests for
	// hostenv.Discover cover the per-field emit/omit logic hermetically;
	// this integration test only verifies the plumbing from up.go to
	// the override renderer.
}

func TestUpCmdDegradesWithNoHostCreds(t *testing.T) {
	// No ssh-agent, no gpg, no gitconfig — the previous hard-fail would
	// have blocked this case. Under discovery, dev up should still render
	// an override and proceed to call compose.
	root := testutil.MakeFakeRepo(t, nil)
	projDir := makeProject(t, root, "p", "PROJECT=p\n")
	t.Chdir(root)
	repoFlag = ""

	testutil.WithFakeHome(t)
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("XDG_RUNTIME_DIR", "")
	log := testutil.WithFakeDocker(t, dockerStubLogsCompose)
	// Plant a failing gpgconf alongside the docker stub so hostenv.Discover
	// doesn't pick up a real one from the host's /usr/bin.
	if err := os.WriteFile(filepath.Join(filepath.Dir(log), "gpgconf"),
		[]byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := testutil.RunCobraCmd(t, newUpCmd(), "p"); err != nil {
		t.Fatalf("up with no creds should succeed, got: %v", err)
	}
	if data, _ := os.ReadFile(log); !strings.Contains(string(data), "compose up -d") {
		t.Errorf("compose up should still run with no creds; log:\n%s", string(data))
	}
	override := filepath.Join(projDir, "docker-compose.override.yml")
	data, err := os.ReadFile(override)
	if err != nil {
		t.Fatalf("override must exist even with no creds: %v", err)
	}
	s := string(data)
	// No ssh agent, so no env var and no socket mount.
	if strings.Contains(s, "SSH_AUTH_SOCK") {
		t.Errorf("override should omit SSH_AUTH_SOCK env when ssh not forwarded:\n%s", s)
	}
	if strings.Contains(s, "/run/host/ssh-agent") {
		t.Errorf("override should omit ssh-agent mount when ssh not forwarded:\n%s", s)
	}
	// No gpg-agent either.
	if strings.Contains(s, "gpg-agent") {
		t.Errorf("override should omit gpg-agent mount when no gpg discovered:\n%s", s)
	}
	// Devtools-specific baseline paths that preflight always auto-creates
	// are still mounted — that's by design, so the container has a
	// consistent filesystem shape across hosts.
	if !strings.Contains(s, "/home/dev/.config/gh:ro") {
		t.Errorf("override should still mount .config/gh (preflight-created baseline):\n%s", s)
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
