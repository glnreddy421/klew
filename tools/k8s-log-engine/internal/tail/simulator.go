package tail

import (
	"context"
	"fmt"
	"io"
	"time"
)

// Simulator emits a realistic mix of JSON and logfmt Kubernetes-style logs.
type Simulator struct {
	// Interval between lines. Zero → 5ms.
	Interval time.Duration
	// Total lines to emit. Zero → run until ctx cancel.
	Count int
	// Clock anchors generated timestamps (nil → time.Now).
	Clock func() time.Time
}

// Run implements Source.
func (s *Simulator) Run(ctx context.Context, out chan<- Event) error {
	interval := s.Interval
	if interval <= 0 {
		interval = 5 * time.Millisecond
	}
	clock := s.Clock
	if clock == nil {
		clock = time.Now
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	templates := []func(n int, now time.Time) string{
		func(n int, now time.Time) string {
			return fmt.Sprintf(
				`{"time":%q,"level":"info","msg":"health check ok","pod":"payment-api-7db86bb96c-xzqpw","status":"ready"}`,
				now.UTC().Format(time.RFC3339Nano),
			)
		},
		func(n int, now time.Time) string {
			ip := fmt.Sprintf("10.0.%d.%d", (n/256)%256, n%256)
			return fmt.Sprintf(
				`{"ts":%q,"level":"error","msg":"dial tcp %s:5432: connect: connection refused","pod":"auth-service-7db86bb96c-xzqpw"}`,
				now.UTC().Format(time.RFC3339),
				ip,
			)
		},
		func(n int, now time.Time) string {
			uid := fmt.Sprintf("550e8400-e29b-41d4-a716-%012d", n%1_000_000_000_000)
			return fmt.Sprintf(
				`time=%s level=error msg="payment failed" userId=%s error.code=TIMEOUT pod=payment-worker-5f9c8d7b6c-abcde`,
				now.UTC().Format(time.RFC3339),
				uid,
			)
		},
		func(n int, now time.Time) string {
			return fmt.Sprintf(
				`{"time":%q,"level":"warn","msg":"upstream timeout after 3s","pod":"checkout-6c8d9f4b2a-mnpqr","target":"https://example.internal/v1"}`,
				now.UTC().Format(time.RFC3339),
			)
		},
		func(n int, now time.Time) string {
			return fmt.Sprintf(
				`time=%s level=info msg="GET /health 200" latency_ms=%d pod=frontend-84b7c9d5f-hjklo`,
				now.UTC().Format(time.RFC3339),
				10+(n%40),
			)
		},
	}

	n := 0
	for {
		if s.Count > 0 && n >= s.Count {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			line := templates[n%len(templates)](n, clock())
			ev := decodeLine(line)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case out <- ev:
				n++
			}
		}
	}
}

// Ensure Simulator satisfies Source.
var _ Source = (*Simulator)(nil)

// NopCloser wraps an io.Reader without a Close method.
type NopCloser struct{ io.Reader }

func (NopCloser) Close() error { return nil }
