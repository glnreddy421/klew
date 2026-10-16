//go:build windows

package kube

import "os"

// Windows session dirs live under the per-user profile (cache, home, or the
// per-user TEMP), so ownership is already scoped to the current user.
func ownedByCurrentUser(os.FileInfo) bool {
	return true
}
