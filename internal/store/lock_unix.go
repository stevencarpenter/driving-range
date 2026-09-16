//go:build darwin || linux

package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// Lock reserves a state directory until unlock or process exit. Keep the lock file:
// unlinking it would allow another process to lock a different inode.
func Lock(stateDir string) (func(), error) {
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(stateDir, "runner.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("another golf runner owns this state directory: %w", err)
	}
	var once sync.Once
	return func() { once.Do(func() { syscall.Flock(int(f.Fd()), syscall.LOCK_UN); f.Close() }) }, nil
}
