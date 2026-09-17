package kube

import (
	"os"
	"testing"
)

func TestSetNetworkProxy(t *testing.T) {
	t.Cleanup(func() {
		SetNetworkProxy("", "", "")
	})

	opts := SetNetworkProxy("http://127.0.0.1:8888", "http://127.0.0.1:8889", "localhost")
	if opts.HTTPProxy != "http://127.0.0.1:8888" {
		t.Fatalf("HTTPProxy = %q", opts.HTTPProxy)
	}
	if os.Getenv("HTTP_PROXY") != "http://127.0.0.1:8888" {
		t.Fatalf("HTTP_PROXY env = %q", os.Getenv("HTTP_PROXY"))
	}
	if os.Getenv("NO_PROXY") != "localhost" {
		t.Fatalf("NO_PROXY env = %q", os.Getenv("NO_PROXY"))
	}

	SetNetworkProxy("", "", "")
	if os.Getenv("HTTP_PROXY") != "" {
		t.Fatalf("expected HTTP_PROXY unset, got %q", os.Getenv("HTTP_PROXY"))
	}
}
