package hostenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverEmptyHome(t *testing.T) {
	home := t.TempDir()
	// Clear env vars that might leak from the test runner.
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("XDG_RUNTIME_DIR", "")
	// Neutralize gpgconf on PATH.
	t.Setenv("PATH", home)

	c := Discover(home)
	if c != (HostCreds{}) {
		t.Fatalf("empty home with cleared env should produce empty HostCreds, got: %+v", c)
	}
}

func TestDiscoverFullyPopulatedHome(t *testing.T) {
	home := t.TempDir()

	// ssh-agent.
	sshSock := filepath.Join(home, "ssh.sock")
	mustTouch(t, sshSock, 0o600)
	t.Setenv("SSH_AUTH_SOCK", sshSock)

	// gpg-agent via XDG_RUNTIME_DIR (we skip PATH-based gpgconf here).
	xdg := filepath.Join(home, "run")
	gnupgRunDir := filepath.Join(xdg, "gnupg")
	if err := os.MkdirAll(gnupgRunDir, 0o700); err != nil {
		t.Fatal(err)
	}
	gpgSock := filepath.Join(gnupgRunDir, "S.gpg-agent")
	mustTouch(t, gpgSock, 0o600)
	t.Setenv("XDG_RUNTIME_DIR", xdg)
	// Make sure gpgconf isn't found (PATH points at nothing useful).
	t.Setenv("PATH", filepath.Join(home, "no-such-dir"))

	// gpg material.
	gpgHome := filepath.Join(home, ".gnupg")
	mustMkdir(t, filepath.Join(gpgHome, "public-keys.d"), 0o700)
	mustTouch(t, filepath.Join(gpgHome, "pubring.kbx"), 0o600)
	mustTouch(t, filepath.Join(gpgHome, "trustdb.gpg"), 0o600)
	mustTouch(t, filepath.Join(gpgHome, "ownertrust"), 0o600)

	// Git + gh.
	mustTouch(t, filepath.Join(home, ".gitconfig"), 0o644)
	mustMkdir(t, filepath.Join(home, ".config", "gh"), 0o755)

	// Claude.
	claudeDir := filepath.Join(home, ".claude")
	mustMkdir(t, claudeDir, 0o755)
	mustTouch(t, filepath.Join(claudeDir, "settings.json"), 0o644)
	mustTouch(t, filepath.Join(claudeDir, "CLAUDE.md"), 0o644)
	mustMkdir(t, filepath.Join(claudeDir, "agents"), 0o755)

	c := Discover(home)

	if c.SSHAgentSock != sshSock {
		t.Errorf("SSHAgentSock = %q, want %q", c.SSHAgentSock, sshSock)
	}
	if c.GPGAgentSock != gpgSock {
		t.Errorf("GPGAgentSock = %q, want %q", c.GPGAgentSock, gpgSock)
	}
	if c.GPGKeyboxDir != filepath.Join(gpgHome, "public-keys.d") {
		t.Errorf("GPGKeyboxDir = %q", c.GPGKeyboxDir)
	}
	if c.GPGPubringKbx != filepath.Join(gpgHome, "pubring.kbx") {
		t.Errorf("GPGPubringKbx = %q", c.GPGPubringKbx)
	}
	if c.GPGTrustdb != filepath.Join(gpgHome, "trustdb.gpg") {
		t.Errorf("GPGTrustdb = %q", c.GPGTrustdb)
	}
	if c.GPGOwnerTrust != filepath.Join(gpgHome, "ownertrust") {
		t.Errorf("GPGOwnerTrust = %q", c.GPGOwnerTrust)
	}
	if c.GitConfig != filepath.Join(home, ".gitconfig") {
		t.Errorf("GitConfig = %q", c.GitConfig)
	}
	if c.GHConfig != filepath.Join(home, ".config", "gh") {
		t.Errorf("GHConfig = %q", c.GHConfig)
	}
	if c.ClaudeSettings != filepath.Join(claudeDir, "settings.json") {
		t.Errorf("ClaudeSettings = %q", c.ClaudeSettings)
	}
	if c.ClaudeMd != filepath.Join(claudeDir, "CLAUDE.md") {
		t.Errorf("ClaudeMd = %q", c.ClaudeMd)
	}
	if c.ClaudeAgents != filepath.Join(claudeDir, "agents") {
		t.Errorf("ClaudeAgents = %q", c.ClaudeAgents)
	}
}

