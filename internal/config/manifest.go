// Package config holds gotm's manifest and its lock over ~/.gotm.
package config

import (
	"github.com/meop/golie/manifest"

	"github.com/meop/gotm/internal/store"
)

// The manifest is golie's, the one ghpm keeps: gotm is ghpm with a single
// package, so its installs are recorded the same way — "go" for the toolchain
// that follows go's newest release, "go@1.26" for one held to a line and
// "go@1.26.4" for one held to a release.
type (
	Entry    = manifest.Entry
	Manifest = manifest.Manifest
)

// Package is the one package gotm manages, and Source where its releases
// come from, as the manifest's repo map records them.
const (
	Package = "go"
	Source  = "go.dev/dl"
)

// PinLatest is what a toolchain that follows go's newest stable release is
// asked for by, and the pin level it is recorded with.
const PinLatest = manifest.PinLatest

// LoadManifest reads the manifest, or returns an empty one when there is none
// yet.
func LoadManifest() (*Manifest, error) {
	l, err := store.Layout()
	if err != nil {
		return nil, err
	}
	return manifest.Load(l.ManifestPath())
}

// SaveManifest writes the manifest; see golie's manifest.Save.
func SaveManifest(m *Manifest) error {
	l, err := store.Layout()
	if err != nil {
		return err
	}
	return manifest.Save(m, l.ManifestPath())
}
