package kube

import (
	"regexp"
	"strconv"
	"strings"
)

var k8sVersionRe = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`)

type k8sSemver struct {
	major int
	minor int
	patch int
	raw   string
}

func parseK8sSemver(gitVersion string) (k8sSemver, bool) {
	gitVersion = strings.TrimSpace(gitVersion)
	if gitVersion == "" {
		return k8sSemver{}, false
	}
	m := k8sVersionRe.FindStringSubmatch(gitVersion)
	if len(m) != 4 {
		return k8sSemver{}, false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	return k8sSemver{
		major: major,
		minor: minor,
		patch: patch,
		raw:   "v" + m[1] + "." + m[2] + "." + m[3],
	}, true
}

// versionSkewRequiresDownload reports whether clusterVersion is outside the bundled
// kubectl skew window (+/- one minor version per Kubernetes compatibility rules).
func versionSkewRequiresDownload(bundledVersion, clusterVersion string) bool {
	bundled, okB := parseK8sSemver(bundledVersion)
	cluster, okC := parseK8sSemver(clusterVersion)
	if !okB || !okC {
		return false
	}
	if bundled.major != cluster.major {
		return true
	}
	diff := bundled.minor - cluster.minor
	if diff < 0 {
		diff = -diff
	}
	return diff > 1
}

func normalizeKubectlReleaseVersion(gitVersion string) string {
	if v, ok := parseK8sSemver(gitVersion); ok {
		return v.raw
	}
	v := strings.TrimSpace(gitVersion)
	if strings.HasPrefix(v, "v") {
		return v
	}
	return ""
}
