package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/necrogami/devtools/internal/testutil"
)

// clearAgentEnv zeroes out env + PATH so hostenv.Discover can't latch
// onto a real gpg-agent / gpgconf on the test host. Returns the
// isolated $HOME path so tests can seed specific files under it.
func clearAgentEnv(t *testing.T) string {
	t.Helper()
	home := testutil.WithFakeHome(t)
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("XDG_RUNTIME_DIR", "")
	// Directory with nothing in it — gpgconf and similar won't resolve.
	t.Setenv("PATH", filepath.Join(home, ".no-such-bin"))
	return home
}

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
	clearAgentEnv(t)
	r := checkGPGAgent()
	if r.lvl != warn {
		t.Errorf("expected warn, got %v: %s", r.lvl, r.msg)
	}
	if !strings.Contains(r.msg, "gpgconf") {
		t.Errorf("missing-gpg message should cite the discovery chain: %s", r.msg)
	}
}

func TestCheckGPGAgentPresentViaXDG(t *testing.T) {
	home := clearAgentEnv(t)
	// Place the socket under a fake XDG dir and re-set the env so
	// hostenv.discoverGPGAgent picks it up through its XDG fallback.
	xdg := filepath.Join(home, "run")
	if err := os.MkdirAll(filepath.Join(xdg, "gnupg"), 0o700); err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(xdg, "gnupg", "S.gpg-agent")
	if err := os.WriteFile(sock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", xdg)

	r := checkGPGAgent()
	if r.lvl != pass {
		t.Errorf("expected pass, got %v: %s", r.lvl, r.msg)
	}
	if r.msg != sock {
		t.Errorf("expected message to be the socket path %q, got %q", sock, r.msg)
	}
}

func TestCheckKeyboxdNotInUse(t *testing.T) {
	clearAgentEnv(t)
	r := checkKeyboxd()
	if r.lvl != pass {
		t.Errorf("expected pass when keyboxd not configured, got %v: %s", r.lvl, r.msg)
	}
	if !strings.Contains(r.msg, "not in use") {
		t.Errorf("message should describe keyboxd as unused: %s", r.msg)
	}
}

func TestCheckKeyboxdConfiguredButNoSocket(t *testing.T) {
	home := clearAgentEnv(t)
	// use-keyboxd in common.conf but no socket anywhere → should FAIL.
	gpgHome := filepath.Join(home, ".gnupg")
	if err := os.MkdirAll(gpgHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gpgHome, "common.conf"),
		[]byte("# comment\nuse-keyboxd\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := checkKeyboxd()
	if r.lvl != fail {
		t.Errorf("expected fail when use-keyboxd set but socket absent, got %v: %s", r.lvl, r.msg)
	}
	if !strings.Contains(r.msg, "use-keyboxd") {
		t.Errorf("message should cite use-keyboxd: %s", r.msg)
	}
}

func TestCheckKeyboxdFullyConfigured(t *testing.T) {
	home := clearAgentEnv(t)
	// common.conf + socket present → PASS.
	gpgHome := filepath.Join(home, ".gnupg")
	if err := os.MkdirAll(gpgHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gpgHome, "common.conf"),
		[]byte("use-keyboxd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	xdg := filepath.Join(home, "run")
	if err := os.MkdirAll(filepath.Join(xdg, "gnupg"), 0o700); err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(xdg, "gnupg", "S.keyboxd")
	if err := os.WriteFile(sock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", xdg)

	r := checkKeyboxd()
	if r.lvl != pass {
		t.Errorf("expected pass, got %v: %s", r.lvl, r.msg)
	}
	if !strings.Contains(r.msg, "use-keyboxd") {
		t.Errorf("message should cite use-keyboxd: %s", r.msg)
	}
}

func TestCheckKeyboxdCommentedOut(t *testing.T) {
	// common.conf that only comments-mentions keyboxd should not trip
	// the "use-keyboxd configured" branch.
	home := clearAgentEnv(t)
	gpgHome := filepath.Join(home, ".gnupg")
	if err := os.MkdirAll(gpgHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gpgHome, "common.conf"),
		[]byte("# use-keyboxd  (disabled)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := checkKeyboxd()
	if r.lvl != pass {
		t.Errorf("commented-out use-keyboxd should not fail, got %v: %s", r.lvl, r.msg)
	}
}

func TestPrintForwardSummaryListsMissingAndPresent(t *testing.T) {
	home := clearAgentEnv(t)
	// Seed only gitconfig to prove a single + line appears and the rest
	// render as - lines with explanations.
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	printForwardSummary(&buf)
	s := buf.String()

	if !strings.Contains(s, "forwarded into container") {
		t.Errorf("summary header missing:\n%s", s)
	}
	if !strings.Contains(s, "+ gitconfig") {
		t.Errorf("gitconfig should appear as present (+):\n%s", s)
	}
	if !strings.Contains(s, "- ssh-agent socket") {
		t.Errorf("ssh-agent should appear as missing (-):\n%s", s)
	}
	if !strings.Contains(s, "- gpg-agent socket") {
		t.Errorf("gpg-agent should appear as missing (-):\n%s", s)
	}
	if !strings.Contains(s, "no agent on host") {
		t.Errorf("missing-ssh explanation should be inline:\n%s", s)
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
