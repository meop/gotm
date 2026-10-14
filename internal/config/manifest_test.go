package config

import (
	"os"
	"path/filepath"
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
	if err != nil || m.Extracts == nil || len(m.Extracts) != 0 {
		t.Fatalf("fresh manifest = %+v, %v", m, err)
	}
}

// the manifest is ghpm's, kept at ~/.gotm/manifest.json
func TestManifestRoundTrip(t *testing.T) {
	home := withHome(t)
	want := &Manifest{
		Repos: map[string]string{Package: Source},
		Extracts: map[string]Entry{
			"go":      {Pin: "latest", Version: "1.27.2", Bin: map[string]string{"go": "bin/go"}},
			"go@1.26": {Pin: "minor", Version: "1.26.9", Bin: map[string]string{"go@1.26": "bin/go"}},
		},
	}
	if err := SaveManifest(want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadManifest()
	if err != nil || len(got.Extracts) != 2 || got.Extracts["go@1.26"].Version != "1.26.9" || got.Repos["go"] != "go.dev/dl" {
		t.Fatalf("loaded %+v, %v", got, err)
	}
	dir := filepath.Join(home, ".gotm")
	if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
		t.Errorf("the manifest is not at %s: %v", dir, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "manifest.json.tmp")); err == nil {
		t.Error("the temp file a save writes through was left behind")
	}
}

func TestStoreRefusesTheRealHomeUnderTest(t *testing.T) {
	t.Setenv("GOTM_TEST_HOME", "")
	if _, err := LoadManifest(); err == nil {
		t.Fatal("a test with no GOTM_TEST_HOME reached for the real home")
	}
}
