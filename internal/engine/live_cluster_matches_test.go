package engine

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/glnreddy421/klew/internal/kube"
)

func TestLiveKlewLabMatchHealth(t *testing.T) {
	if os.Getenv("KLEW_LIVE_CLUSTER") == "" {
		t.Skip("set KLEW_LIVE_CLUSTER=1")
	}
	for _, query := range []string{"payment-api", ""} {
		query := query
		t.Run("query="+queryLabel(query), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			client, err := kube.NewFromFlags("", "docker-desktop", "klew-lab")
			if err != nil {
				t.Fatal(err)
			}
			col := &kube.Collector{Client: client}
			bundle, err := col.Collect(ctx, kube.CollectOptions{Namespace: "klew-lab", Query: query})
			if err != nil {
				t.Fatal(err)
			}
			type matchOut struct {
				Kind string `json:"kind"`
				Name string `json:"name"`
			}
			var matches []matchOut
			for _, m := range bundle.MatchedObjects {
				matches = append(matches, matchOut{m.Ref.Kind, m.Ref.Name})
			}
			sort.Slice(matches, func(i, j int) bool {
				if matches[i].Kind != matches[j].Kind {
					return matches[i].Kind < matches[j].Kind
				}
				return matches[i].Name < matches[j].Name
			})
			out := map[string]any{
				"matches": matches,
				"pods":    len(bundle.Pods),
				"nominal": WorkloadNominal(bundle),
				"active":  IncidentActive(bundle),
			}
			b, _ := json.MarshalIndent(out, "", "  ")
			t.Log(string(b))
		})
	}
}

func queryLabel(q string) string {
	if q == "" {
		return "ALL"
	}
	return q
}
