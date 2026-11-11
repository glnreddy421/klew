package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"2.0.0", "2.0.0", 0},
		{"v2.0.1", "2.0.0", 1},
		{"2.0.0", "v2.1.0", -1},
		{"2.0.0-rc.1", "2.0.0", 0}, // suffix ignored for numeric core
		{"1.9.9", "2.0.0", -1},
		{"", "0.0.1", -1},
	}
	for _, tc := range cases {
		if got := Compare(tc.a, tc.b); got != tc.want {
			t.Fatalf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestPickDownloadURL(t *testing.T) {
	rel := githubRelease{
		Assets: []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		}{
			{Name: "Klew-2.1.0-macos-arm64.zip", BrowserDownloadURL: "https://example/arm64.zip"},
			{Name: "Klew-2.1.0-macos-amd64.zip", BrowserDownloadURL: "https://example/amd64.zip"},
		},
	}
	if got := pickDownloadURL(rel, "darwin", "arm64"); got != "https://example/arm64.zip" {
		t.Fatalf("arm64: got %q", got)
	}
	if got := pickDownloadURL(rel, "darwin", "amd64"); got != "https://example/amd64.zip" {
		t.Fatalf("amd64: got %q", got)
	}
	if got := pickDownloadURL(rel, "linux", "amd64"); got != "" {
		t.Fatalf("linux: got %q", got)
	}
}

func TestCheck(t *testing.T) {
	payload, err := json.Marshal(map[string]any{
		"tag_name": "v2.1.0",
		"html_url": "https://github.com/glnreddy421/klew/releases/tag/v2.1.0",
		"assets": []map[string]string{
			{
				"name":                 "Klew-2.1.0-macos-arm64.zip",
				"browser_download_url": "https://example/Klew-2.1.0-macos-arm64.zip",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/glnreddy421/klew/releases/latest" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("missing User-Agent")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)

	prev := latestAPIURL
	latestAPIURL = srv.URL + "/repos/glnreddy421/klew/releases/latest"
	t.Cleanup(func() { latestAPIURL = prev })

	res := Check(context.Background(), "2.0.0", "darwin", "arm64")
	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
	if !res.UpdateAvailable {
		t.Fatal("expected update available")
	}
	if res.LatestVersion != "2.1.0" {
		t.Fatalf("latest: %q", res.LatestVersion)
	}
	if res.DownloadURL != "https://example/Klew-2.1.0-macos-arm64.zip" {
		t.Fatalf("download: %q", res.DownloadURL)
	}

	current := Check(context.Background(), "2.1.0", "darwin", "arm64")
	if current.UpdateAvailable {
		t.Fatal("expected up to date")
	}
}

func TestNormalizeTag(t *testing.T) {
	if got := normalizeTag(" v1.2.3 "); got != "1.2.3" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeTag(""); got != "0.0.0" {
		t.Fatalf("got %q", got)
	}
}
