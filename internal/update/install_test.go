package update

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestAppBundleFromExecutable(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "Klew.app", "Contents", "MacOS", "Klew")
	got, err := appBundleFromExecutable(exe)
	if err != nil || got != filepath.Join(dir, "Klew.app") {
		t.Fatalf("got %q, %v", got, err)
	}

	for name, exe := range map[string]string{
		"not a bundle": filepath.Join(dir, "bin", "klew"),
		"translocated": "/private/var/folders/x/AppTranslocation/ABC/d/Klew.app/Contents/MacOS/Klew",
		"disk image":   "/Volumes/Klew/Klew.app/Contents/MacOS/Klew",
		"not writable": "/nonexistent-klew-dir/Klew.app/Contents/MacOS/Klew",
	} {
		if _, err := appBundleFromExecutable(exe); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestParseSums(t *testing.T) {
	a := strings.Repeat("a", 64)
	b := strings.Repeat("B", 64)
	sums, err := parseSums(strings.NewReader(
		a + "  Klew-2.2.0-macos-arm64.zip\n" +
			b + "  ./klew-cli-2.2.0-sbom.spdx.json\n" +
			"not a line\n" +
			"zz  short.zip\n"))
	if err != nil {
		t.Fatal(err)
	}
	if sums["Klew-2.2.0-macos-arm64.zip"] != a || sums["klew-cli-2.2.0-sbom.spdx.json"] != strings.ToLower(b) {
		t.Fatalf("sums = %v", sums)
	}
	if len(sums) != 2 {
		t.Fatalf("want 2 entries, got %v", sums)
	}
	if _, err := parseSums(strings.NewReader("nothing here\n")); err == nil {
		t.Fatal("want an error for a file with no checksums")
	}
}

func TestParseTeamID(t *testing.T) {
	out := "Executable=/Applications/Klew.app/Contents/MacOS/Klew\nAuthority=Developer ID Application: X (ABCDE12345)\nTeamIdentifier=ABCDE12345\n"
	if got, err := parseTeamID(out); err != nil || got != "ABCDE12345" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := parseTeamID("TeamIdentifier=not set\n"); err == nil {
		t.Fatal("ad-hoc signature: want an error")
	}
	if _, err := parseTeamID("code object is not signed at all\n"); err == nil {
		t.Fatal("unsigned: want an error")
	}
}

// fakeRelease serves a zip holding Klew.app and a SHA256SUMS file listing it.
type fakeRelease struct {
	srv     *httptest.Server
	zipURL  string
	sumsURL string
}

func newFakeRelease(t *testing.T, version string, badSum bool) fakeRelease {
	t.Helper()
	zipName := "Klew-" + version + "-macos-arm64.zip"
	var buf strings.Builder
	zw := zip.NewWriter(&writerAdapter{&buf})
	w, err := zw.Create("Klew.app/Contents/Info.plist")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(w, "version %s", version)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	data := buf.String()
	sum := sha256.Sum256([]byte(data))
	hexSum := hex.EncodeToString(sum[:])
	if badSum {
		hexSum = strings.Repeat("0", 64)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/"+zipName, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(data)) })
	mux.HandleFunc("/sums.txt", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", hexSum, zipName)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return fakeRelease{srv: srv, zipURL: srv.URL + "/" + zipName, sumsURL: srv.URL + "/sums.txt"}
}

type writerAdapter struct{ b *strings.Builder }

func (w *writerAdapter) Write(p []byte) (int, error) { return w.b.Write(p) }

// stubTools replaces the macOS tools: both apps are signed by team, and the
// new app reports version.
func stubTools(t *testing.T, target, newTeam, version string) {
	t.Helper()
	root := t.TempDir()
	saved := []any{selfUpdateTarget, teamIDOf, verifySignature, assessGatekeeper, bundleVersionOf, unzipApp, stagingRoot, allowHTTP}
	t.Cleanup(func() {
		selfUpdateTarget = saved[0].(func() (string, error))
		teamIDOf = saved[1].(func(string) (string, error))
		verifySignature = saved[2].(func(string) error)
		assessGatekeeper = saved[3].(func(string) error)
		bundleVersionOf = saved[4].(func(string) (string, error))
		unzipApp = saved[5].(func(string, string) error)
		stagingRoot = saved[6].(func() (string, error))
		allowHTTP = saved[7].(bool)
	})
	allowHTTP = true
	selfUpdateTarget = func() (string, error) { return target, nil }
	teamIDOf = func(app string) (string, error) {
		if app == target {
			return "TEAM1", nil
		}
		return newTeam, nil
	}
	verifySignature = func(string) error { return nil }
	assessGatekeeper = func(string) error { return nil }
	bundleVersionOf = func(string) (string, error) { return version, nil }
	unzipApp = unzipForTest
	stagingRoot = func() (string, error) { return root, nil }
}

func unzipForTest(zipPath, dest string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		p := filepath.Join(dest, f.Name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}
		if err := os.WriteFile(p, b, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func available(rel fakeRelease, latest string) Result {
	return Result{
		CurrentVersion: "2.1.0", LatestVersion: latest, UpdateAvailable: true,
		DownloadURL: rel.zipURL, ChecksumsURL: rel.sumsURL,
	}
}

func TestPrepareVerifiesTheUpdate(t *testing.T) {
	target := filepath.Join(t.TempDir(), "Klew.app")
	rel := newFakeRelease(t, "2.2.0", false)
	stubTools(t, target, "TEAM1", "2.2.0")

	s, err := Prepare(context.Background(), available(rel, "2.2.0"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Version != "2.2.0" || s.Target != target || filepath.Base(s.App) != "Klew.app" {
		t.Fatalf("staged = %+v", s)
	}
	if _, err := os.Stat(filepath.Join(s.App, "Contents", "Info.plist")); err != nil {
		t.Fatalf("staged app missing: %v", err)
	}
}

func TestPrepareRefusesUnverifiedUpdates(t *testing.T) {
	cases := []struct {
		name    string
		badSum  bool
		team    string
		version string
		setup   func()
		want    string
	}{
		{name: "checksum mismatch", badSum: true, team: "TEAM1", version: "2.2.0", want: "does not match the release checksum"},
		{name: "another team", team: "EVIL9", version: "2.2.0", want: "signed by team EVIL9"},
		{name: "wrong version inside", team: "TEAM1", version: "2.1.5", want: "expected 2.2.0"},
		{name: "bad signature", team: "TEAM1", version: "2.2.0",
			setup: func() { verifySignature = func(string) error { return errors.New("modified") } }, want: "signature is not valid"},
		{name: "not notarized", team: "TEAM1", version: "2.2.0",
			setup: func() { assessGatekeeper = func(string) error { return errors.New("rejected") } }, want: "Gatekeeper rejects"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "Klew.app")
			rel := newFakeRelease(t, "2.2.0", tc.badSum)
			stubTools(t, target, tc.team, tc.version)
			if tc.setup != nil {
				tc.setup()
			}
			root, _ := stagingRoot()
			_, err := Prepare(context.Background(), available(rel, "2.2.0"))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if _, err := os.Stat(filepath.Join(root, "2.2.0")); !os.IsNotExist(err) {
				t.Fatalf("a refused update must leave nothing staged (stat err %v)", err)
			}
		})
	}
}

func TestPrepareNeedsASignedRunningApp(t *testing.T) {
	target := filepath.Join(t.TempDir(), "Klew.app")
	rel := newFakeRelease(t, "2.2.0", false)
	stubTools(t, target, "TEAM1", "2.2.0")
	teamIDOf = func(string) (string, error) { return "", errors.New("not signed") }
	if _, err := Prepare(context.Background(), available(rel, "2.2.0")); err == nil || !strings.Contains(err.Error(), "cannot verify updates") {
		t.Fatalf("err = %v", err)
	}
}

func TestPrepareRefusesPlainHTTP(t *testing.T) {
	target := filepath.Join(t.TempDir(), "Klew.app")
	rel := newFakeRelease(t, "2.2.0", false)
	stubTools(t, target, "TEAM1", "2.2.0")
	allowHTTP = false
	if _, err := Prepare(context.Background(), available(rel, "2.2.0")); err == nil || !strings.Contains(err.Error(), "non-HTTPS") {
		t.Fatalf("err = %v", err)
	}
}

// The helper script swaps the bundles once the given process has exited.
func TestInstallScriptSwapsTheApp(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs /bin/sh")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "Applications", "Klew.app")
	newApp := filepath.Join(dir, "stage", "Klew.app")
	for app, v := range map[string]string{target: "old", newApp: "new"} {
		if err := os.MkdirAll(app, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(app, "v"), []byte(v), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	script := filepath.Join(dir, "install.sh")
	// "open" would launch the app; record the call instead.
	bin := filepath.Join(dir, "bin")
	_ = os.MkdirAll(bin, 0o700)
	if err := os.WriteFile(filepath.Join(bin, "open"), []byte("#!/bin/sh\necho \"$1\" > \""+dir+"/opened\"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(script, []byte(installScript), 0o700); err != nil {
		t.Fatal(err)
	}

	sleeper := exec.Command("sleep", "0.3")
	if err := sleeper.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { _ = sleeper.Wait() }()

	cmd := exec.Command("/bin/sh", script, fmt.Sprint(sleeper.Process.Pid), target, newApp, filepath.Join(dir, "stage", "previous-Klew.app"))
	cmd.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
	start := time.Now()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v: %s", err, out)
	}
	if time.Since(start) < 200*time.Millisecond {
		t.Fatal("the script did not wait for the process to exit")
	}
	if b, _ := os.ReadFile(filepath.Join(target, "v")); string(b) != "new" {
		t.Fatalf("target holds %q, want the new app", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "opened")); strings.TrimSpace(string(b)) != target {
		t.Fatalf("opened %q, want %s", b, target)
	}
	if _, err := os.Stat(filepath.Join(dir, "stage", "previous-Klew.app")); !os.IsNotExist(err) {
		t.Fatal("the old app should be removed after a successful swap")
	}
}
