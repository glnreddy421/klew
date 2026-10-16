package kube

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// TerminalShellKube configures an embedded terminal session without mutating the
// user's on-disk kubeconfig.
type TerminalShellKube struct {
	// KubeconfigPath is exported as KUBECONFIG; empty leaves the shell's own env.
	KubeconfigPath string
	// Context is empty for a plain shell that is not pinned to any cluster.
	Context   string
	Namespace string
	// Isolated is true when KubeconfigPath is a session-local copy.
	Isolated bool
	// Notice is shown in the shell banner when the session is not a normal,
	// isolated cluster shell (fallbacks, missing kubeconfig).
	Notice  string
	Cleanup func()
}

// ErrKubeconfigUnavailable marks kubeconfig problems (missing file, no current
// context, unknown context) where a plain shell is still a reasonable result.
var ErrKubeconfigUnavailable = errors.New("kubeconfig unavailable")

const terminalKubeconfigPrefix = "kubeconfig-"

// writeTerminalKubeconfigEncoded is overridden in tests to simulate write failures.
var writeTerminalKubeconfigEncoded = defaultWriteTerminalKubeconfigEncoded

// PrepareTerminalShellKube loads kubeconfig and writes a session-local copy pinned
// to the requested context and namespace (empty namespace → the context's own).
// The caller's kubeconfig file is never modified.
//
// Errors wrapping ErrKubeconfigUnavailable mean no cluster can be pinned; callers
// may fall back to PlainTerminalShellKube. If the session copy cannot be written,
// the on-disk kubeconfig is used only when its current-context already matches
// the requested context; otherwise an error is returned so the shell never
// silently targets a different cluster.
func PrepareTerminalShellKube(sourcePath, contextName, namespace string) (TerminalShellKube, error) {
	tc, err := loadTerminalKubeconfig(sourcePath, contextName, namespace)
	if err != nil {
		return TerminalShellKube{}, err
	}

	encoded, err := clientcmd.Write(tc.raw)
	if err != nil {
		return TerminalShellKube{}, fmt.Errorf("encode kubeconfig: %w", err)
	}

	sessionPath, cleanup, writeErr := writeTerminalKubeconfigEncoded(encoded)
	if writeErr == nil {
		return TerminalShellKube{
			KubeconfigPath: sessionPath,
			Context:        tc.context,
			Namespace:      tc.namespace,
			Isolated:       true,
			Cleanup:        cleanup,
		}, nil
	}

	if tc.fileContext != tc.context {
		return TerminalShellKube{}, fmt.Errorf(
			"cannot write session kubeconfig for context %q (kubeconfig current-context is %q): %w",
			tc.context, tc.fileContext, writeErr,
		)
	}
	return TerminalShellKube{
		KubeconfigPath: tc.resolvedPath,
		Context:        tc.context,
		Namespace:      tc.fileNamespace,
		Notice:         "Using your kubeconfig directly (session copy unavailable); namespace changes here affect it.",
	}, nil
}

// PlainTerminalShellKube is a shell with the user's own environment and no
// cluster pinning, used when the kubeconfig cannot be loaded.
func PlainTerminalShellKube(reason error) TerminalShellKube {
	return TerminalShellKube{
		Notice: fmt.Sprintf("No cluster context pinned: %v. kubectl uses your shell's own configuration.", reason),
	}
}

type terminalKubeconfig struct {
	raw          clientcmdapi.Config
	context      string
	namespace    string
	resolvedPath string
	// fileContext / fileNamespace describe the on-disk kubeconfig before any
	// session overrides are applied.
	fileContext   string
	fileNamespace string
}

func loadTerminalKubeconfig(sourcePath, contextName, namespace string) (terminalKubeconfig, error) {
	loadingRules := configureLoadingRules(sourcePath)
	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})
	r, err := cfg.RawConfig()
	if err != nil {
		return terminalKubeconfig{}, fmt.Errorf("%w: read kubeconfig: %v", ErrKubeconfigUnavailable, err)
	}

	out := terminalKubeconfig{
		fileContext:   r.CurrentContext,
		fileNamespace: contextNamespace(r, r.CurrentContext),
		resolvedPath:  resolvedKubeconfigPath(sourcePath),
	}

	if contextName != "" {
		r.CurrentContext = contextName
	}
	if r.CurrentContext == "" {
		return terminalKubeconfig{}, fmt.Errorf("%w: kubeconfig has no current context", ErrKubeconfigUnavailable)
	}
	ctx := r.Contexts[r.CurrentContext]
	if ctx == nil {
		return terminalKubeconfig{}, fmt.Errorf("%w: context %q not found in kubeconfig", ErrKubeconfigUnavailable, r.CurrentContext)
	}

	out.context = r.CurrentContext
	out.namespace = namespace
	if out.namespace == "" {
		out.namespace = contextNamespace(r, out.context)
	}

	// Copy before overriding so the loaded config's shared *Context is untouched.
	pinned := *ctx
	pinned.Namespace = out.namespace
	r.Contexts[out.context] = &pinned
	out.raw = r
	return out, nil
}

