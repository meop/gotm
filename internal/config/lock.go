package config

import (
	"github.com/meop/golie/lock"

	"github.com/meop/gotm/internal/store"
)

// AcquireLock takes gotm's lock over ~/.gotm and returns the function that
// releases it.
func AcquireLock() (func(), error) {
	l, err := store.Layout()
	if err != nil {
		return nil, err
	}
	return lock.Acquire(l.LockPath(), "gotm")
}
