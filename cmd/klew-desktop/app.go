package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/glnreddy421/klew/internal/api"
	"github.com/glnreddy421/klew/internal/details"
	"github.com/glnreddy421/klew/internal/engine"
	"github.com/glnreddy421/klew/internal/kube"
	"github.com/glnreddy421/klew/internal/model"
	"github.com/glnreddy421/klew/internal/service"
)

// bootOptions seeds a window from CLI flags (used when cloning a window).
type bootOptions struct {
	Context    string
	Namespace  string
	Kubeconfig string
}

// OpenWindowOptions opens a new Klew process/window (iTerm/Warp-style clone).
// Empty fields inherit from the current window's cluster state.
type OpenWindowOptions struct {
	Context    string `json:"context"`
	Namespace  string `json:"namespace"`
	Kubeconfig string `json:"kubeconfig"`
}

// App exposes the shared investigation backend to the React frontend.
type App struct {
	ctx context.Context
	mu  sync.Mutex
	svc *service.Service

	boot    bootOptions
	cluster kube.ClusterState

	terminals *terminalManager

	watchCancel context.CancelFunc
	rootCancel  context.CancelFunc

	lastUIEmit    time.Time
	uiEmitPending *time.Timer
}

func NewApp(boot bootOptions) *App { return &App{boot: boot} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.boot.Kubeconfig != "" {
		a.cluster.KubeconfigPath = a.boot.Kubeconfig
	}
	a.cluster = a.refreshCluster(a.boot.Context, a.boot.Namespace)
	a.updateWindowTitle()
	a.emitCluster()
}

func (a *App) shutdown(context.Context) {
	a.stopInvestigation()
	if a.terminals != nil {
		a.terminals.closeAll()
	}
}

func (a *App) refreshCluster(contextName, namespace string) kube.ClusterState {
	if a.ctx == nil {
		a.ctx = context.Background()
	}
	selectedCtx := contextName
	selectedNS := namespace
	if selectedCtx == "" {
		selectedCtx = a.cluster.SelectedContext
	}
	if selectedNS == "" {
		selectedNS = a.cluster.SelectedNamespace
	}
	return kube.RefreshClusterState(a.ctx, a.cluster.KubeconfigPath, selectedCtx, selectedNS)
}

func (a *App) invalidateClusterStatusCache() {
	a.mu.Lock()
	path := a.cluster.KubeconfigPath
	ctxName := a.cluster.SelectedContext
	if ctxName == "" {
		ctxName = a.cluster.CurrentContext
	}
	a.mu.Unlock()
	if ctxName == "" {
		return
	}
	client, err := kube.NewFromFlags(path, ctxName, "")
	if err != nil {
		return
	}
	kube.InvalidateClusterStatus(path, client)
}

func (a *App) emitCluster() {
	if a.ctx == nil {
		return
	}
	a.mu.Lock()
	st := a.cluster
	a.mu.Unlock()
	a.updateWindowTitle()
	runtime.EventsEmit(a.ctx, "cluster", st)
}

func (a *App) updateWindowTitle() {
	if a.ctx == nil {
		return
	}
	a.mu.Lock()
	ctxName := a.cluster.SelectedContext
	if ctxName == "" {
		ctxName = a.cluster.CurrentContext
	}
	ns := a.cluster.SelectedNamespace
	a.mu.Unlock()

	title := "Klew"
	switch {
	case ctxName != "" && ns != "":
		title = fmt.Sprintf("Klew — %s / %s", ctxName, ns)
	case ctxName != "":
		title = fmt.Sprintf("Klew — %s", ctxName)
	}
	runtime.WindowSetTitle(a.ctx, title)
}