func contextNamespace(cfg clientcmdapi.Config, name string) string {
	if c := cfg.Contexts[name]; c != nil && c.Namespace != "" {
		return c.Namespace
	}
	return "default"
}

func resolvedKubeconfigPath(sourcePath string) string {
	_, path, err := configLoader(sourcePath)
	if err == nil && strings.TrimSpace(path) != "" {
		return path
	}
	if p := strings.TrimSpace(sourcePath); p != "" {
		return p
	}
	if p := DefaultKubeconfigPath(); p != "" {
		return p
	}
	return ""
}

// terminalSessionKubeconfigDirs lists candidate directories, most private first.
// Session files hold credentials, so per-user locations beat the shared temp dir.
func terminalSessionKubeconfigDirs() []string {
	var out []string
	if cache, err := os.UserCacheDir(); err == nil {
		out = append(out, filepath.Join(cache, "klew", "terminal-sessions"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		out = append(out, filepath.Join(home, ".klew", "terminal-sessions"))
	}
	tmpName := "klew-terminal-kubeconfig"
	if uid := os.Getuid(); uid >= 0 {
		tmpName += "-" + strconv.Itoa(uid)
	}
	return append(out, filepath.Join(os.TempDir(), tmpName))
}

func defaultWriteTerminalKubeconfigEncoded(encoded []byte) (sessionPath string, cleanup func(), err error) {
	var lastErr error
	for _, dir := range terminalSessionKubeconfigDirs() {
		path, cleanup, err := writeTerminalKubeconfigInDir(dir, encoded)
		if err == nil {
			return path, cleanup, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("session kubeconfig unavailable")
	}
	return "", nil, lastErr
}

// ensurePrivateDir creates dir (0700) or accepts an existing one only if it is a
// real directory owned by the current user and not accessible to others. This
// blocks a pre-created shared directory from being used to swap the file.
func ensurePrivateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s is not a private directory", dir)
	}
	if !ownedByCurrentUser(info) {
		return fmt.Errorf("%s is owned by another user", dir)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("restrict %s: %w", dir, err)
		}
	}
	return nil
}

func writeTerminalKubeconfigInDir(dir string, encoded []byte) (sessionPath string, cleanup func(), err error) {
	if err := ensurePrivateDir(dir); err != nil {
		return "", nil, err
	}
	// The owning PID is embedded so stale files from crashed processes can be swept.
	f, err := os.CreateTemp(dir, fmt.Sprintf("%s%d-*", terminalKubeconfigPrefix, os.Getpid()))
	if err != nil {
		return "", nil, err
	}
	path := f.Name()
	if _, err := f.Write(encoded); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", nil, err
	}
	return path, func() { _ = os.Remove(path) }, nil
}

// SweepStaleTerminalKubeconfigs removes session kubeconfigs (which hold
// credentials) left behind by Klew processes that exited without cleanup.
// Files owned by live processes, including other Klew windows, are kept.
func SweepStaleTerminalKubeconfigs() {
	for _, dir := range terminalSessionKubeconfigDirs() {
		sweepStaleTerminalKubeconfigsIn(dir, processAlive)
	}
}

func sweepStaleTerminalKubeconfigsIn(dir string, alive func(pid int) bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	self := os.Getpid()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		pid, ok := terminalKubeconfigOwnerPID(e.Name())
		if !ok || pid == self || alive(pid) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, e.Name()))
	}
}

func terminalKubeconfigOwnerPID(name string) (int, bool) {
	rest, ok := strings.CutPrefix(name, terminalKubeconfigPrefix)
	if !ok {
		return 0, false
	}
	pidStr, _, ok := strings.Cut(rest, "-")
	if !ok {
		return 0, false
	}
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	defer p.Release()
	if runtime.GOOS == "windows" {
		// FindProcess opens a handle on Windows and fails for missing PIDs.
		return true
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}
