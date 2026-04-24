package compose

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/necrogami/devtools/internal/hostenv"
)

func TestRenderOverrideEmptyCreds(t *testing.T) {
	var buf bytes.Buffer
	if err := RenderOverride(hostenv.HostCreds{}, &buf); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	// Must still contain the "services: tools:" skeleton so compose is happy.
	for _, want := range []string{"services:", "tools:", "nothing to forward"} {
		if !strings.Contains(s, want) {
			t.Errorf("empty-creds output missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "volumes:") {
		t.Errorf("empty-creds output should omit volumes: block:\n%s", s)
	}
	if strings.Contains(s, "SSH_AUTH_SOCK") {
		t.Errorf("empty-creds output should omit SSH_AUTH_SOCK:\n%s", s)
	}
}

func TestRenderOverrideFullCreds(t *testing.T) {
	c := hostenv.HostCreds{
		SSHAgentSock:   "/tmp/ssh.sock",
		GPGAgentSock:   "/run/user/1000/gnupg/S.gpg-agent",
		GPGKeyboxDir:   "/home/u/.gnupg/public-keys.d",
		GPGPubringKbx:  "/home/u/.gnupg/pubring.kbx",
		GPGTrustdb:     "/home/u/.gnupg/trustdb.gpg",
		GPGOwnerTrust:  "/home/u/.gnupg/ownertrust",
		GitConfig:      "/home/u/.gitconfig",
		GHConfig:       "/home/u/.config/gh",
		ClaudeSettings: "/home/u/.claude/settings.json",
		ClaudeMd:       "/home/u/.claude/CLAUDE.md",
		ClaudeAgents:   "/home/u/.claude/agents",
	}

	var buf bytes.Buffer
	if err := RenderOverride(c, &buf); err != nil {
		t.Fatal(err)
	}
	s := buf.String()

	// Every host path should land in the volumes list, paired with the
	// right container path.
	wants := []string{
		"SSH_AUTH_SOCK: /run/host/ssh-agent",
		"/tmp/ssh.sock:/run/host/ssh-agent",
		"/run/user/1000/gnupg/S.gpg-agent:/run/host/gpg-agent",
		"/home/u/.gnupg/public-keys.d:/home/dev/.gnupg/public-keys.d:ro",
		"/home/u/.gnupg/pubring.kbx:/home/dev/.gnupg/pubring.kbx:ro",
		"/home/u/.gnupg/trustdb.gpg:/home/dev/.gnupg/trustdb.gpg:ro",
		"/home/u/.gnupg/ownertrust:/home/dev/.gnupg/ownertrust:ro",
		"/home/u/.gitconfig:/home/dev/.gitconfig:ro",
		"/home/u/.config/gh:/home/dev/.config/gh:ro",
		"/home/u/.claude/settings.json:/home/dev/.claude/settings.json:ro",
		"/home/u/.claude/CLAUDE.md:/home/dev/.claude/CLAUDE.md:ro",
		"/home/u/.claude/agents:/home/dev/.claude/agents:ro",
	}
	for _, w := range wants {
		if !strings.Contains(s, w) {
			t.Errorf("output missing %q:\n%s", w, s)
		}
	}
}

func TestRenderOverrideSSHOnly(t *testing.T) {
	// Typical CI runner / headless box: only ssh-agent is forwarded.
	c := hostenv.HostCreds{SSHAgentSock: "/tmp/ssh.sock"}
	var buf bytes.Buffer
	if err := RenderOverride(c, &buf); err != nil {
		t.Fatal(err)
	}
	s := buf.String()

	if !strings.Contains(s, "SSH_AUTH_SOCK: /run/host/ssh-agent") {
		t.Errorf("missing env var with ssh present:\n%s", s)
	}
	if !strings.Contains(s, "/tmp/ssh.sock:/run/host/ssh-agent") {
		t.Errorf("missing ssh mount:\n%s", s)
	}
	// Nothing else should appear.
	for _, banned := range []string{"gpg-agent", ".gnupg", "gitconfig", "claude"} {
		if strings.Contains(s, banned) {
			t.Errorf("ssh-only output should not contain %q:\n%s", banned, s)
		}
	}
}

func TestRenderOverrideGPG21KeyboxOnly(t *testing.T) {
	// Fresh gpg 2.1+: only public-keys.d exists, no pubring.kbx.
	c := hostenv.HostCreds{
		GPGKeyboxDir: "/home/u/.gnupg/public-keys.d",
		GPGTrustdb:   "/home/u/.gnupg/trustdb.gpg",
	}
	var buf bytes.Buffer
	if err := RenderOverride(c, &buf); err != nil {
		t.Fatal(err)
	}
	s := buf.String()

	if !strings.Contains(s, "public-keys.d:/home/dev/.gnupg/public-keys.d:ro") {
		t.Errorf("expected keybox-dir mount:\n%s", s)
	}
	if strings.Contains(s, "pubring.kbx") {
		t.Errorf("pubring.kbx should be absent when GPGPubringKbx empty:\n%s", s)
	}
}

func TestWriteOverrideRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := hostenv.HostCreds{SSHAgentSock: "/tmp/sock"}

	if err := WriteOverride(dir, c); err != nil {
		t.Fatal(err)
	}
	path := OverrideFile(dir)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "/tmp/sock:/run/host/ssh-agent") {
		t.Errorf("persisted override missing mount:\n%s", string(data))
	}
	if !strings.HasPrefix(string(data), "# GENERATED") {
		t.Errorf("persisted override missing generated-by header:\n%s", string(data))
	}
}

func TestWriteOverrideOverwrites(t *testing.T) {
	dir := t.TempDir()
	// First write with ssh only.
	if err := WriteOverride(dir, hostenv.HostCreds{SSHAgentSock: "/old"}); err != nil {
		t.Fatal(err)
	}
	// Second write with different state must fully overwrite, not append.
	if err := WriteOverride(dir, hostenv.HostCreds{GitConfig: "/etc/gitconfig"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "docker-compose.override.yml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "/old") {
		t.Errorf("second write should not retain first payload:\n%s", s)
	}
	if !strings.Contains(s, "/etc/gitconfig") {
		t.Errorf("second write missing new payload:\n%s", s)
	}
}
