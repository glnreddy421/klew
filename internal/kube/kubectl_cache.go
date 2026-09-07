package kube

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

var clusterKubectlMu sync.Mutex

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

func kubectlPlatform() (goos, goarch string) {
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
	if st, err := os.Stat(dest); err == nil && !st.IsDir() && st.Size() > 0 {
		return dest, nil
	}

	clusterKubectlMu.Lock()
	defer clusterKubectlMu.Unlock()

	if st, err := os.Stat(dest); err == nil && !st.IsDir() && st.Size() > 0 {
		return dest, nil
	}

	goos, goarch := kubectlPlatform()
	if goos != "darwin" {
		return "", fmt.Errorf("cluster kubectl download not supported on %s yet", goos)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://dl.k8s.io/release/%s/bin/%s/%s/kubectl", release, goos, goarch)
	tmp := dest + ".download"
	if err := downloadFile(ctx, url, tmp); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	_ = os.WriteFile(filepath.Join(filepath.Dir(dest), "version.txt"), []byte(release), 0o644)
	return dest, nil
}

func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download kubectl: HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}
