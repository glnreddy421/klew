package kube

import (
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

func TestIsAuthConnectionError(t *testing.T) {
	if !IsAuthConnectionError(apierrors.NewUnauthorized("no way")) {
		t.Fatal("expected unauthorized")
	}
	if !IsAuthConnectionError(errors.New("running aws eks get-token: token expired")) {
		t.Fatal("expected eks token expired")
	}
	if IsAuthConnectionError(errors.New("connection refused")) {
		t.Fatal("connection refused is not auth")
	}
}

func TestFormatConnectionErrorAuth(t *testing.T) {
	msg := FormatConnectionError("api server", apierrors.NewUnauthorized("credentials"))
	if msg == "" || msg == "api server: credentials" {
		t.Fatalf("expected auth wrapper, got %q", msg)
	}
}

func TestFormatConnectionErrorGeneric(t *testing.T) {
	err := apierrors.NewServiceUnavailable("busy")
	msg := FormatConnectionError("api server", err)
	if msg != "api server: "+err.Error() {
		t.Fatalf("unexpected: %q", msg)
	}
}
