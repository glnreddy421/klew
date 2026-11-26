package update

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// A self-update replaces the running Klew.app with the latest release:
//
//  1. Prepare downloads the release's macOS zip and its SHA256SUMS file into
//     Klew's cache folder, checks the zip's SHA-256, unpacks it, and checks the
//     new app: a valid code signature from the same Apple team as the running
//     app, Gatekeeper's notarization check, and the expected version. Nothing
//     is replaced yet.
//  2. Install starts a small helper that waits for Klew to quit, moves the old
//     app aside, moves the new one into its place and opens it. If the move
//     fails it puts the old app back and opens that instead.
//
// The team check means a release can only update Klew if it was signed with
// the same Developer ID as the copy already installed: a GitHub account or a
// download alone is not enough to ship an update.

const (
	appBundleName = "Klew.app"
	// maxDownloadBytes caps the zip download (the app is about 60 MB).
	maxDownloadBytes = 500 << 20
	maxSumsBytes     = 1 << 20
)

// Staged is a downloaded, verified update ready to install.
type Staged struct {
	Version string `json:"version"`
	// App is the verified Klew.app in the staging folder.
	App string `json:"app"`
	// Target is the installed Klew.app it replaces.
	Target string `json:"target"`
	dir    string
}

// Hooks for tests: the macOS tools that check signatures and read versions.
var (
	selfUpdateTarget = currentAppBundle
	teamIDOf         = codesignTeamID
	verifySignature  = codesignVerify
	assessGatekeeper = spctlAssess
	bundleVersionOf  = plistShortVersion
	unzipApp         = dittoExtract
	stagingRoot      = defaultStagingRoot
	httpClient       = &http.Client{Timeout: 10 * time.Minute}
)

// currentAppBundle returns the Klew.app this process runs from, or why it
// cannot update itself in place.
func currentAppBundle() (string, error) {
	if runtime.GOOS != "darwin" {
		return "", errors.New("self-update is only available in the macOS app")
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot find the running app: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return appBundleFromExecutable(exe)
}

// appBundleFromExecutable maps .../Klew.app/Contents/MacOS/Klew to
// .../Klew.app and checks that the folder holding it can take a new copy.
func appBundleFromExecutable(exe string) (string, error) {
	macos := filepath.Dir(exe)
	contents := filepath.Dir(macos)
	app := filepath.Dir(contents)
	if filepath.Base(macos) != "MacOS" || filepath.Base(contents) != "Contents" || !strings.HasSuffix(app, ".app") {
		return "", errors.New("not running from an app bundle")
	}
	if strings.Contains(app, "/AppTranslocation/") {
		return "", errors.New("macOS is running Klew from a temporary location; move Klew to Applications to update it in place")
	}
	if strings.HasPrefix(app, "/Volumes/") {
		return "", errors.New("running from a disk image; copy Klew to Applications to update it in place")
	}
	if err := checkWritable(filepath.Dir(app)); err != nil {
		return "", fmt.Errorf("cannot replace Klew in %s: %w", filepath.Dir(app), err)
	}
	return app, nil
}

func checkWritable(dir string) error {
	f, err := os.CreateTemp(dir, ".klew-update-check-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name)
}

func defaultStagingRoot() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "Klew", "updates"), nil
}

// Prepare downloads and verifies the update described by res. It returns an
// error, and leaves the installed app untouched, if any check fails.
func Prepare(ctx context.Context, res Result) (Staged, error) {
	if !res.UpdateAvailable {
		return Staged{}, errors.New("no update is available")
	}
	if res.DownloadURL == "" || res.ChecksumsURL == "" {
		return Staged{}, errors.New("this release has no verified download for this Mac")
	}
	target, err := selfUpdateTarget()
	if err != nil {
		return Staged{}, err
	}
	wantTeam, err := teamIDOf(target)
	if err != nil {
		return Staged{}, fmt.Errorf("this copy of Klew is not signed, so it cannot verify updates: %w", err)
	}

	root, err := stagingRoot()
	if err != nil {
		return Staged{}, err
	}
	// Start clean: a half-finished earlier attempt is never reused.
	_ = os.RemoveAll(root)
	dir := filepath.Join(root, res.LatestVersion)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Staged{}, err
	}
	fail := func(err error) (Staged, error) {
		_ = os.RemoveAll(dir)
		return Staged{}, err
	}

	zipName := path.Base(res.DownloadURL)
	sums, err := fetchSums(ctx, res.ChecksumsURL)
	if err != nil {
		return fail(fmt.Errorf("download checksums: %w", err))
	}
	want, ok := sums[zipName]
	if !ok {
		return fail(fmt.Errorf("the release's checksums do not list %s", zipName))
	}
	zipPath := filepath.Join(dir, zipName)
	got, err := download(ctx, res.DownloadURL, zipPath)
	if err != nil {
		return fail(fmt.Errorf("download %s: %w", zipName, err))
	}
	if !strings.EqualFold(got, want) {
		return fail(fmt.Errorf("%s does not match the release checksum", zipName))
	}

	unpacked := filepath.Join(dir, "unpacked")
	if err := unzipApp(zipPath, unpacked); err != nil {
		return fail(fmt.Errorf("unpack %s: %w", zipName, err))
	}
	_ = os.Remove(zipPath)
	app := filepath.Join(unpacked, appBundleName)
	if fi, err := os.Stat(app); err != nil || !fi.IsDir() {
		return fail(fmt.Errorf("%s does not contain %s", zipName, appBundleName))
	}

	if err := verifySignature(app); err != nil {
		return fail(fmt.Errorf("the new app's signature is not valid: %w", err))
	}
	team, err := teamIDOf(app)
	if err != nil {
		return fail(fmt.Errorf("the new app is not signed: %w", err))
	}
	if team != wantTeam {
		return fail(fmt.Errorf("the new app is signed by team %s, not %s like this copy", team, wantTeam))
	}
	if err := assessGatekeeper(app); err != nil {
		return fail(fmt.Errorf("macOS Gatekeeper rejects the new app: %w", err))
	}
	version, err := bundleVersionOf(app)
	if err != nil {
		return fail(fmt.Errorf("read the new app's version: %w", err))
	}
	if Compare(version, res.LatestVersion) != 0 {
		return fail(fmt.Errorf("the new app is version %s, expected %s", version, res.LatestVersion))
	}
	if Compare(version, res.CurrentVersion) <= 0 {
		return fail(fmt.Errorf("the new app (%s) is not newer than this one (%s)", version, res.CurrentVersion))
	}
	return Staged{Version: version, App: app, Target: target, dir: dir}, nil
}

