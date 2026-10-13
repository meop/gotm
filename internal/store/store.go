// Package store names the ~/.gotm directory layout.
package store

import (
	"os"
	"path/filepath"

	"github.com/meop/golie/home"
)

const testHomeEnv = "GOTM_TEST_HOME"

// UserHomeDir returns the user's real home in production and an explicitly
// isolated home in tests; see golie's home package for the guard.
func UserHomeDir() (string, error) { return home.Dir(testHomeEnv) }

// Dir returns the root ~/.gotm directory path without creating it.
func Dir() (string, error) {
	home, err := UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gotm"), nil
}

func subDir(mkdir bool, elems ...string) (string, error) {
	base, err := Dir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(append([]string{base}, elems...)...)
	if mkdir {
		return dir, os.MkdirAll(dir, 0755)
	}
	return dir, nil
}

// BinDir holds gotm itself and the shims it stamps for the toolchain's
// commands — everything meant to be on the user's PATH.
func BinDir() (string, error) {
	return subDir(true, "bin")
}

// ShimDir holds sheesh's kebab stamper: a tool gotm needs to work rather than
// one it installs for you, so it sits under vendor and off PATH.
func ShimDir() (string, error) {
	return subDir(true, "vendor", "sheesh")
}

// ToolchainsDir is where Go trees are unpacked, one directory per release.
func ToolchainsDir() (string, error) {
	return subDir(false, "toolchain")
}

// ToolchainDir is the GOROOT of one release, named as go.dev names it
// ("go1.27.2"). It is not created here: its existence is what says the
// release is installed.
func ToolchainDir(release string) (string, error) {
	base, err := ToolchainsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, release), nil
}

// DownloadDir holds an archive only while it is being fetched and unpacked.
func DownloadDir() (string, error) {
	return subDir(true, "download")
}
