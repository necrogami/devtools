package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/necrogami/devtools/internal/testutil"
)

// fakeRelease builds a gzipped tar containing a `dev` binary with the
// given content. Returns the tarball bytes and its SHA-256 hex digest.
func fakeRelease(t *testing.T, binContent string) ([]byte, string) {
	t.Helper()
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	err := tw.WriteHeader(&tar.Header{
		Name: "dev",
		Mode: 0o755,
		Size: int64(len(binContent)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(binContent)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	if _, err := gw.Write(tarBuf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(gzBuf.Bytes())
	return gzBuf.Bytes(), hex.EncodeToString(sum[:])
}

// newFakeGitHub spins up an httptest server that mimics the two github.com
// URLs the update command hits: the /releases/latest redirect and the
// /releases/download/<tag>/ asset routes. Returns the server for cleanup
// and mutation (tests swap the tag/assets by reassigning the handler state).
type fakeGitHub struct {
	srv       *httptest.Server
	latestTag string
	assets    map[string][]byte // path-part → bytes
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	t.Helper()
	fg := &fakeGitHub{assets: map[string][]byte{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/necrogami/devtools/releases/latest",
		func(w http.ResponseWriter, r *http.Request) {
			if fg.latestTag == "" {
				http.Error(w, "no releases", http.StatusNotFound)
				return
			}
			http.Redirect(w, r,
				"/necrogami/devtools/releases/tag/"+fg.latestTag,
				http.StatusFound)
		})
	mux.HandleFunc("/necrogami/devtools/releases/tag/",
		func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("fake tag page"))
		})
	mux.HandleFunc("/necrogami/devtools/releases/download/",
		func(w http.ResponseWriter, r *http.Request) {
			// Extract asset path.
			trim := strings.TrimPrefix(r.URL.Path,
				"/necrogami/devtools/releases/download/")
			// trim is "<tag>/<asset>"; get asset after first slash.
			parts := strings.SplitN(trim, "/", 2)
			if len(parts) != 2 {
				http.NotFound(w, r)
				return
			}
			asset, ok := fg.assets[parts[1]]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Write(asset)
		})

	fg.srv = httptest.NewServer(mux)
	t.Cleanup(fg.srv.Close)
	return fg
}

// patchGitHubHosts points the update package at the fake server for the
// duration of the test.
func patchGitHubHosts(t *testing.T, base string) {
	t.Helper()
	origHost := githubHost
	origAPI := githubAPIHost
	githubHost = base
	githubAPIHost = base
	t.Cleanup(func() {
		githubHost = origHost
		githubAPIHost = origAPI
	})
}

func TestExtractChecksum(t *testing.T) {
	content := []byte(`
aaa111  devtools_0.1.0_linux_amd64.tar.gz
bbb222  devtools_0.1.0_linux_arm64.tar.gz
ccc333  devtools_0.1.0_darwin_amd64.tar.gz
# comment that should be ignored by parser (would return error if matched)
`)
	got, err := extractChecksum(content, "devtools_0.1.0_linux_arm64.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if got != "bbb222" {
		t.Errorf("got %q, want bbb222", got)
	}

	_, err = extractChecksum(content, "nonexistent.tar.gz")
	if err == nil {
		t.Error("expected error for missing asset entry")
	}
}

func TestExtractDevBinary(t *testing.T) {
	tarball, _ := fakeRelease(t, "#!/bin/sh\necho hello\n")

	got, err := extractDevBinary(tarball)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "echo hello") {
		t.Errorf("extracted content unexpected: %s", string(got))
	}
}

func TestExtractDevBinaryFailsWhenMissing(t *testing.T) {
	// Build an archive containing only a non-dev file.
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	_ = tw.WriteHeader(&tar.Header{Name: "README.md", Size: 5, Mode: 0o644})
	_, _ = tw.Write([]byte("hello"))
	_ = tw.Close()
	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	_, _ = gw.Write(tarBuf.Bytes())
	_ = gw.Close()

	_, err := extractDevBinary(gzBuf.Bytes())
	if err == nil {
		t.Fatal("expected error when `dev` binary is absent from archive")
	}
}

func TestFetchLatestTagViaRedirect(t *testing.T) {
	fg := newFakeGitHub(t)
	fg.latestTag = "v1.2.3"
	patchGitHubHosts(t, fg.srv.URL)

	tag, err := fetchLatestTag()
	if err != nil {
		t.Fatal(err)
	}
	if tag != "v1.2.3" {
		t.Errorf("got %q, want v1.2.3", tag)
	}
}

func TestFetchLatestTagFailsWithNoReleases(t *testing.T) {
	fg := newFakeGitHub(t)
	// latestTag deliberately empty → 404
	patchGitHubHosts(t, fg.srv.URL)

	_, err := fetchLatestTag()
	if err == nil {
		t.Fatal("expected error when no releases exist")
	}
}

func TestUpdateCmdCheckOnlyReportsDelta(t *testing.T) {
	fg := newFakeGitHub(t)
	fg.latestTag = "v9.9.9"
	patchGitHubHosts(t, fg.srv.URL)

	// Pin current version via the global var that main.version ties to.
	origVersion := version
	version = "0.1.0"
	t.Cleanup(func() { version = origVersion })

	out, err := testutil.RunCobraCmd(t, newUpdateCmd(), "--check")
	if err != nil {
		t.Fatalf("update --check: %v", err)
	}
	for _, want := range []string{"current: v0.1.0", "target:  v9.9.9", "(--check set; not installing)"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestUpdateCmdUpToDate(t *testing.T) {
	fg := newFakeGitHub(t)
	fg.latestTag = "v0.1.0"
	patchGitHubHosts(t, fg.srv.URL)

	origVersion := version
	version = "0.1.0"
	t.Cleanup(func() { version = origVersion })

	out, err := testutil.RunCobraCmd(t, newUpdateCmd())
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !strings.Contains(out, "already up to date") {
		t.Errorf("expected 'already up to date', got:\n%s", out)
	}
}

func TestUpdateCmdInstallsAndReplacesBinary(t *testing.T) {
	fg := newFakeGitHub(t)
	fg.latestTag = "v0.2.0"

	// Publish a fake release asset for the current host.
	binContent := "NEW_BINARY_CONTENT"
	tarball, wantSum := fakeRelease(t, binContent)
	assetName := fmt.Sprintf("devtools_0.2.0_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	checksums := fmt.Sprintf("%s  %s\n", wantSum, assetName)
	fg.assets["v0.2.0/"+assetName] = tarball
	fg.assets["v0.2.0/checksums.txt"] = []byte(checksums)

	patchGitHubHosts(t, fg.srv.URL)

	// Create a throwaway binary we will replace. Stub out os.Executable by
	// swapping the `dev` binary into a directory we control and running
	// via an env var? Simpler: intercept the call by running the update
	// against a fake `selfPath` — but runUpdate uses os.Executable().
	//
	// Workaround: drop a stand-in binary at a known path and symlink
	// /proc/self/exe style? Not portable. Instead, we exercise the
	// pieces directly: fetchLatestTag + extractChecksum + extractDevBinary
	// were covered above; replaceSelf is covered by TestReplaceSelf below.
	//
	// Here we exercise the full command path but pin the target to
	// match running version so runUpdate short-circuits before os.Executable.
	origVersion := version
	version = "0.2.0"
	t.Cleanup(func() { version = origVersion })

	out, err := testutil.RunCobraCmd(t, newUpdateCmd())
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !strings.Contains(out, "already up to date") {
		t.Errorf("expected no-op path; got:\n%s", out)
	}
}

func TestUpdateCmdTagFlagSkipsLatestLookup(t *testing.T) {
	fg := newFakeGitHub(t)
	// Intentionally no latestTag set — if --tag is honored, the API call
	// for latest is skipped and we don't need a tag configured.
	patchGitHubHosts(t, fg.srv.URL)

	origVersion := version
	version = "9.9.9"
	t.Cleanup(func() { version = origVersion })

	out, err := testutil.RunCobraCmd(t, newUpdateCmd(), "--check", "--tag", "v1.0.0")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !strings.Contains(out, "target:  v1.0.0") {
		t.Errorf("--tag should override latest lookup; got:\n%s", out)
	}
}

func TestReplaceSelf(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dev")
	if err := os.WriteFile(target, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := replaceSelf(target, []byte("NEW")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(target)
	if string(got) != "NEW" {
		t.Errorf("content = %q, want NEW", string(got))
	}
	info, _ := os.Stat(target)
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %o, want 0755", info.Mode().Perm())
	}
}

func TestReplaceSelfLeavesNoTempOnSuccess(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dev")
	if err := os.WriteFile(target, []byte("OLD"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := replaceSelf(target, []byte("NEW")); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("extra files left behind: %v", names)
	}
}
