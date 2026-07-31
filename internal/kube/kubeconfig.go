package kube

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"k8s.io/client-go/tools/clientcmd"
)

// ContextOption is one kubeconfig context entry for UI pickers.
type ContextOption struct {
	Name      string `json:"name"`
	Cluster   string `json:"cluster"`
	User      string `json:"user"`
	Namespace string `json:"namespace"`
	IsCurrent bool   `json:"isCurrent"`
}

// ClusterState is the dynamic cluster/namespace selection shown in the desktop top bar.
type ClusterState struct {
	KubeconfigPath    string          `json:"kubeconfigPath"`
	CurrentContext    string          `json:"currentContext"`
	SelectedContext   string          `json:"selectedContext"`
	SelectedNamespace string          `json:"selectedNamespace"`
	Cluster           string          `json:"cluster"`
	User              string          `json:"user"`
	Contexts          []ContextOption `json:"contexts"`
	Namespaces        []string        `json:"namespaces"`
	SyncedAt          time.Time       `json:"syncedAt"`
	SyncError         string          `json:"syncError,omitempty"`
}

// LoadKubeConfigSnapshot reads contexts from kubeconfig on disk (no cluster calls).
func LoadKubeConfigSnapshot(kubeconfigPath string) (ClusterState, error) {
	loader, path, err := configLoader(kubeconfigPath)
	if err != nil {
		return ClusterState{}, err
	}
	raw, err := loader.RawConfig()
	if err != nil {
		return ClusterState{}, fmt.Errorf("read kubeconfig: %w", err)
	}
	if raw.CurrentContext == "" {
		return ClusterState{}, fmt.Errorf("kubeconfig has no current context")
	}

	names := make([]string, 0, len(raw.Contexts))
	for name := range raw.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)

	contexts := make([]ContextOption, 0, len(names))
	for _, name := range names {
		c := raw.Contexts[name]
		contexts = append(contexts, ContextOption{
			Name:      name,
			Cluster:   c.Cluster,
			User:      c.AuthInfo,
			Namespace: c.Namespace,
			IsCurrent: name == raw.CurrentContext,
		})
	}

	selected := raw.CurrentContext
	ctxCfg := raw.Contexts[selected]
	ns := ctxCfg.Namespace
	if ns == "" {
		ns = "default"
	}

	return ClusterState{
		KubeconfigPath:    path,
		CurrentContext:    raw.CurrentContext,
		SelectedContext:   selected,
		SelectedNamespace: ns,
		Cluster:           ctxCfg.Cluster,
		User:              ctxCfg.AuthInfo,
		Contexts:          contexts,
		Namespaces:        []string{ns},
		SyncedAt:          time.Now().UTC(),
	}, nil
}

// RefreshClusterState reloads kubeconfig and live namespace list for the selected context.
func RefreshClusterState(ctx context.Context, kubeconfigPath, selectedContext, selectedNamespace string) ClusterState {
	base, err := LoadKubeConfigSnapshot(kubeconfigPath)
	if err != nil {
		path := kubeconfigPath
		if path == "" {
			path = DefaultKubeconfigPath()
		}
		return ClusterState{
			KubeconfigPath: path,
			SyncedAt:       time.Now().UTC(),
			SyncError:      err.Error(),
		}
	}

	if selectedContext != "" {
		base.SelectedContext = selectedContext
	}
	if ctx := findContext(base.Contexts, base.SelectedContext); ctx != nil {
		base.Cluster = ctx.Cluster
		base.User = ctx.User
		if selectedNamespace == "" && ctx.Namespace != "" {
			base.SelectedNamespace = ctx.Namespace
		}
	}

	if selectedNamespace != "" {
		base.SelectedNamespace = selectedNamespace
	}
	if base.SelectedNamespace == "" {
		base.SelectedNamespace = "default"
	}

	client, err := NewFromFlags(base.KubeconfigPath, base.SelectedContext, "")
	if err != nil {
		base.SyncError = err.Error()
		base.SyncedAt = time.Now().UTC()
		return base
	}

	nss, err := ListNamespaces(ctx, client)
	if err != nil {
		base.SyncError = fmt.Sprintf("list namespaces: %v", err)
		base.Namespaces = fallbackNamespaces(base.SelectedNamespace)
		base.SyncedAt = time.Now().UTC()
		return base
	}
	sort.Strings(nss)
	base.Namespaces = nss
	base.SelectedNamespace = pickNamespace(nss, base.SelectedNamespace)
	base.SyncError = ""
	base.SyncedAt = time.Now().UTC()
	return base
}

func configLoader(kubeconfigPath string) (clientcmd.ClientConfig, string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfigPath != "" {
		loadingRules.ExplicitPath = kubeconfigPath
	}
	path := kubeconfigPath
	if path == "" {
		path = DefaultKubeconfigPath()
		if path == "" {
			path = loadingRules.GetDefaultFilename()
		}
	}
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, nil)
	return loader, path, nil
}

func findContext(contexts []ContextOption, name string) *ContextOption {
	for i := range contexts {
		if contexts[i].Name == name {
			return &contexts[i]
		}
	}
	return nil
}

func pickNamespace(available []string, preferred string) string {
	if preferred != "" {
		for _, ns := range available {
			if ns == preferred {
				return ns
			}
		}
	}
	for _, ns := range available {
		if ns == "default" {
			return ns
		}
	}
	if len(available) > 0 {
		return available[0]
	}
	if preferred != "" {
		return preferred
	}
	return "default"
}

func fallbackNamespaces(preferred string) []string {
	if preferred == "" {
		return []string{"default"}
	}
	return []string{preferred}
}

// KubeconfigModTime returns the newest mod time among kubeconfig files on disk.
func KubeconfigModTime(kubeconfigPath string) (time.Time, error) {
	path := kubeconfigPath
	if path == "" {
		path = DefaultKubeconfigPath()
	}
	if path == "" {
		return time.Time{}, fmt.Errorf("kubeconfig path not found")
	}
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
