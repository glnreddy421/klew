package kube

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func kubectlServer(t *testing.T, binary, checksum string, checksumStatus int) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/kubectl":
			_, _ = w.Write([]byte(binary))
		case "/kubectl.sha256":
			w.WriteHeader(checksumStatus)
			_, _ = w.Write([]byte(checksum))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL + "/kubectl"
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func TestDownloadVerifiedKubectlAcceptsMatchingChecksum(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "v1.31.4", "kubectl")
	// dl.k8s.io publishes the bare digest; "<digest>  <name>" must work too.
	url := kubectlServer(t, "kubectl-binary", strings.ToUpper(sha256Hex("kubectl-binary"))+"  kubectl\n", http.StatusOK)

	if err := downloadVerifiedKubectl(context.Background(), url, dest); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(dest); err != nil || string(b) != "kubectl-binary" {
		t.Fatalf("dest = %q, %v", b, err)
	}
	if st, _ := os.Stat(dest); st.Mode().Perm()&0o100 == 0 {
		t.Fatalf("kubectl is not executable: %v", st.Mode())
	}
	if !cachedKubectlVerified(dest) {
		t.Fatal("verified marker missing after a matching download")
	}
}

func TestDownloadVerifiedKubectlFailsClosed(t *testing.T) {
	cases := map[string]struct {
		checksum string
		status   int
		wantErr  string
	}{
		"mismatch":         {sha256Hex("something else"), http.StatusOK, "checksum mismatch"},
		"checksum missing": {"", http.StatusNotFound, "HTTP 404"},
		"malformed":        {"not-a-digest", http.StatusOK, "malformed checksum"},
		"empty":            {"", http.StatusOK, "empty checksum"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "kubectl")
			url := kubectlServer(t, "tampered-binary", tc.checksum, tc.status)

			err := downloadVerifiedKubectl(context.Background(), url, dest)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
			for _, p := range []string{dest, dest + ".download", dest + verifiedSuffix} {
				if _, err := os.Stat(p); !os.IsNotExist(err) {
					t.Errorf("%s left behind after a failed verification", filepath.Base(p))
				}
			}
		})
	}
}

func TestCachedKubectlWithoutMarkerIsNotTrusted(t *testing.T) {
	// Binaries cached before verification existed have no marker and must be
	// downloaded and verified again.
	dest := filepath.Join(t.TempDir(), "kubectl")
	if err := os.WriteFile(dest, []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if cachedKubectlVerified(dest) {
		t.Fatal("unverified cached kubectl was trusted")
	}
	if err := os.WriteFile(dest+verifiedSuffix, []byte(sha256Hex("legacy")), 0o600); err != nil {
		t.Fatal(err)
	}
	if !cachedKubectlVerified(dest) {
		t.Fatal("verified cached kubectl was not trusted")
	}
}
