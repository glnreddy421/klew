package kube

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// KubectlInfo describes which kubectl binary Klew uses.
type KubectlInfo struct {
	ActivePath            string `json:"activePath"`
	BundledPath           string `json:"bundledPath"`
	SystemPath            string `json:"systemPath"`
	ClusterMatchedPath    string `json:"clusterMatchedPath,omitempty"`
	Source                string `json:"source"` // bundled, cluster-matched, system, custom
	Version               string `json:"version,omitempty"`
	ClusterVersion        string `json:"clusterVersion,omitempty"`
	MatchClusterKubectl   bool   `json:"matchClusterKubectl"`
	ClusterSkewDetected   bool   `json:"clusterSkewDetected"`
	Available             bool   `json:"available"`
}

var kubectlConfig struct {
	sync.RWMutex
	useBundled          bool
	customPath          string
	matchClusterKubectl bool
}

func init() {
	kubectlConfig.useBundled = true
	kubectlConfig.matchClusterKubectl = true
}

// SetKubectlOptions configures kubectl resolution for subprocess use.
func SetKubectlOptions(useBundled bool, customPath string, matchClusterKubectl bool) {
	kubectlConfig.Lock()
	defer kubectlConfig.Unlock()
	kubectlConfig.useBundled = useBundled
	kubectlConfig.customPath = strings.TrimSpace(customPath)
	kubectlConfig.matchClusterKubectl = matchClusterKubectl
}

func bundledKubectlVersionLabel() string {
	if v := bundledKubectlVersion(); v != "" {
		return v
	}
	path := BundledKubectlPath()
	if path == "" {
		return ""
	}
	return kubectlClientVersion(path)
}

// ResolveKubectl returns kubectl for generic use (no cluster version hint).
func ResolveKubectl() (path, source string) {
	return resolveKubectl("", context.Background())
}

// ResolveKubectlForCluster picks kubectl for a target cluster, downloading a
// version-matched binary when skew exceeds +/- one minor version.
func ResolveKubectlForCluster(clusterVersion string) (path, source string) {
	return resolveKubectl(clusterVersion, context.Background())
}

func resolveKubectl(clusterVersion string, ctx context.Context) (path, source string) {
	kubectlConfig.RLock()
	useBundled := kubectlConfig.useBundled
	custom := kubectlConfig.customPath
	matchCluster := kubectlConfig.matchClusterKubectl
	kubectlConfig.RUnlock()

	if custom != "" {
		if st, err := os.Stat(custom); err == nil && !st.IsDir() {
			return custom, "custom"
		}
	}

	bundled := BundledKubectlPath()
	bundledVersion := bundledKubectlVersionLabel()

	if useBundled && matchCluster && clusterVersion != "" && bundledVersion != "" &&
		versionSkewRequiresDownload(bundledVersion, clusterVersion) {
		if matched, err := EnsureClusterKubectl(ctx, clusterVersion); err == nil && matched != "" {
			return matched, "cluster-matched"
		}
	}

	if useBundled && bundled != "" {
		return bundled, "bundled"
	}
	if system := systemKubectlPath(); system != "" {
		return system, "system"
	}
	return "", ""
}

// BundledKubectlPath returns the kubectl binary inside Klew.app/Contents/Resources, if present.
func BundledKubectlPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return ""
	}
	candidate := filepath.Join(filepath.Dir(exe), "..", "Resources", "kubectl", "kubectl")
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return ""
	}
	if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
		return candidate
	}
	return ""
}

func bundledKubectlVersion() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return ""
	}
	versionFile := filepath.Join(filepath.Dir(exe), "..", "Resources", "kubectl", "version.txt")
	versionFile, err = filepath.Abs(versionFile)
	if err != nil {
		return ""
	}
	raw, err := os.ReadFile(versionFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func systemKubectlPath() string {
	path, err := exec.LookPath("kubectl")
	if err != nil {
		return ""
	}
	return path
}

// KubectlDir returns the directory containing the active kubectl binary, if any.
func KubectlDir() string {
	path, _ := ResolveKubectl()
	return kubectlDir(path)
}

// KubectlDirForCluster returns the kubectl directory for a specific cluster version.
func KubectlDirForCluster(clusterVersion string) string {
	path, _ := ResolveKubectlForCluster(clusterVersion)
	return kubectlDir(path)
}

func kubectlDir(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Dir(path)
}

// GetKubectlInfo returns display metadata for Settings.
func GetKubectlInfo(clusterVersion string) KubectlInfo {
	kubectlConfig.RLock()
	matchCluster := kubectlConfig.matchClusterKubectl
	kubectlConfig.RUnlock()

	bundledVersion := bundledKubectlVersionLabel()
	active, source := resolveKubectl(clusterVersion, context.Background())
	clusterMatched := ""
	if clusterVersion != "" && bundledVersion != "" &&
		versionSkewRequiresDownload(bundledVersion, clusterVersion) {
		clusterMatched = cachedClusterKubectlPath(normalizeKubectlReleaseVersion(clusterVersion))
	}

	info := KubectlInfo{
		ActivePath:          active,
		BundledPath:         BundledKubectlPath(),
		SystemPath:          systemKubectlPath(),
		ClusterMatchedPath:  clusterMatched,
		Source:              source,
		ClusterVersion:      clusterVersion,
		MatchClusterKubectl: matchCluster,
		ClusterSkewDetected: clusterVersion != "" && bundledVersion != "" &&
			versionSkewRequiresDownload(bundledVersion, clusterVersion),
		Available: active != "",
	}
	if active != "" {
		info.Version = kubectlClientVersion(active)
	}
	return info
}

func kubectlClientVersion(path string) string {
	if path == "" {
		return ""
	}
	out, err := exec.Command(path, "version", "--client", "-o", "yaml").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "gitVersion:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "gitVersion:"))
		}
	}
	return ""
}
