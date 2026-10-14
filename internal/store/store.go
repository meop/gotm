// Package store names ~/.gotm. The layout inside it is golie's, the same one
// ghpm keeps in ~/.ghpm.
package store

import (
	"path/filepath"

	"github.com/meop/golie/home"
	"github.com/meop/golie/layout"
)

const testHomeEnv = "GOTM_TEST_HOME"

// UserHomeDir returns the user's real home in production and an explicitly
// isolated home in tests; see golie's home package for the guard.
func UserHomeDir() (string, error) { return home.Dir(testHomeEnv) }

// Layout is ~/.gotm: bin, vendor, extract and download, with the manifest and
// the lock beside them.
func Layout() (layout.Layout, error) {
	home, err := UserHomeDir()
	if err != nil {
		return layout.Layout{}, err
	}
	return layout.Layout{Root: filepath.Join(home, ".gotm")}, nil
}
