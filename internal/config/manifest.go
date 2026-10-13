package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/meop/golie/vers"

	"github.com/meop/gotm/internal/store"
)

// PinLatest is the pin of a toolchain that follows go's newest stable release.
const PinLatest = "latest"

// Toolchain is one go gotm was asked to keep. Version is the release its pin
// resolved to and is unpacked on disk, in go.dev's spelling ("go1.27.2"). Bin
// names the shims stamped for it.
type Toolchain struct {
	Version string   `json:"version"`
	Bin     []string `json:"bin,omitempty"`
}

// Manifest is everything gotm records: the toolchains that were added, each
// keyed by its pin — "latest", a line ("1.26") or one release ("1.26.4"). They
// sit side by side: every pin has its own shims, and nothing switches between
// them.
type Manifest struct {
	Toolchains map[string]Toolchain `json:"toolchain"`
}

// Pins lists the added pins, latest first and then newest line first.
func (m *Manifest) Pins() []string {
	pins := make([]string, 0, len(m.Toolchains))
	for pin := range m.Toolchains {
		pins = append(pins, pin)
	}
	slices.SortFunc(pins, func(a, b string) int {
		if a == PinLatest || b == PinLatest {
			if a == b {
				return 0
			}
			if a == PinLatest {
				return -1
			}
			return 1
		}
		if c := vers.CompareVersions(b, a); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	return pins
}

// Uses reports whether any toolchain other than the one pinned except is on
// release: a tree is shared by every pin that resolves to it, and is only
// removed once the last of them has moved off it.
func (m *Manifest) Uses(release, except string) bool {
	for pin, tc := range m.Toolchains {
		if pin != except && tc.Version == release {
			return true
		}
	}
	return false
}

func manifestPath() (string, error) {
	dir, err := store.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "manifest.json"), nil
}

// LoadManifest reads the manifest, or returns an empty one when there is none
// yet.
func LoadManifest() (*Manifest, error) {
	path, err := manifestPath()
	if err != nil {
		return nil, err
	}
	m := &Manifest{}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		if err := json.Unmarshal(data, m); err != nil {
			return nil, err
		}
	}
	if m.Toolchains == nil {
		m.Toolchains = map[string]Toolchain{}
	}
	return m, nil
}

// SaveManifest writes the manifest through a temp file and a rename, so a
// crash mid-write leaves the old one in place rather than a truncated one.
func SaveManifest(m *Manifest) error {
	path, err := manifestPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
