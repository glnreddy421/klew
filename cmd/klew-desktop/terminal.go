package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/glnreddy421/klew/internal/kube"

	"github.com/creack/pty"
	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// TerminalOptions seeds a cluster-scoped shell session.
type TerminalOptions struct {
	Kubeconfig string `json:"kubeconfig"`
	Context    string `json:"context"`
	Namespace  string `json:"namespace"`
	Shell      string `json:"shell"` // empty or "system" → $SHELL
	Cols       int    `json:"cols"`
	Rows       int    `json:"rows"`
}

// TerminalShellChoice is one selectable shell for the first-run picker.
type TerminalShellChoice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Path  string `json:"path"`
}

// TerminalInfo describes an active PTY session.
type TerminalInfo struct {
	ID        string `json:"id"`
	Shell     string `json:"shell"`
	Context   string `json:"context"`
	Namespace string `json:"namespace"`
	// Notice explains a fallback (e.g. no cluster pinned); empty for a normal session.
	Notice string `json:"notice,omitempty"`
}

type terminalSession struct {
	id                string
	cmd               *exec.Cmd
	pty               *os.File
	done              chan struct{}
	kubeconfigCleanup func()
}

type terminalManager struct {
	app      *App
	mu       sync.Mutex
	sessions map[string]*terminalSession
}

func newTerminalManager(app *App) *terminalManager {
	go kube.SweepStaleTerminalKubeconfigs()
	return &terminalManager{
		app:      app,
		sessions: make(map[string]*terminalSession),
	}
}

func (m *terminalManager) closeAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, sess := range m.sessions {
		m.closeSessionLocked(id, sess)
	}
	m.sessions = make(map[string]*terminalSession)
}

// StartTerminal opens an interactive shell with KUBECONFIG and kubectl context applied.
func (a *App) StartTerminal(opts TerminalOptions) (TerminalInfo, error) {
	if a.ctx == nil {
		return TerminalInfo{}, fmt.Errorf("app not ready")
	}
	if a.terminals == nil {
		a.terminals = newTerminalManager(a)
	}
	return a.terminals.start(opts)
}

func (a *App) WriteTerminal(id, data string) error {
	if a.terminals == nil {
		return fmt.Errorf("terminal not found")
	}
	return a.terminals.write(id, data)
}

func (a *App) ResizeTerminal(id string, cols, rows int) error {
	if a.terminals == nil {
		return fmt.Errorf("terminal not found")
	}
	return a.terminals.resize(id, cols, rows)
}

func (a *App) CloseTerminal(id string) error {
	if a.terminals == nil {
		return nil
	}
	return a.terminals.close(id)
}

// GetTerminalShellChoices lists shells available for the in-app terminal picker.
func (a *App) GetTerminalShellChoices() []TerminalShellChoice {
	system := userShell()
	choices := []TerminalShellChoice{
		{
			ID:    "system",
			Label: "System default",
			Path:  system,
		},
	}
	seen := map[string]bool{system: true}
	for _, c := range []struct {
		id, path, label string
	}{
		{"zsh", "/bin/zsh", "zsh"},
		{"bash", "/bin/bash", "bash"},
		{"sh", "/bin/sh", "sh"},
	} {
		if _, err := os.Stat(c.path); err != nil {
			continue
		}
		if seen[c.path] {
			continue
		}
		seen[c.path] = true
		choices = append(choices, TerminalShellChoice{
			ID:    c.path,
			Label: c.label,
			Path:  c.path,
		})
	}
	return choices
}

