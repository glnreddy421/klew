package kube

import (
	"context"

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
