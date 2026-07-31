package draincfg

import (
	"fmt"

	"github.com/kloudmate/drain3"
)

// NewMiner builds a production Drain3 TemplateMiner with masking for transient
// cloud / Kubernetes identifiers (IPs, pod names, UUIDs, timestamps).
func NewMiner() (*drain3.TemplateMiner, error) {
	tm, err := drain3.New(
		drain3.WithSimTh(0.5),
		drain3.WithDepth(4),
		drain3.WithMaxChildren(100),
		drain3.WithMaxClusters(2000),
		drain3.WithExtraDelimiters("=", ":"),
		drain3.WithParametrizeNumericTokens(true),

		// IPv4
		drain3.WithMasking(
			`\b(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\b`,
			"IP",
		),
		// IPv6 (compressed + full forms)
		drain3.WithMasking(
			`\b(?:(?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}|(?:[0-9a-fA-F]{1,4}:){1,7}:|(?:[0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|::(?:[0-9a-fA-F]{1,4}:){0,5}[0-9a-fA-F]{1,4}|::)\b`,
			"IPV6",
		),
		// UUID
		drain3.WithMasking(
			`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`,
			"UUID",
		),
		// ISO8601 / RFC3339 timestamps
		drain3.WithMasking(
			`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?`,
			"TS",
		),
		// K8s pod / ReplicaSet instance names: name-hash-suffix (e.g. auth-service-7db86bb96c-xzqpw)
		drain3.WithMasking(
			`\b[a-z0-9]+(?:-[a-z0-9]+)*-[a-f0-9]{8,10}-[a-z0-9]{5}\b`,
			"POD",
		),
	)
	if err != nil {
		return nil, fmt.Errorf("drain3.New: %w", err)
	}
	return tm, nil
}