func (m *terminalManager) start(opts TerminalOptions) (TerminalInfo, error) {
	shell := resolveTerminalShell(opts.Shell)
	kubeconfig := strings.TrimSpace(opts.Kubeconfig)
	contextName := strings.TrimSpace(opts.Context)
	// Empty namespace → the context's own namespace (or "default").
	namespace := strings.TrimSpace(opts.Namespace)

	// Cluster pinning is best effort: on any failure the user still gets their
	// normal shell, with a notice that kubectl is not pinned.
	shellKube, err := kube.PrepareTerminalShellKube(kubeconfig, contextName, namespace)
	if err != nil {
		shellKube = kube.PlainTerminalShellKube(err)
	}

	// Never block shell start on the API server: use what is cached and warm the
	// rest so later shells pick up a cluster-matched kubectl.
	clusterVersion := kube.CachedClusterVersion(kubeconfig, shellKube.Context)
	if clusterVersion == "" && shellKube.Context != "" {
		go warmTerminalKubectl(m.app, kubeconfig, shellKube.Context)
	}
	cmd, err := buildClusterShellCommand(shell, shellKube, kube.KubectlDirForClusterCached(clusterVersion))
	if err != nil {
		if shellKube.Cleanup != nil {
			shellKube.Cleanup()
		}
		return TerminalInfo{}, err
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		if shellKube.Cleanup != nil {
			shellKube.Cleanup()
		}
		return TerminalInfo{}, fmt.Errorf("start pty: %w", err)
	}

	cols, rows := opts.Cols, opts.Rows
	if cols < 20 {
		cols = 120
	}
	if rows < 5 {
		rows = 32
	}
	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})

	id := uuid.NewString()
	sess := &terminalSession{
		id:                id,
		cmd:               cmd,
		pty:               ptmx,
		done:              make(chan struct{}),
		kubeconfigCleanup: shellKube.Cleanup,
	}

	m.mu.Lock()
	m.sessions[id] = sess
	m.mu.Unlock()

	go m.pumpOutput(id, sess)
	go m.waitExit(id, sess)

	return TerminalInfo{
		ID:        id,
		Shell:     shell,
		Context:   shellKube.Context,
		Namespace: shellKube.Namespace,
		Notice:    shellKube.Notice,
	}, nil
}

func (m *terminalManager) write(id, data string) error {
	m.mu.Lock()
	sess := m.sessions[id]
	m.mu.Unlock()
	if sess == nil || sess.pty == nil {
		return fmt.Errorf("terminal not found")
	}
	_, err := sess.pty.WriteString(data)
	return err
}

