// Package update checks GitHub releases for newer versions of the CLI and
// performs in-place binary upgrades.
package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	repo         = "itapulab/itapu-cli"
	ReleasesURL  = "https://github.com/" + repo + "/releases"
	maxBinaryMiB = 64
)

// LatestTag resolves the newest release tag (e.g. "v0.2.0") from the
// /releases/latest redirect — unlike the GitHub API, it is not rate-limited
// for anonymous clients (same trick as install.sh).
func LatestTag(client *http.Client) (string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodHead, ReleasesURL+"/latest", nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	tag := path.Base(resp.Request.URL.Path)
	if _, ok := parseVersion(tag); !ok {
		return "", errors.New("could not determine the latest release")
	}
	return tag, nil
}

// parseVersion parses "v1.2.3" or "1.2.3" (a pre-release suffix like "-rc1"
// is ignored) into numeric parts. ok is false for anything else — dev
// builds, git describe output, empty strings.
func parseVersion(s string) (parts [3]int, ok bool) {
	s = strings.TrimPrefix(s, "v")
	s, _, _ = strings.Cut(s, "-")
	fields := strings.Split(s, ".")
	if len(fields) == 0 || len(fields) > 3 {
		return parts, false
	}
	for i, f := range fields {
		n, err := strconv.Atoi(f)
		if err != nil || n < 0 {
			return parts, false
		}
		parts[i] = n
	}
	return parts, true
}

// DisplayVersion renders a stamped version for messages: "v0.3.0" for
// release versions, the raw string ("dev", git describe output) otherwise.
func DisplayVersion(v string) string {
	if _, ok := parseVersion(v); ok {
		return "v" + strings.TrimPrefix(v, "v")
	}
	return v
}

// Newer reports whether tag is a release newer than current. comparable is
// false when either side doesn't parse as a version (e.g. dev builds).
func Newer(tag, current string) (newer, comparable bool) {
	a, okA := parseVersion(tag)
	b, okB := parseVersion(current)
	if !okA || !okB {
		return false, false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] > b[i], true
		}
	}
	return false, true
}

// Apply downloads the release archive for tag, verifies it against the
// published checksums and atomically replaces the current executable.
func Apply(tag string) error {
	if runtime.GOOS == "windows" {
		return errors.New("automatic update is not supported on Windows — download the latest from " + ReleasesURL)
	}
	version := strings.TrimPrefix(tag, "v")
	name := fmt.Sprintf("itapu_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
	base := ReleasesURL + "/download/" + tag + "/"

	archive, err := download(base + name)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", name, err)
	}
	sums, err := download(base + "checksums.txt")
	if err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}
	if err := verifyChecksum(name, archive, sums); err != nil {
		return err
	}
	binary, err := extractBinary(archive)
	if err != nil {
		return fmt.Errorf("extracting %s: %w", name, err)
	}
	return replaceExecutable(binary)
}

func download(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBinaryMiB<<20))
}

// verifyChecksum checks data against the goreleaser checksums.txt format:
// "<sha256-hex>  <filename>" per line.
func verifyChecksum(name string, data, sums []byte) error {
	got := hex.EncodeToString(func() []byte { s := sha256.Sum256(data); return s[:] }())
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == name {
			if !strings.EqualFold(fields[0], got) {
				return fmt.Errorf("checksum mismatch for %s — aborting update", name)
			}
			return nil
		}
	}
	return fmt.Errorf("no checksum published for %s — aborting update", name)
}

// extractBinary returns the `itapu` file from the release tar.gz (the
// archives place it at the root, see .goreleaser.yaml).
func extractBinary(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, errors.New("archive does not contain the itapu binary")
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag == tar.TypeReg && path.Base(hdr.Name) == "itapu" {
			return io.ReadAll(io.LimitReader(tr, maxBinaryMiB<<20))
		}
	}
}

// replaceExecutable writes the new binary next to the current one and
// renames it into place, so the swap is atomic and a failed download never
// leaves a broken install.
func replaceExecutable(binary []byte) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	tmp := filepath.Join(filepath.Dir(exe), fmt.Sprintf(".itapu.new-%d", os.Getpid()))
	if err := os.WriteFile(tmp, binary, 0o755); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("no write permission for %s — re-run with sudo, or reinstall to a user-writable dir with install.sh", filepath.Dir(exe))
		}
		return err
	}
	if err := os.Rename(tmp, exe); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
