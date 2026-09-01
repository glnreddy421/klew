// Package version holds release metadata injected at build time.
package version

var (
	// Version is the semver release tag (e.g. 0.1.0).
	Version = "0.1.7"
	// Commit is the short git SHA at build time.
	Commit = "none"
	// Date is the UTC build timestamp (RFC3339).
	Date = "unknown"
)
