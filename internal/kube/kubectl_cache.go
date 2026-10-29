package kube

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var clusterKubectlMu sync.Mutex

// kubectlReleaseURL is the kubectl download base; every binary has a published
// "<binary>.sha256" next to it.
const kubectlReleaseURL = "https://dl.k8s.io/release"

// verifiedSuffix names the marker written beside a cached kubectl after its
// SHA-256 matched the published checksum. A cached binary without it (from
// releases before verification) is downloaded and verified again.
const verifiedSuffix = ".sha256"

func klewDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(home, "AppData", "Roaming", "Klew")
	default:
		return filepath.Join(home, "Library", "Application Support", "Klew")
	}
}

func kubectlPlatform() (goos, goarch string) { //nolint:unparam // runtime.GOOS is constant per build, so the linter sees a single value
	goos = runtime.GOOS
	goarch = runtime.GOARCH
	if goos == "darwin" && goarch == "arm64" {
		return goos, goarch
	}
	if goos == "darwin" && goarch == "amd64" {
		return goos, goarch
	}
	return goos, goarch
}

func cachedClusterKubectlPath(releaseVersion string) string {
	if releaseVersion == "" {
		return ""
	}
	base := klewDataDir()
	if base == "" {
		return ""
	}
	goos, goarch := kubectlPlatform()
	return filepath.Join(base, "binaries", releaseVersion, goos, goarch, "kubectl")
}

// EnsureClusterKubectl downloads and caches kubectl for a cluster release when missing.
func EnsureClusterKubectl(ctx context.Context, gitVersion string) (string, error) {
	release := normalizeKubectlReleaseVersion(gitVersion)
	if release == "" {
		return "", fmt.Errorf("unsupported cluster version %q", gitVersion)
	}
	dest := cachedClusterKubectlPath(release)
	if dest == "" {
		return "", fmt.Errorf("klew data directory unavailable")
	}
	if cachedKubectlVerified(dest) {
		return dest, nil
	}

	clusterKubectlMu.Lock()
	defer clusterKubectlMu.Unlock()

	if cachedKubectlVerified(dest) {
		return dest, nil
	}

	goos, goarch := kubectlPlatform()
	if goos != "darwin" {
		return "", fmt.Errorf("cluster kubectl download not supported on %s yet", goos)
	}

	url := fmt.Sprintf("%s/%s/bin/%s/%s/kubectl", kubectlReleaseURL, release, goos, goarch)
	if err := downloadVerifiedKubectl(ctx, url, dest); err != nil {
		return "", err
	}
	_ = os.WriteFile(filepath.Join(filepath.Dir(dest), "version.txt"), []byte(release), 0o600)
	return dest, nil
}

func cachedKubectlVerified(dest string) bool {
	st, err := os.Stat(dest)
	if err != nil || st.IsDir() || st.Size() == 0 {
		return false
	}
	_, err = os.Stat(dest + verifiedSuffix)
	return err == nil
}

// downloadVerifiedKubectl downloads url to dest only if its SHA-256 matches the
// published url+".sha256". It fails closed: a missing or malformed checksum,
// or a mismatch, leaves nothing at dest.
func downloadVerifiedKubectl(ctx context.Context, url, dest string) error {
	want, err := fetchSHA256(ctx, url+".sha256")
	if err != nil {
		return fmt.Errorf("kubectl checksum: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return err
	}
	tmp := dest + ".download"
	got, err := downloadFile(ctx, url, tmp)
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if got != want {
		_ = os.Remove(tmp)
		return fmt.Errorf("kubectl checksum mismatch for %s: got %s, want %s", url, got, want)
	}
	if err := os.Chmod(tmp, 0o700); err != nil { //nolint:gosec // G302: kubectl binary needs the owner execute bit
		_ = os.Remove(tmp)
		return err
	}
	_ = os.Remove(dest + verifiedSuffix)
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.WriteFile(dest+verifiedSuffix, []byte(want+"\n"), 0o600)
}

// fetchSHA256 reads a published checksum file: a hex SHA-256, optionally
// followed by a file name.
func fetchSHA256(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := (&http.Client{Timeout: time.Minute}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(body))
	if len(fields) == 0 {
		return "", fmt.Errorf("empty checksum at %s", url)
	}
	sum := strings.ToLower(fields[0])
	if b, err := hex.DecodeString(sum); err != nil || len(b) != sha256.Size {
		return "", fmt.Errorf("malformed checksum at %s", url)
	}
	return sum, nil
}

// downloadFile writes url to dest and returns the hex SHA-256 of what it wrote.
func downloadFile(ctx context.Context, url, dest string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download kubectl: HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// DataDir is Klew's per-user application data directory ("" when unknown).
func DataDir() string {
	return klewDataDir()
}
