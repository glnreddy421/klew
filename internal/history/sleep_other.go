//go:build !darwin

package history

import "time"

// systemSleptSince is only known on macOS; elsewhere a late heartbeat stays an
// unknown gap.
func systemSleptSince(time.Time) bool { return false }
