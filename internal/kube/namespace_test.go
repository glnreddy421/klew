package kube

import "testing"

func TestRequireNamespaceScope(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		flagNS  string
		ctxNS   string
		allNS   bool
		wantNS  string
		wantAll bool
		wantErr bool
	}{
		{name: "flag", flagNS: "klew-lab", wantNS: "klew-lab"},
		{name: "context ignored", ctxNS: "prod", wantErr: true},
		{name: "flag beats context", flagNS: "klew-lab", ctxNS: "prod", wantNS: "klew-lab"},
		{name: "all namespaces", allNS: true, wantAll: true},
		{name: "implicit default rejected", wantErr: true},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ns, all, err := RequireNamespaceScope(tc.flagNS, tc.ctxNS, tc.allNS)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ns != tc.wantNS || all != tc.wantAll {
				t.Fatalf("got ns=%q all=%v, want ns=%q all=%v", ns, all, tc.wantNS, tc.wantAll)
			}
		})
	}
}

func TestRequireNamespaceOrError(t *testing.T) {
	t.Parallel()
	_, _, err := RequireNamespaceOrError("", false)
	if err == nil {
		t.Fatal("expected error without -n")
	}
}