// OpenNewWindow spawns a new Klew process for another cluster/context.
// The current window is left unchanged (multi-cluster via multiple windows).
func (a *App) OpenNewWindow(opts OpenWindowOptions) error {
	a.mu.Lock()
	cur := a.cluster
	a.mu.Unlock()

	kubeconfig := opts.Kubeconfig
	if kubeconfig == "" {
		kubeconfig = cur.KubeconfigPath
	}
	contextName := opts.Context
	if contextName == "" {
		contextName = cur.SelectedContext
		if contextName == "" {
			contextName = cur.CurrentContext
		}
	}
	namespace := opts.Namespace
	if namespace == "" && contextName == cur.SelectedContext {
		namespace = cur.SelectedNamespace
	}
	if namespace == "" {
		for _, c := range cur.Contexts {
			if c.Name == contextName {
				namespace = c.Namespace
				break
			}
		}
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	args := make([]string, 0, 3)
	if kubeconfig != "" {
		args = append(args, "--kubeconfig="+kubeconfig)
	}
	if contextName != "" {
		args = append(args, "--context="+contextName)
	}
	if namespace != "" {
		args = append(args, "--namespace="+namespace)
	}

	cmd := exec.Command(exe, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open new window: %w", err)
	}
	_ = cmd.Process.Release()
	return nil
}

// GetCluster returns the current context/namespace picker state.
func (a *App) GetCluster() kube.ClusterState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cluster
}

// GetClusterStatus returns cluster-wide API reachability, version, and node inventory.
func (a *App) GetClusterStatus() kube.ClusterStatus {
	a.mu.Lock()
	path := a.cluster.KubeconfigPath
	ctxName := a.cluster.SelectedContext
	if ctxName == "" {
		ctxName = a.cluster.CurrentContext
	}
	a.mu.Unlock()

	c := a.ctx
	if c == nil {
		c = context.Background()
	}
	return kube.CollectClusterStatus(c, path, ctxName)
}

// SyncCluster reloads kubeconfig from disk and refreshes namespace list.
func (a *App) SyncCluster() kube.ClusterState {
	a.mu.Lock()
	ctx := a.cluster.SelectedContext
	ns := a.cluster.SelectedNamespace
	a.mu.Unlock()

	st := a.refreshCluster(ctx, ns)
	a.mu.Lock()
	a.cluster = st
	a.mu.Unlock()
	a.invalidateClusterStatusCache()
	a.emitCluster()
	return st
}

// SelectContext switches the active kube context and reloads namespaces.
func (a *App) SelectContext(contextName string) kube.ClusterState {
	if contextName == "" {
		return a.GetCluster()
	}
	a.mu.Lock()
	defaultNS := ""
	for _, c := range a.cluster.Contexts {
		if c.Name == contextName {
			defaultNS = c.Namespace
			break
		}
	}
	a.mu.Unlock()

	st := a.refreshCluster(contextName, defaultNS)
	a.mu.Lock()
	a.cluster = st
	a.mu.Unlock()
	a.invalidateClusterStatusCache()
	a.emitCluster()
	return st
}

// SelectNamespace updates the investigation namespace boundary.
func (a *App) SelectNamespace(namespace string) kube.ClusterState {
	a.mu.Lock()
	a.cluster.SelectedNamespace = namespace
	st := a.cluster
	a.mu.Unlock()
	a.emitCluster()
	return st
}