func TestDiscoverGPG21KeyboxOnly(t *testing.T) {
	// On a fresh gpg 2.1+ setup the keybox dir exists but pubring.kbx
	// doesn't. Both should be emittable independently.
	home := t.TempDir()
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("PATH", filepath.Join(home, "no-such"))

	gpgHome := filepath.Join(home, ".gnupg")
	mustMkdir(t, filepath.Join(gpgHome, "public-keys.d"), 0o700)
	mustTouch(t, filepath.Join(gpgHome, "trustdb.gpg"), 0o600)

	c := Discover(home)

	if c.GPGKeyboxDir == "" {
		t.Error("expected keybox dir to be discovered")
	}
	if c.GPGPubringKbx != "" {
		t.Errorf("pubring.kbx shouldn't be discovered when absent, got %q", c.GPGPubringKbx)
	}
	if c.GPGTrustdb == "" {
		t.Error("expected trustdb to be discovered")
	}
}

func TestDiscoverLegacyPubringOnly(t *testing.T) {
	// Legacy gpg / keybox compat: only pubring.kbx exists, no public-keys.d.
	home := t.TempDir()
	t.Setenv("SSH_AUTH_SOCK", "")
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("PATH", filepath.Join(home, "no-such"))

	gpgHome := filepath.Join(home, ".gnupg")
	mustMkdir(t, gpgHome, 0o700)
	mustTouch(t, filepath.Join(gpgHome, "pubring.kbx"), 0o600)

	c := Discover(home)
	if c.GPGPubringKbx == "" {
		t.Error("expected pubring.kbx to be discovered")
	}
	if c.GPGKeyboxDir != "" {
		t.Errorf("keybox dir shouldn't be discovered when absent, got %q", c.GPGKeyboxDir)
	}
}

func TestSSHAgentIgnoredWhenSocketMissing(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "/does/not/exist/agent.sock")
	if got := discoverSSHAgent(); got != "" {
		t.Errorf("missing socket should produce empty string, got %q", got)
	}
}

func TestGPGAgentPrefersGpgconf(t *testing.T) {
	home := t.TempDir()

	// Put a fake gpgconf on PATH that prints a known socket.
	stubDir := t.TempDir()
	sock := filepath.Join(home, "gpgconf-sock")
	mustTouch(t, sock, 0o600)

	stub := filepath.Join(stubDir, "gpgconf")
	script := "#!/bin/sh\necho " + sock + "\n"
	if err := os.WriteFile(stub, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", stubDir)
	t.Setenv("XDG_RUNTIME_DIR", "") // ensure fallback isn't used
	if got := discoverGPGAgent(home); got != sock {
		t.Errorf("discoverGPGAgent = %q, want %q (via gpgconf)", got, sock)
	}
}

func TestGPGAgentFallsBackToXDG(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PATH", filepath.Join(home, "no-such"))

	xdg := filepath.Join(home, "run")
	mustMkdir(t, filepath.Join(xdg, "gnupg"), 0o700)
	sock := filepath.Join(xdg, "gnupg", "S.gpg-agent")
	mustTouch(t, sock, 0o600)
	t.Setenv("XDG_RUNTIME_DIR", xdg)

	if got := discoverGPGAgent(home); got != sock {
		t.Errorf("discoverGPGAgent = %q, want XDG fallback %q", got, sock)
	}
}

func TestGPGAgentFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("PATH", filepath.Join(home, "no-such"))
	t.Setenv("XDG_RUNTIME_DIR", "")

	gpgHome := filepath.Join(home, ".gnupg")
	mustMkdir(t, gpgHome, 0o700)
	sock := filepath.Join(gpgHome, "S.gpg-agent")
	mustTouch(t, sock, 0o600)

	if got := discoverGPGAgent(home); got != sock {
		t.Errorf("discoverGPGAgent = %q, want home fallback %q", got, sock)
	}
}

// ----- helpers ---------------------------------------------------------------

func mustTouch(t *testing.T, p string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func mustMkdir(t *testing.T, p string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(p, mode); err != nil {
		t.Fatal(err)
	}
}
