package engine

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/glnreddy421/klew/internal/kube"
)

// Live cluster probe — run with: go test ./internal/engine -run TestLiveKlewLabPaymentAPI -v
func TestLiveKlewLabPaymentAPI(t *testing.T) {
	if os.Getenv("KLEW_LIVE_CLUSTER") == "" {
		t.Skip("set KLEW_LIVE_CLUSTER=1 to probe local cluster")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	client, err := kube.NewFromFlags("", "docker-desktop", "klew-lab")
	if err != nil {
		t.Fatal(err)
	}
	col := &kube.Collector{Client: client}
	bundle, err := col.Collect(ctx, kube.CollectOptions{Namespace: "klew-lab", Query: "payment-api"})
	if err != nil {
		t.Fatal(err)
	}
	type podOut struct {
		Name  string `json:"name"`
		Phase string `json:"phase"`
		Ready bool   `json:"ready"`
	}
	var pods []podOut
	for _, p := range bundle.Pods {
		pods = append(pods, podOut{p.Name, p.Phase, p.Ready})
	}
	out := map[string]any{
		"matched": len(bundle.MatchedObjects),
		"pods":    pods,
		"nominal": WorkloadNominal(bundle),
		"active":  IncidentActive(bundle),
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	t.Log(string(b))
	if !WorkloadNominal(bundle) {
		t.Fatalf("expected nominal workload for payment-api")
	}
	if IncidentActive(bundle) {
		t.Fatalf("expected no active incident for payment-api")
	}
}