// GetView returns the current investigation snapshot for the UI.
// GetObjectDetails returns a kind-aware Kubernetes object inspector payload.
func (a *App) GetObjectDetails(kind, name, namespace string) (*details.ObjectDetail, error) {
	a.mu.Lock()
	svc := a.svc
	a.mu.Unlock()
	if svc == nil {
		return nil, fmt.Errorf("no active investigation")
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return svc.ObjectDetails(ctx, kind, name, namespace)
}

func (a *App) GetView() api.View {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.svc == nil {
		return api.View{}
	}
	return a.svc.View()
}

// DiscoverOptions configures pre-flight match discovery before investigation.
type DiscoverOptions struct {
	Query      string `json:"query"`
	Namespace  string `json:"namespace"`
	Kubeconfig string `json:"kubeconfig"`
	Context    string `json:"context"`
}

// DiscoverMatches finds resources matching the query in a namespace by name
// substring across core Kubernetes kinds (workloads, networking, config, RBAC).
// An empty query returns every listed resource in the namespace.
func (a *App) DiscoverMatches(opts DiscoverOptions) ([]model.MatchedObject, error) {
	if a.ctx == nil {
		a.ctx = context.Background()
	}

	kcfg := opts.Kubeconfig
	ctxName := opts.Context
	ns := opts.Namespace
	if kcfg == "" || ctxName == "" || ns == "" {
		a.mu.Lock()
		cluster := a.cluster
		a.mu.Unlock()
		if kcfg == "" {
			kcfg = cluster.KubeconfigPath
		}
		if ctxName == "" {
			ctxName = cluster.SelectedContext
		}
		if ns == "" {
			ns = cluster.SelectedNamespace
		}
	}

	client, err := kube.NewFromFlags(kcfg, ctxName, ns)
	if err != nil {
		return nil, err
	}
	if ns == "" {
		ns = client.Namespace
		if ns == "" {
			ns = client.ContextNamespace
		}
	}
	matches, err := kube.DiscoverMatches(a.ctx, client, ns, opts.Query)
	if matches == nil {
		matches = []model.MatchedObject{}
	}
	return matches, err
}

// CatalogOptions configures dynamic resource catalog discovery.
type CatalogOptions struct {
	Namespace     string `json:"namespace"`
	Kubeconfig    string `json:"kubeconfig"`
	Context       string `json:"context"`
	IncludeCounts bool   `json:"includeCounts"`
	Refresh       bool   `json:"refresh"`
}

func (a *App) resolveCatalogClient(opts CatalogOptions) (*kube.Client, string, error) {
	if a.ctx == nil {
		a.ctx = context.Background()
	}
	kcfg := opts.Kubeconfig
	ctxName := opts.Context
	ns := opts.Namespace
	if kcfg == "" || ctxName == "" || ns == "" {
		a.mu.Lock()
		cluster := a.cluster
		a.mu.Unlock()
		if kcfg == "" {
			kcfg = cluster.KubeconfigPath
		}
		if ctxName == "" {
			ctxName = cluster.SelectedContext
		}
		if ns == "" {
			ns = cluster.SelectedNamespace
		}
	}
	client, err := kube.NewFromFlags(kcfg, ctxName, ns)
	if err != nil {
		return nil, "", err
	}
	if ns == "" {
		ns = client.Namespace
		if ns == "" {
			ns = client.ContextNamespace
		}
	}
	return client, ns, nil
}

// GetResourceCatalog returns a discovery-driven, RBAC-aware resource catalog.
func (a *App) GetResourceCatalog(opts CatalogOptions) (model.ResourceCatalog, error) {
	client, ns, err := a.resolveCatalogClient(opts)
	if err != nil {
		return model.ResourceCatalog{}, err
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.Refresh {
		return kube.RefreshResourceCatalog(ctx, client, ns, opts.IncludeCounts)
	}
	return kube.BuildResourceCatalog(ctx, client, ns, opts.IncludeCounts)
}

// RefreshResourceCatalog invalidates cached discovery/auth and rebuilds the catalog.
func (a *App) RefreshResourceCatalog(opts CatalogOptions) (model.ResourceCatalog, error) {
	opts.Refresh = true
	return a.GetResourceCatalog(opts)
}

// ListCatalogEntitiesOptions configures lazy entity listing for a resource GVR.
type ListCatalogEntitiesOptions struct {
	ResourceID    string `json:"resourceId"`
	Namespace     string `json:"namespace"`
	ClusterScoped bool   `json:"clusterScoped"`
	Kubeconfig    string `json:"kubeconfig"`
	Context       string `json:"context"`
}

// ListCatalogEntities returns lightweight entities for a selected catalog resource.
func (a *App) ListCatalogEntities(opts ListCatalogEntitiesOptions) (model.CatalogEntityList, error) {
	client, ns, err := a.resolveCatalogClient(CatalogOptions{
		Namespace:  opts.Namespace,
		Kubeconfig: opts.Kubeconfig,
		Context:    opts.Context,
	})
	if err != nil {
		return model.CatalogEntityList{AccessState: model.ResourceAccessError, Error: err.Error()}, err
	}
	if opts.ResourceID == "" {
		return model.CatalogEntityList{AccessState: model.ResourceAccessError, Error: "resourceId is required"}, fmt.Errorf("resourceId is required")
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	listNS := ns
	if opts.Namespace != "" {
		listNS = opts.Namespace
	}
	if opts.ClusterScoped {
		listNS = ""
	}
	return kube.ListCatalogEntities(ctx, client, listNS, opts.ResourceID)
}

// StartOptions configures a live investigation from the desktop UI.
type StartOptions struct {
	Query            string `json:"query"`
	Namespace        string `json:"namespace"`
	AllNamespaces    bool   `json:"allNamespaces"`
	Kubeconfig       string `json:"kubeconfig"`
	Context          string `json:"context"`
	Tail             int    `json:"tail"`
	RefreshSec       int    `json:"refreshSec"`
	WindowSec        int    `json:"windowSec"`
	MaxLogRequests   int    `json:"maxLogRequests"`
	AutoRefresh      *bool  `json:"autoRefresh"`
	UseMetricsServer *bool  `json:"useMetricsServer"`
}

// StartInvestigation begins live cluster collection.
func (a *App) StartInvestigation(opts StartOptions) error {
	a.stopInvestigation()

	if opts.Tail <= 0 {
		opts.Tail = 200
	}
	refresh := time.Duration(opts.RefreshSec) * time.Second
	if refresh <= 0 {
		refresh = 10 * time.Second
	}
	window := time.Duration(opts.WindowSec) * time.Second
	if window <= 0 {
		window = 15 * time.Minute
	}
	autoRefresh := true
	if opts.AutoRefresh != nil {
		autoRefresh = *opts.AutoRefresh
	}
	disableMetrics := false
	if opts.UseMetricsServer != nil {
		disableMetrics = !*opts.UseMetricsServer
	}

	rootCtx, rootCancel := context.WithCancel(a.ctx)
	watchCtx, watchCancel := context.WithCancel(rootCtx)

	a.mu.Lock()
	a.rootCancel = rootCancel
	a.watchCancel = watchCancel
	a.mu.Unlock()

	kcfg := opts.Kubeconfig
	if kcfg == "" {
		a.mu.Lock()
		cluster := a.cluster
		a.mu.Unlock()
		kcfg = cluster.KubeconfigPath
	}
	ctxName := opts.Context
	if ctxName == "" {
		a.mu.Lock()
		cluster := a.cluster
		a.mu.Unlock()
		ctxName = cluster.SelectedContext
	}
	ns := opts.Namespace
	if ns == "" {
		a.mu.Lock()
		cluster := a.cluster
		a.mu.Unlock()
		ns = cluster.SelectedNamespace
	}

	client, err := kube.NewFromFlags(kcfg, ctxName, ns)
	if err != nil {
		a.stopInvestigation()
		return err
	}
	allNS := opts.AllNamespaces
	if !allNS && ns == "" {
		ns = client.ContextNamespace
		if ns == "" {
			ns = client.Namespace
		}
	}
	svc, err := service.Start(rootCtx, client, engine.LiveOptions{
		Query:          opts.Query,
		Namespace:      ns,
		AllNS:          allNS,
		Tail:           opts.Tail,
		PollEvery:      refresh,
		Window:         window,
		MaxLogRequests: opts.MaxLogRequests,
		AutoRefresh:    autoRefresh,
		DisableMetrics: disableMetrics,
	})
	if err != nil {
		a.stopInvestigation()
		return err
	}
	a.mu.Lock()
	a.svc = svc
	a.mu.Unlock()
	go a.watchLoop(watchCtx)
	return nil
}

// StopInvestigation ends the active live session.
func (a *App) StopInvestigation() {
	a.stopInvestigation()
}

// LogTailOptions configures on-demand multipod log gathering.
type LogTailOptions struct {
	PodNames []string `json:"podNames"`
}

// LogTailActive reports whether log follows are running for the active session.
func (a *App) LogTailActive() bool {
	a.mu.Lock()
	svc := a.svc
	a.mu.Unlock()
	if svc == nil {
		return false
	}
	return svc.LogTailActive()
}

// StartLogTail begins gathering container logs from pods in the investigation scope.
func (a *App) StartLogTail(opts LogTailOptions) error {
	a.mu.Lock()
	svc := a.svc
	a.mu.Unlock()
	if svc == nil {
		return fmt.Errorf("no active investigation")
	}
	return svc.StartLogTail(engine.LogTailOptions{PodNames: opts.PodNames})
}

// StopLogTail stops log follows without ending the investigation.
func (a *App) StopLogTail() {
	a.mu.Lock()
	svc := a.svc
	a.mu.Unlock()
	if svc != nil {
		svc.StopLogTail()
	}
}

// PauseLogTail closes GetLogs streams while retaining the gather selection.
func (a *App) PauseLogTail() error {
	a.mu.Lock()
	svc := a.svc
	a.mu.Unlock()
	if svc == nil {
		return fmt.Errorf("no active investigation")
	}
	err := svc.PauseLogTail()
	if err == nil {
		a.emitState()
	}
	return err
}

// ResumeLogTail reopens GetLogs follows for the paused gather selection.
func (a *App) ResumeLogTail() error {
	a.mu.Lock()
	svc := a.svc
	a.mu.Unlock()
	if svc == nil {
		return fmt.Errorf("no active investigation")
	}
	err := svc.ResumeLogTail()
	if err == nil {
		a.emitState()
	}
	return err
}

// ClearLogs removes all buffered log lines from the active investigation.
func (a *App) ClearLogs() {
	a.mu.Lock()
	svc := a.svc
	a.mu.Unlock()
	if svc != nil {
		svc.ClearLogs()
	}
}

func (a *App) stopInvestigation() {
	a.mu.Lock()
	watchCancel := a.watchCancel
	rootCancel := a.rootCancel
	svc := a.svc
	pending := a.uiEmitPending
	a.watchCancel = nil
	a.rootCancel = nil
	a.svc = nil
	a.uiEmitPending = nil
	a.mu.Unlock()

	if pending != nil {
		pending.Stop()
	}
	if watchCancel != nil {
		watchCancel()
	}
	if svc != nil {
		svc.Stop()
	}
	if rootCancel != nil {
		rootCancel()
	}
}

func (a *App) watchLoop(ctx context.Context) {
	a.emitState()
	a.mu.Lock()
	svc := a.svc
	a.mu.Unlock()
	if svc == nil {
		return
	}
	svc.Watch(ctx, func(_ model.InvestigationState) {
		a.emitState()
	})
}

func (a *App) emitState() {
	if a.ctx == nil {
		return
	}
	// CapState + Wails JSON is relatively expensive; never emit denser than ~3/sec.
	a.mu.Lock()
	now := time.Now()
	if !a.lastUIEmit.IsZero() && now.Sub(a.lastUIEmit) < 350*time.Millisecond {
		if a.uiEmitPending == nil {
			delay := 350*time.Millisecond - now.Sub(a.lastUIEmit)
			a.uiEmitPending = time.AfterFunc(delay, func() {
				a.mu.Lock()
				a.uiEmitPending = nil
				a.mu.Unlock()
				a.emitStateNow()
			})
		}
		a.mu.Unlock()
		return
	}
	a.mu.Unlock()
	a.emitStateNow()
}

func (a *App) emitStateNow() {
	if a.ctx == nil {
		return
	}
	view := a.GetView()
	a.mu.Lock()
	a.lastUIEmit = time.Now()
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, "state", view)
}

// SetAutoRefresh toggles snapshot refresh during a live session.
func (a *App) SetAutoRefresh(enabled bool) {
	a.mu.Lock()
	svc := a.svc
	a.mu.Unlock()
	if svc != nil {
		svc.SetAutoRefresh(enabled)
	}
}

// SetPollEverySec changes the snapshot refresh interval for the active session.
func (a *App) SetPollEverySec(sec int) {
	if sec <= 0 {
		return
	}
	a.mu.Lock()
	svc := a.svc
	a.mu.Unlock()
	if svc != nil {
		svc.SetPollEvery(time.Duration(sec) * time.Second)
	}
}

// OpenKubeconfigDir opens the default kubeconfig directory in the system browser.
func (a *App) OpenKubeconfigDir() {
	home, err := os.UserHomeDir()
	if err != nil || a.ctx == nil {
		return
	}
	runtime.BrowserOpenURL(a.ctx, "file://"+home+"/.kube")
}

// SetKubeconfigPath updates the kubeconfig path and refreshes cluster state.
// Empty path clears the override so RefreshClusterState uses default resolution.
func (a *App) SetKubeconfigPath(path string) kube.ClusterState {
	a.mu.Lock()
	a.cluster.KubeconfigPath = path
	ctxName := a.cluster.SelectedContext
	ns := a.cluster.SelectedNamespace
	a.mu.Unlock()

	st := a.refreshCluster(ctxName, ns)
	a.mu.Lock()
	a.cluster = st
	a.mu.Unlock()
	a.emitCluster()
	return st
}
