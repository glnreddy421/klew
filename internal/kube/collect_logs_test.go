package kube

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/client-go/kubernetes/fake"

	"github.com/glnreddy421/klew/v2/internal/model"
)

func TestCollectLogsKeepsPodOrder(t *testing.T) {
	t.Parallel()
	var bundle model.EvidenceBundle
	var want []string
	for i := range maxConcurrentLogFetches * 3 {
		name := fmt.Sprintf("api-%02d", i)
		bundle.Pods = append(bundle.Pods, model.PodSummary{Name: name, Containers: []model.ContainerStatus{{Name: "app", RestartCount: int32(i % 2)}}})
		want = append(want, name)
	}
	col := &Collector{Client: &Client{Clientset: fake.NewSimpleClientset()}}
	col.collectLogs(context.Background(), "shop", 10, &bundle)

	if len(bundle.Logs) != len(want) || len(bundle.Warnings) != 0 {
		t.Fatalf("logs = %d, warnings = %v", len(bundle.Logs), bundle.Warnings)
	}
	for i, l := range bundle.Logs {
		if l.PodName != want[i] {
			t.Fatalf("logs[%d] = %s, want %s", i, l.PodName, want[i])
		}
	}
	if len(bundle.PreviousLogs) != len(want)/2 {
		t.Fatalf("previous logs = %d, want %d (restarted pods only)", len(bundle.PreviousLogs), len(want)/2)
	}
}

func TestNewFromFlagsSetsRequestLimits(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config")
	cfg := `apiVersion: v1
kind: Config
current-context: lab
clusters: [{name: lab, cluster: {server: "https://127.0.0.1:6443"}}]
contexts: [{name: lab, context: {cluster: lab, user: lab}}]
users: [{name: lab, user: {}}]
`
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := NewFromFlags(path, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if c.Config.QPS != clientQPS || c.Config.Burst != clientBurst {
		t.Fatalf("QPS/Burst = %v/%d", c.Config.QPS, c.Config.Burst)
	}
}
