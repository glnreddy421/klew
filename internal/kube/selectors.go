package kube

import (
	"context"
	"os"
	"os/exec"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListNamespaces returns active namespace names.
func ListNamespaces(ctx context.Context, c *Client) ([]string, error) {
	list, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(list.Items))
	for _, ns := range list.Items {
		if ns.Status.Phase == "Terminating" {
			continue
		}
		out = append(out, ns.Name)
	}
	return out, nil
}

// ListNamespacesViaKubectl lists namespaces using the kubectl binary. This matches
// what works in the embedded login shell when client-go auth/env differs.
func ListNamespacesViaKubectl(ctx context.Context, kubeconfigPath, contextName, clusterVersion string) ([]string, error) {
	kubectl, _ := ResolveKubectlForCluster(clusterVersion)
	if kubectl == "" {
		return nil, exec.ErrNotFound
	}

	args := []string{"get", "ns", "-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\n\"}{end}"}
	if contextName != "" {
		args = append([]string{"--context", contextName}, args...)
	}
	kubeconfigPath = strings.TrimSpace(kubeconfigPath)
	if kubeconfigPath != "" && !strings.Contains(kubeconfigPath, string(os.PathListSeparator)) {
		args = append([]string{"--kubeconfig", kubeconfigPath}, args...)
	}

	cmd := exec.CommandContext(ctx, kubectl, args...)
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	outNS := make([]string, 0, len(lines))
	for _, ns := range lines {
		ns = strings.TrimSpace(ns)
		if ns != "" {
			outNS = append(outNS, ns)
		}
	}
	if len(outNS) == 0 {
		return nil, exec.ErrNotFound
	}
	return outNS, nil
}
