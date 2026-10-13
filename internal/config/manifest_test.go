package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func withHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("GOTM_TEST_HOME", home)
	return home
}

func TestManifestStartsEmpty(t *testing.T) {
	withHome(t)
	m, err := LoadManifest()
	if err != nil || m.Toolchains == nil || len(m.Toolchains) != 0 {
		t.Fatalf("fresh manifest = %+v, %v", m, err)
	}
}

func TestManifestRoundTrip(t *testing.T) {
	home := withHome(t)
	want := &Manifest{Toolchains: map[string]Toolchain{
		"latest": {Version: "go1.27.2", Bin: []string{"go", "gofmt"}},
		"1.26":   {Version: "go1.26.9", Bin: []string{"go1.26", "gofmt1.26"}},
	}}
	if err := SaveManifest(want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadManifest()
	if err != nil || len(got.Toolchains) != 2 || got.Toolchains["1.26"].Version != "go1.26.9" {
		t.Fatalf("loaded %+v, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".gotm", "manifest.json.tmp")); err == nil {
		t.Error("the temp file a save writes through was left behind")
	}
}

func TestPinsOrder(t *testing.T) {
	m := &Manifest{Toolchains: map[string]Toolchain{"1.9": {}, "1.26.4": {}, "latest": {}, "1.26": {}, "1.27": {}}}
	want := []string{"latest", "1.27", "1.26.4", "1.26", "1.9"}
	if got := m.Pins(); !slices.Equal(got, want) {
		t.Errorf("Pins() = %v, want %v", got, want)
	}
}

func TestUses(t *testing.T) {
	m := &Manifest{Toolchains: map[string]Toolchain{
		"latest": {Version: "go1.27.2"},
		"1.27":   {Version: "go1.27.2"},
		"1.26":   {Version: "go1.26.9"},
	}}
	if !m.Uses("go1.27.2", "latest") {
		t.Error("1.27 is on go1.27.2 too, and Uses did not say so")
	}
	if m.Uses("go1.26.9", "1.26") {
		t.Error("nothing but 1.26 is on go1.26.9, and Uses said something was")
	}
}

func TestStoreRefusesTheRealHomeUnderTest(t *testing.T) {
	t.Setenv("GOTM_TEST_HOME", "")
	if _, err := LoadManifest(); err == nil {
		t.Fatal("a test with no GOTM_TEST_HOME reached for the real home")
	}
}