// fetchSums reads a shasum file ("<hex>  <name>" per line) into name → hash.
func fetchSums(ctx context.Context, url string) (map[string]string, error) {
	body, err := get(ctx, url)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	return parseSums(io.LimitReader(body, maxSumsBytes))
}

func parseSums(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) != 2 {
			continue
		}
		hash, name := fields[0], strings.TrimPrefix(fields[1], "*")
		if len(hash) != sha256.Size*2 {
			continue
		}
		if _, err := hex.DecodeString(hash); err != nil {
			continue
		}
		out[path.Base(name)] = strings.ToLower(hash)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New("no checksums found")
	}
	return out, nil
}

// download saves url to dest and returns its SHA-256.
func download(ctx context.Context, url, dest string) (string, error) {
	body, err := get(ctx, url)
	if err != nil {
		return "", err
	}
	defer body.Close()
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(body, maxDownloadBytes+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	if n > maxDownloadBytes {
		return "", errors.New("download is larger than expected")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func get(ctx context.Context, url string) (io.ReadCloser, error) {
	if !strings.HasPrefix(url, "https://") && !allowHTTP {
		return nil, fmt.Errorf("refusing a non-HTTPS URL: %s", url)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Klew")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return resp.Body, nil
}

// allowHTTP lets tests serve downloads from httptest.
var allowHTTP = false

// Install starts the helper that swaps the app once this process (pid) has
// exited, then returns; the caller must quit the app right after.
func Install(s Staged, pid int) error {
	if s.App == "" || s.Target == "" || s.dir == "" {
		return errors.New("no update has been prepared")
	}
	script := filepath.Join(s.dir, "install.sh")
	// Run through /bin/sh below, so it needs no execute bit.
	if err := os.WriteFile(script, []byte(installScript), 0o600); err != nil {
		return err
	}
	backup := filepath.Join(s.dir, "previous-"+appBundleName)
	logPath := filepath.Join(s.dir, "install.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer logFile.Close()
	cmd := exec.Command("/bin/sh", script, fmt.Sprint(pid), s.Target, s.App, backup)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	// Its own session, so it outlives Klew quitting.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// installScript waits for Klew (pid $1) to exit, then replaces $2 with $3,
// keeping the old app at $4 until the move succeeds.
const installScript = `#!/bin/sh
set -u
pid="$1"; target="$2"; new="$3"; backup="$4"
i=0
while kill -0 "$pid" 2>/dev/null; do
  i=$((i + 1))
  if [ "$i" -gt 600 ]; then echo "Klew did not quit; update not installed"; exit 1; fi
  sleep 0.1
done
rm -rf "$backup"
if ! mv "$target" "$backup"; then
  echo "could not move the old app aside; update not installed"
  open "$target"
  exit 1
fi
if ! mv "$new" "$target"; then
  echo "could not move the new app into place; restoring the old one"
  mv "$backup" "$target"
  open "$target"
  exit 1
fi
open "$target"
rm -rf "$backup"
echo "updated"
`

// ---- macOS tools ----------------------------------------------------------

func codesignTeamID(app string) (string, error) {
	// codesign -dv prints its details on stderr.
	out, err := exec.Command("codesign", "-dv", "--verbose=2", app).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return parseTeamID(string(out))
}

func parseTeamID(out string) (string, error) {
	for _, line := range strings.Split(out, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "TeamIdentifier="); ok {
			v = strings.TrimSpace(v)
			if v == "" || v == "not set" {
				return "", errors.New("no team identifier")
			}
			return v, nil
		}
	}
	return "", errors.New("no team identifier")
}

func codesignVerify(app string) error {
	out, err := exec.Command("codesign", "--verify", "--deep", "--strict", app).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func spctlAssess(app string) error {
	out, err := exec.Command("spctl", "--assess", "--type", "execute", app).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func plistShortVersion(app string) (string, error) {
	out, err := exec.Command("plutil", "-extract", "CFBundleShortVersionString", "raw", "-o", "-",
		filepath.Join(app, "Contents", "Info.plist")).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func dittoExtract(zip, dest string) error {
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return err
	}
	out, err := exec.Command("ditto", "-x", "-k", zip, dest).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}
