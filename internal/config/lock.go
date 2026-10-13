package config

import (
	"path/filepath"

	"github.com/meop/golie/lock"

	"github.com/meop/gotm/internal/store"
)

// AcquireLock takes gotm's lock over ~/.gotm and returns the function that
// releases it.
func AcquireLock() (func(), error) {
	dir, err := store.Dir()
	if err != nil {
		return nil, err
	}
	return lock.Acquire(filepath.Join(dir, ".lock"), "gotm")
}
