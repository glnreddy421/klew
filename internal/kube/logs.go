package kube

import (
	"bufio"
	"context"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/glnreddy421/klew/internal/model"
)

func (col *Collector) fetchLogs(ctx context.Context, ns, pod, container string, previous bool, tailLines int) (model.LogRecord, error) {
	opts := &corev1.PodLogOptions{
		Container: container,
		Previous:  previous,
		TailLines: int64Ptr(int64(tailLines)),
	}
	req := col.Client.Clientset.CoreV1().Pods(ns).GetLogs(pod, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return model.LogRecord{}, err
	}
	defer stream.Close()

	var lines []string
	sc := bufio.NewScanner(stream)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	truncated := false
	if len(lines) > tailLines {
		lines = lines[len(lines)-tailLines:]
		truncated = true
	}
	return model.LogRecord{
		PodName:       pod,
		ContainerName: container,
		Previous:      previous,
		Lines:         lines,
		Truncated:     truncated,
		CollectedAt:   time.Now().UTC(),
	}, sc.Err()
}

func int64Ptr(v int64) *int64 { return &v }

// LogSpikeDetect returns true if error-like lines exceed threshold in last N lines.
func LogSpikeDetect(lines []string, threshold int) bool {
	if threshold <= 0 {
		threshold = 3
	}
	count := 0
	start := 0
	if len(lines) > 50 {
		start = len(lines) - 50
	}
	for _, l := range lines[start:] {
		lower := strings.ToLower(l)
		if strings.Contains(lower, "error") || strings.Contains(lower, "fatal") || strings.Contains(lower, "panic") {
			count++
		}
	}
	return count >= threshold
}

var _ = metav1.GetOptions{}