func (m *terminalManager) resize(id string, cols, rows int) error {
	m.mu.Lock()
	sess := m.sessions[id]
	m.mu.Unlock()
	if sess == nil || sess.pty == nil {
		return fmt.Errorf("terminal not found")
	}
	if cols < 2 {
		cols = 2
	}
	if rows < 2 {
		rows = 2
	}
	return pty.Setsize(sess.pty, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

func (m *terminalManager) close(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	sess := m.sessions[id]
	if sess == nil {
		return nil
	}
	m.closeSessionLocked(id, sess)
	delete(m.sessions, id)
	return nil
}

func (m *terminalManager) closeSessionLocked(id string, sess *terminalSession) {
	select {
	case <-sess.done:
	default:
		close(sess.done)
	}
	if sess.kubeconfigCleanup != nil {
		sess.kubeconfigCleanup()
		sess.kubeconfigCleanup = nil
	}
	if sess.pty != nil {
		_ = sess.pty.Close()
	}
	if sess.cmd != nil && sess.cmd.Process != nil {
		_ = sess.cmd.Process.Kill()
	}
	if m.app != nil && m.app.ctx != nil {
		runtime.EventsEmit(m.app.ctx, "terminal:exit", map[string]string{"id": id})
	}
}

func (m *terminalManager) pumpOutput(id string, sess *terminalSession) {
	buf := make([]byte, 4096)
	for {
		n, err := sess.pty.Read(buf)
		if n > 0 && m.app != nil && m.app.ctx != nil {
			runtime.EventsEmit(m.app.ctx, "terminal:output", map[string]string{
				"id":   id,
				"data": string(buf[:n]),
			})
		}
		if err != nil {
			if err != io.EOF {
				runtime.EventsEmit(m.app.ctx, "terminal:output", map[string]string{
					"id":   id,
					"data": fmt.Sprintf("\r\n\x1b[33m[session ended: %v]\x1b[0m\r\n", err),
				})
			}
			return
		}
	}
}

func (m *terminalManager) waitExit(id string, sess *terminalSession) {
	if sess.cmd != nil {
		_ = sess.cmd.Wait()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if cur, ok := m.sessions[id]; ok && cur == sess {
		m.closeSessionLocked(id, sess)
		delete(m.sessions, id)
	}
}

var terminalWarmInFlight sync.Map

func warmTerminalKubectl(app *App, kubeconfig, contextName string) {
	key := kubeconfig + "\x00" + contextName
	if _, busy := terminalWarmInFlight.LoadOrStore(key, struct{}{}); busy {
		return
	}
	defer terminalWarmInFlight.Delete(key)

	ctx := context.Background()
	if app != nil && app.ctx != nil {
		ctx = app.ctx
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if st := kube.CollectClusterStatus(ctx, kubeconfig, contextName); st.KubernetesVersion != "" {
		kube.ResolveKubectlForCluster(st.KubernetesVersion)
	}
}

func buildClusterShellCommand(shell string, tk kube.TerminalShellKube, kubectlDir string) (*exec.Cmd, error) {
	shell = strings.TrimSpace(shell)
	if shell == "" {
		return nil, fmt.Errorf("no shell available")
	}

	if goruntime.GOOS == "windows" {
		return buildWindowsShellCommand(shell, tk, kubectlDir)
	}

	// Like an IDE terminal: the user's own login shell, pinned only through env.
	cmd := exec.Command(shell, "-l")
	cmd.Env = clusterShellEnv(tk.KubeconfigPath, tk.Context, tk.Namespace, kubectlDir)
	cmd.Dir = homeDir()
	return cmd, nil
}

func resolveTerminalShell(requested string) string {
	requested = strings.TrimSpace(requested)
	if requested == "" || requested == "system" {
		return userShell()
	}
	if _, err := os.Stat(requested); err == nil {
		return requested
	}
	return userShell()
}

func buildWindowsShellCommand(shell string, tk kube.TerminalShellKube, kubectlDir string) (*exec.Cmd, error) {
	cmd := exec.Command(shell, "-NoLogo")
	cmd.Env = clusterShellEnv(tk.KubeconfigPath, tk.Context, tk.Namespace, kubectlDir)
	cmd.Dir = homeDir()
	return cmd, nil
}

func clusterShellEnv(kubeconfig, contextName, namespace, kubectlDir string) []string {
	env := os.Environ()
	if dir := kubectlDir; dir != "" {
		path := os.Getenv("PATH")
		if path == "" {
			path = dir
		} else {
			path = dir + string(os.PathListSeparator) + path
		}
		env = appendEnv(env, "PATH="+path)
	}
	if kubeconfig != "" {
		env = appendEnv(env, "KUBECONFIG="+kubeconfig)
	}
	if contextName != "" {
		env = appendEnv(env, "KLEW_CONTEXT="+contextName)
	}
	if namespace != "" {
		env = appendEnv(env, "KLEW_NAMESPACE="+namespace)
	}
	term := os.Getenv("TERM")
	if term == "" {
		term = "xterm-256color"
	}
	env = appendEnv(env, "TERM="+term)
	return env
}

func appendEnv(env []string, kv string) []string {
	key, _, _ := strings.Cut(kv, "=")
	out := env[:0]
	for _, e := range env {
		if !strings.HasPrefix(e, key+"=") {
			out = append(out, e)
		}
	}
	return append(out, kv)
}

func userShell() string {
	if goruntime.GOOS == "windows" {
		return "powershell.exe"
	}
	if s := strings.TrimSpace(os.Getenv("SHELL")); s != "" {
		return s
	}
	if _, err := os.Stat("/bin/zsh"); err == nil {
		return "/bin/zsh"
	}
	return "/bin/bash"
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	return "."
}
