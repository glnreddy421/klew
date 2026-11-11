// Package update checks GitHub Releases for a newer Klew desktop build.
// It never downloads or installs; callers decide how to prompt the user.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	githubOwner = "glnreddy421"
	githubRepo  = "klew"
	releasesURL = "https://github.com/" + githubOwner + "/" + githubRepo + "/releases"

	ChannelHomebrew = "homebrew"
	ChannelDirect   = "direct"
	ChannelUnknown  = "unknown"

	BrewUpgradeCommand = "brew update && brew upgrade --cask klew"
)

// latestAPIURL is the GitHub Releases latest endpoint (overridable in tests).
var latestAPIURL = "https://api.github.com/repos/" + githubOwner + "/" + githubRepo + "/releases/latest"

// Result is the outcome of CheckForUpdate.
type Result struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	Channel         string `json:"channel"`
	UpgradeCommand  string `json:"upgradeCommand,omitempty"`
	ReleaseURL      string `json:"releaseURL"`
	DownloadURL     string `json:"downloadURL,omitempty"`
	Message         string `json:"message"`
	Error           string `json:"error,omitempty"`
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// Check fetches the latest GitHub release and compares it to currentVersion.
// goos/goarch select a matching download asset when present; pass empty to use runtime.
func Check(ctx context.Context, currentVersion, goos, goarch string) Result {
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	channel := DetectChannel()
	res := Result{
		CurrentVersion: normalizeTag(currentVersion),
		Channel:        channel,
		ReleaseURL:     releasesURL,
	}
	if channel == ChannelHomebrew {
		res.UpgradeCommand = BrewUpgradeCommand
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	rel, err := fetchLatest(ctx, res.CurrentVersion)
	if err != nil {
		res.Error = err.Error()
		res.Message = "Could not check for updates."
		return res
	}

	latest := normalizeTag(rel.TagName)
	res.LatestVersion = latest
	if rel.HTMLURL != "" {
		res.ReleaseURL = rel.HTMLURL
	}
	res.DownloadURL = pickDownloadURL(rel, goos, goarch)
	res.UpdateAvailable = Compare(latest, res.CurrentVersion) > 0

	switch {
	case res.UpdateAvailable && channel == ChannelHomebrew:
		res.Message = fmt.Sprintf("Klew %s is available (you have %s). Upgrade with Homebrew.", latest, res.CurrentVersion)
	case res.UpdateAvailable:
		res.Message = fmt.Sprintf("Klew %s is available (you have %s).", latest, res.CurrentVersion)
	default:
		res.Message = fmt.Sprintf("You're up to date (Klew %s).", res.CurrentVersion)
	}
	return res
}

func fetchLatest(ctx context.Context, currentVersion string) (githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, latestAPIURL, nil)
	if err != nil {
		return githubRelease{}, err
	}
	ua := "Klew"
	if currentVersion != "" && currentVersion != "0.0.0" {
		ua = "Klew/" + currentVersion
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return githubRelease{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return githubRelease{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return githubRelease{}, fmt.Errorf("GitHub releases: HTTP %d", resp.StatusCode)
	}
	var rel githubRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return githubRelease{}, err
	}
	if strings.TrimSpace(rel.TagName) == "" {
		return githubRelease{}, fmt.Errorf("GitHub releases: empty tag")
	}
	return rel, nil
}

func pickDownloadURL(rel githubRelease, goos, goarch string) string {
	want := assetNameHint(goos, goarch)
	if want == "" {
		return ""
	}
	for _, a := range rel.Assets {
		name := strings.ToLower(a.Name)
		if strings.Contains(name, want) && strings.HasSuffix(name, ".zip") {
			return a.BrowserDownloadURL
		}
	}
	return ""
}

func assetNameHint(goos, goarch string) string {
	if goos != "darwin" {
		return ""
	}
	switch goarch {
	case "arm64":
		return "macos-arm64"
	case "amd64":
		return "macos-amd64"
	default:
		return ""
	}
}

func normalizeTag(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if v == "" {
		return "0.0.0"
	}
	return v
}

// Compare returns -1 if a<b, 0 if equal, 1 if a>b for dotted numeric versions.
// Pre-release / build suffixes are ignored after the first hyphen or plus.
func Compare(a, b string) int {
	ap := versionParts(normalizeTag(a))
	bp := versionParts(normalizeTag(b))
	n := len(ap)
	if len(bp) > n {
		n = len(bp)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(ap) {
			av = ap[i]
		}
		if i < len(bp) {
			bv = bp[i]
		}
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func versionParts(v string) []int {
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	segs := strings.Split(v, ".")
	out := make([]int, 0, len(segs))
	for _, s := range segs {
		n, err := strconv.Atoi(s)
		if err != nil {
			n = 0
		}
		out = append(out, n)
	}
	return out
}
