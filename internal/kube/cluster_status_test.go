package kube

import (
	"context"
	"testing"
)

func TestCollectClusterStatusNoContext(t *testing.T) {
	st := CollectClusterStatus(context.Background(), "", "")
	if st.Available {
		t.Fatal("expected unavailable without context")
	}
	if st.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestCollectClusterStatusInvalidKubeconfig(t *testing.T) {
	st := CollectClusterStatus(context.Background(), "/nonexistent/kubeconfig", "alpha")
	if st.Available {
		t.Fatal("expected unavailable for invalid kubeconfig")
	}
	if st.Error == "" {
		t.Fatal("expected error message")
	}
}
