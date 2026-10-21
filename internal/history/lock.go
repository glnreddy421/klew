package history

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrLocked means another Klew process already records this cluster.
var ErrLocked = errors.New("another Klew window is recording this context")

// Lock is an exclusive, process-wide lock on one cluster key. The OS drops it
// when the process exits, so a crashed window never blocks recording.
type Lock struct {
	f *os.File
}

// AcquireLock takes the recording lock for key under dir without blocking.
// Returns ErrLocked when another process holds it.
func AcquireLock(dir, key string) (*Lock, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, lockFileName(key)), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := tryLockFile(f); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &Lock{f: f}, nil
}

// Release drops the lock.
func (l *Lock) Release() {
	if l == nil || l.f == nil {
		return
	}
	_ = unlockFile(l.f)
	_ = l.f.Close()
	l.f = nil
}
