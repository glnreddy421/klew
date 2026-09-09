package kube

import "testing"

func TestKubectlResourceArg(t *testing.T) {
	if got := KubectlResourceArg("", "pods"); got != "pods" {
		t.Fatalf("core resource = %q", got)
	}
	if got := KubectlResourceArg("policies.kyverno.io", "validatingpolicies"); got != "validatingpolicies.policies.kyverno.io" {
		t.Fatalf("extension resource = %q", got)
	}
}

func TestFormatKubectlGetCommand(t *testing.T) {
	cmd := FormatKubectlGetCommand(ResourceManifestRequest{
		Context:   "docker-desktop",
		Namespace: "klew-lab",
		Name:      "redis",
	}, "pods")
	want := "kubectl --context docker-desktop -n klew-lab get pods redis -o yaml"
	if cmd != want {
		t.Fatalf("command = %q want %q", cmd, want)
	}

	clusterCmd := FormatKubectlGetCommand(ResourceManifestRequest{
		Context:       "docker-desktop",
		Name:          "deny-latest-tag",
		ClusterScoped: true,
	}, "validatingpolicies.policies.kyverno.io")
	if clusterCmd != "kubectl --context docker-desktop get validatingpolicies.policies.kyverno.io deny-latest-tag -o yaml" {
		t.Fatalf("cluster command = %q", clusterCmd)
	}
}

func TestShellQuote(t *testing.T) {
	if shellQuote("klew-lab") != "klew-lab" {
		t.Fatal("simple token should not be quoted")
	}
	if shellQuote("klew lab") != "'klew lab'" {
		t.Fatal("spaces should be quoted")
	}
}
