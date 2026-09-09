// Package version holds release metadata injected at build time.
package version

var (
	// Version is the semver release tag (e.g. 1.1.0).
	Version = "1.1.0"
	// Commit is the short git SHA at build time.
	Commit = "none"
	// Date is the UTC build timestamp (RFC3339).
	Date = "unknown"
)

// Info is the app build metadata exposed to the UI.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// Get returns current build metadata.
func Get() Info {
	return Info{
		Version: Version,
		Commit:  Commit,
		Date:    Date,
	}
}
