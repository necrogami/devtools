package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// GitHub host roots. Exposed as vars so tests can redirect to an httptest
// server without monkey-patching exec.Command-style dependencies.
var (
	githubHost    = "https://github.com"
	githubAPIHost = "https://api.github.com" // reserved for future use
	updateRepo    = "necrogami/devtools"
)

// httpClient is the client all update-path HTTP calls flow through.
// Tests can swap this for one targeting an httptest server.
var httpClient = http.DefaultClient

func newUpdateCmd() *cobra.Command {
	var (
		checkOnly bool
		pinTag    string
		force     bool
		pullImage bool
	)
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update the `dev` CLI (and optionally the base image) to the latest release",
		Long: "Self-update the running `dev` binary from the latest GitHub release.\n" +
			"Verifies the download against the published SHA-256 checksum before\n" +
			"atomically replacing the current binary.\n\n" +
			"Use --image to also `docker pull` the latest devtools base image.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUpdate(cmd, runUpdateOpts{
				CheckOnly: checkOnly,
				PinTag:    pinTag,
				Force:     force,
				PullImage: pullImage,
			})
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false,
		"only report what would be updated; do not download or install")
	cmd.Flags().StringVar(&pinTag, "tag", "",
		"install a specific CLI version (e.g. v0.1.0) instead of the latest")
	cmd.Flags().BoolVar(&force, "force", false,
		"reinstall even if the target version matches the currently running one")
	cmd.Flags().BoolVar(&pullImage, "image", false,
		"also `docker pull ghcr.io/necrogami/devtools:latest`")
	return cmd
}

type runUpdateOpts struct {
	CheckOnly bool
	PinTag    string
	Force     bool
	PullImage bool
}

func runUpdate(cmd *cobra.Command, opts runUpdateOpts) error {
	out := cmd.OutOrStdout()

	// ----- resolve target version -------------------------------------------
	target := opts.PinTag
	if target == "" {
		latest, err := fetchLatestTag()
		if err != nil {
			return fmt.Errorf("resolve latest tag: %w", err)
		}
		target = latest
	}
	// GoReleaser injects a `v`-prefixed tag (`v0.3.0`); `git describe`-based
	// dev builds do the same. Source-tree builds leave it at the literal
	// "dev". Normalize either form to a "v"-prefixed string so the
	// current/target comparison below is apples-to-apples.
	current := version
	if current != "" && !strings.HasPrefix(current, "v") {
		current = "v" + current
	}

	fmt.Fprintf(out, "current: %s\ntarget:  %s\n", current, target)

	if !opts.Force && target == current {
		fmt.Fprintln(out, "already up to date")
		if opts.PullImage {
			return pullDevtoolsImage(cmd)
		}
		return nil
	}

	if opts.CheckOnly {
		fmt.Fprintln(out, "(--check set; not installing)")
		return nil
	}

	// ----- fetch + verify ---------------------------------------------------
	stripped := strings.TrimPrefix(target, "v")
	asset := fmt.Sprintf("devtools_%s_%s_%s.tar.gz",
		stripped, runtime.GOOS, runtime.GOARCH)

	base := fmt.Sprintf("%s/%s/releases/download/%s",
		githubHost, updateRepo, target)
	fmt.Fprintf(out, "fetching %s/%s\n", base, asset)

	tarball, err := httpGetBytes(base + "/" + asset)
	if err != nil {
		return fmt.Errorf("download %s: %w", asset, err)
	}
	checksums, err := httpGetBytes(base + "/checksums.txt")
	if err != nil {
		return fmt.Errorf("download checksums.txt: %w", err)
	}

	wantSum, err := extractChecksum(checksums, asset)
	if err != nil {
		return err
	}
	gotSum := sha256.Sum256(tarball)
	if hex.EncodeToString(gotSum[:]) != wantSum {
		return fmt.Errorf("checksum mismatch for %s: want %s, got %s",
			asset, wantSum, hex.EncodeToString(gotSum[:]))
	}
	fmt.Fprintln(out, "checksum OK")

	// ----- extract binary ---------------------------------------------------
	binBytes, err := extractDevBinary(tarball)
	if err != nil {
		return fmt.Errorf("extract binary: %w", err)
	}

	// ----- atomic replace ---------------------------------------------------
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve self path: %w", err)
	}
	selfPath, err = filepath.EvalSymlinks(selfPath)
	if err != nil {
		return fmt.Errorf("resolve symlink %s: %w", selfPath, err)
	}
	if err := replaceSelf(selfPath, binBytes); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}
	fmt.Fprintf(out, "installed %s at %s\n", target, selfPath)

	if opts.PullImage {
		return pullDevtoolsImage(cmd)
	}
	return nil
}

// fetchLatestTag follows GitHub's /releases/latest redirect to the tag URL
// and returns the tag portion. No JSON parsing; no jq dependency.
func fetchLatestTag() (string, error) {
	url := fmt.Sprintf("%s/%s/releases/latest", githubHost, updateRepo)
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	final := resp.Request.URL.String()
	tag := final[strings.LastIndex(final, "/")+1:]
	if tag == "" || tag == "latest" {
		return "", fmt.Errorf("could not resolve latest tag from %s", final)
	}
	return tag, nil
}

// httpGetBytes fetches a URL and returns the full body or an error for any
// non-2xx response.
func httpGetBytes(url string) ([]byte, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// extractChecksum parses a goreleaser-format checksums.txt
// ("<sha256>  <filename>" per line) and returns the sum for asset.
func extractChecksum(data []byte, asset string) (string, error) {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[1] == asset {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("no checksum entry for %s", asset)
}

// extractDevBinary reads a gzipped tar and returns the contents of the
// entry named "dev".
func extractDevBinary(tarball []byte) ([]byte, error) {
	gz, err := gzip.NewReader(strings.NewReader(string(tarball)))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil, errors.New("archive did not contain a `dev` binary")
		}
		if err != nil {
			return nil, err
		}
		if filepath.Base(h.Name) == "dev" && h.Typeflag != tar.TypeDir {
			return io.ReadAll(tr)
		}
	}
}

// replaceSelf writes newBytes to a sibling tempfile next to target and
// atomically renames over target. Unix rename-over-running-binary is safe
// (the kernel keeps the old inode alive for the current process).
func replaceSelf(target string, newBytes []byte) error {
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, "dev.new-*")
	if err != nil {
		return err
	}
	cleanup := func() { os.Remove(tmp.Name()) }
	if _, err := tmp.Write(newBytes); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmp.Name(), target); err != nil {
		cleanup()
		return err
	}
	return nil
}

// pullDevtoolsImage shells out to `docker pull` for the canonical latest image.
func pullDevtoolsImage(cmd *cobra.Command) error {
	fmt.Fprintf(cmd.OutOrStdout(), "pulling %s:latest\n", defaultImage)
	return runDocker("pull", defaultImage+":latest")
}
