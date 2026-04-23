package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/necrogami/devtools/internal/testutil"
)

func TestCheckDockerDaemon(t *testing.T) {
	testutil.WithFakeDocker(t, `
if [ "$1" = "version" ]; then
    echo "28.0.0"
    exit 0
fi
exit 0`)

	r := checkDockerDaemon()
	if r.lvl != pass {
		t.Errorf("expected pass, got %v: %s", r.lvl, r.msg)
	}
	if !strings.Contains(r.msg, "28.0.0") {
		t.Errorf("message should include server version: %s", r.msg)
	}
}

func TestCheckDockerDaemonFailsWhenMissing(t *testing.T) {
	// Put an empty dir on PATH so no docker binary is found.
	dir := t.TempDir()
	t.Setenv("PATH", dir)

	r := checkDockerDaemon()
	if r.lvl != fail {
		t.Errorf("expected fail when docker absent, got %v", r.lvl)
	}
}

func TestCheckBuildx(t *testing.T) {
	testutil.WithFakeDocker(t, `
if [ "$1" = "buildx" ]; then
    echo "v0.14.0"
    exit 0
fi
exit 1`)

	r := checkBuildx()
	if r.lvl != pass {
		t.Errorf("expected pass, got %v: %s", r.lvl, r.msg)
	}
}

func TestCheckBuildxMissing(t *testing.T) {
	testutil.WithFakeDocker(t, `exit 1`)
	r := checkBuildx()
	if r.lvl == pass {
		t.Error("expected non-pass when buildx missing")
	}
}

func TestCheckSSHAgentUnset(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	r := checkSSHAgent()
	if r.lvl != warn {
		t.Errorf("expected warn, got %v: %s", r.lvl, r.msg)
	}
}

func TestCheckSSHAgentMissingSocket(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "/nonexistent/socket")
	r := checkSSHAgent()
	if r.lvl != warn {
		t.Errorf("expected warn, got %v: %s", r.lvl, r.msg)
	}
}

func TestCheckGPGAgentMissing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	// don't create gnupg/S.gpg-agent
	r := checkGPGAgent()
	if r.lvl != warn {
		t.Errorf("expected warn, got %v", r.lvl)
	}
}

func TestCheckGPGAgentPresent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", dir)
	sock := filepath.Join(dir, "gnupg", "S.gpg-agent")
	if err := os.MkdirAll(filepath.Dir(sock), 0o700); err != nil {
		t.Fatal(err)
	}
	// Regular file stands in for a socket; check only looks at Stat.
	if err := os.WriteFile(sock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	r := checkGPGAgent()
	if r.lvl != pass {
		t.Errorf("expected pass, got %v: %s", r.lvl, r.msg)
	}
}

func TestCheckGHConfig(t *testing.T) {
	home := testutil.WithFakeHome(t)

	r := checkGHConfig()
	if r.lvl != warn {
		t.Errorf("expected warn when gh hosts.yml absent, got %v", r.lvl)
	}

	p := filepath.Join(home, ".config", "gh", "hosts.yml")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("github.com:\n  user: me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r = checkGHConfig()
	if r.lvl != pass {
		t.Errorf("expected pass with hosts.yml present, got %v: %s", r.lvl, r.msg)
	}
}

func TestCheckSharedVolumesAllMissing(t *testing.T) {
	testutil.WithFakeDocker(t, `exit 1`) // every inspect says missing

	r := checkSharedVolumes()
	if r.lvl != warn {
		t.Errorf("expected warn, got %v", r.lvl)
	}
	if !strings.Contains(r.msg, "missing") {
		t.Errorf("message should mention missing: %s", r.msg)
	}
}

func TestLevelTags(t *testing.T) {
	cases := map[level]string{
		pass: " OK ",
		warn: "WARN",
		fail: "FAIL",
	}
	for lvl, want := range cases {
		if got := lvl.tag(); got != want {
			t.Errorf("level %v tag = %q, want %q", lvl, got, want)
		}
	}
}

func TestDoctorCmdFailsHard(t *testing.T) {
	// Force docker daemon check to FAIL. All other checks can be whatever.
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	testutil.WithFakeHome(t)

	_, err := testutil.RunCobraCmd(t, newDoctorCmd())
	if err == nil {
		t.Error("doctor should return error when a check fails")
	}
}
