package mcpcli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/glnreddy421/klew/v2/internal/engine"
)

// The extensions-examples folder is what users copy from, so every document
// must load and every sample must give the result its comment promises.
func TestExtensionExamples(t *testing.T) {
	const dir = "../../extensions-examples"
	reg := engine.LoadExtensions(dir)
	if len(reg.Warnings) != 0 {
		t.Fatalf("examples do not load cleanly: %v", reg.Warnings)
	}
	if got := len(reg.UserRuleNames()); got != 10 {
		t.Errorf("rules loaded = %d, want 10: %v", got, reg.UserRuleNames())
	}
	for _, tc := range []struct {
		rule, file, namespace string
		match                 []string
	}{
		{"quota-exhausted", "resourcequotas.yaml", "", []string{"MATCH    ResourceQuota/team-quota", "fact: team-quota is full for: pods", "1 of 2 matched"}},
		{"certificate-not-ready", "certificates.yaml", "", []string{"MATCH    Certificate/api-tls", "1 of 2 matched"}},
		{"certificate-renewal-failing", "certificates.yaml", "", []string{"fact: api-tls: 3 failed issuance attempts", "1 of 2 matched"}},
		{"kyverno-policy-not-ready", "clusterpolicies.yaml", "", []string{"MATCH    ClusterPolicy/restrict-registries", "1 of 2 matched"}},
		{"external-dependents", "services.yaml", "holdout", []string{"MATCH    Service/billing-proxy", "1 of 4 matched"}},
	} {
		var out bytes.Buffer
		opts := dryRunOptions{Rule: tc.rule, ObjectFile: dir + "/samples/" + tc.file, Namespace: tc.namespace}
		if err := dryRun(&out, reg, opts); err != nil {
			t.Errorf("%s: %v", tc.rule, err)
			continue
		}
		for _, want := range tc.match {
			if !strings.Contains(out.String(), want) {
				t.Errorf("%s: output lacks %q:\n%s", tc.rule, want, out.String())
			}
		}
	}
}
