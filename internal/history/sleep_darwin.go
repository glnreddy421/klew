//go:build darwin

package history

import (
	"time"

	"golang.org/x/sys/unix"
)

// systemSleptSince reports whether macOS woke from sleep after t, using the
// kernel's last wake time.
func systemSleptSince(t time.Time) bool {
	tv, err := unix.SysctlTimeval("kern.waketime")
	if err != nil || tv == nil || tv.Sec == 0 {
		return false
	}
	return time.Unix(tv.Sec, int64(tv.Usec)*1000).After(t)
}
