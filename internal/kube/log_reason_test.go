package kube

import "testing"

func TestClassifyLogReasonRedisStartupWarningIsNotTimeout(t *testing.T) {
	line := "# Warning: no config file specified, using the default config. In order to specify a config file use redis-server /path/to/redis.conf"
	if got := classifyLogReason(line); got == "Timeout" {
		t.Fatalf("redis startup warning classified as %q", got)
	}
	if got := classifyLogReason("dial tcp 10.0.0.7:6379: connect: connection timed out"); got != "Timeout" {
		t.Fatalf("timed-out dial classified as %q, want Timeout", got)
	}
}
