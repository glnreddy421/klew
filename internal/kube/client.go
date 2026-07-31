package kube

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Client wraps kubernetes clientset and REST config.
type Client struct {
	Clientset kubernetes.Interface
	Config    *rest.Config
	Context   string
	Cluster   string
	User      string
	// Namespace is the effective namespace (never empty).
	Namespace string
	// ContextNamespace is the namespace stored on the kubeconfig context (may be empty).
	ContextNamespace string
}

// NewFromFlags builds a client from kubeconfig flags.
func NewFromFlags(kubeconfig, context, namespace string) (*Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	} else if home := homedir.HomeDir(); home != "" {
		loadingRules.ExplicitPath = filepath.Join(home, ".kube", "config")
	}

	overrides := &clientcmd.ConfigOverrides{}
	if context != "" {
		overrides.CurrentContext = context
	}
	if namespace != "" {
		overrides.Context.Namespace = namespace
	}

	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	restConfig, err := cfg.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}
	cs, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}

	raw, err := cfg.RawConfig()
	if err != nil {
		return nil, fmt.Errorf("read raw config: %w", err)
	}
	currentCtx := raw.CurrentContext
	ctxCfg, ok := raw.Contexts[currentCtx]
	if !ok {
		return nil, fmt.Errorf("context %q not found", currentCtx)
	}
	ns := namespace
	if ns == "" {
		ns = ctxCfg.Namespace
	}
	if ns == "" {
		ns = "default"
	}

	return &Client{
		Clientset:        cs,
		Config:           restConfig,
		Context:          currentCtx,
		Cluster:          ctxCfg.Cluster,
		User:             ctxCfg.AuthInfo,
		Namespace:        ns,
		ContextNamespace: ctxCfg.Namespace,
	}, nil
}

// ContextInfo returns display fields for klew ctx.
func (c *Client) ContextInfo() (context, cluster, user, namespace string) {
	return c.Context, c.Cluster, c.User, c.Namespace
}

// DefaultKubeconfigPath returns the default kubeconfig path if it exists.
func DefaultKubeconfigPath() string {
	if p := os.Getenv("KUBECONFIG"); p != "" {
		return p
	}
	if home := homedir.HomeDir(); home != "" {
		return filepath.Join(home, ".kube", "config")
	}
	return ""
}
