package cli

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/meop/golie/layout"
	"github.com/meop/golie/pointer"
	"github.com/meop/gotm/internal/config"
	"github.com/meop/gotm/internal/godl"
	"github.com/meop/gotm/internal/store"
)

// release is a go release whose archive is already in the download cache: a go
// tree that says its version and ships no commands, so nothing is stamped
func release(t *testing.T, l layout.Layout, version string) godl.Release {
	t.Helper()
	downloads, err := l.DownloadDir(config.Source, version)
	if err != nil {
		t.Fatal(err)
	}
	name := "go" + version + ".tar.gz"
	f, err := os.Create(filepath.Join(downloads, name))
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.New()
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	body := []byte("go" + version)
	for _, hdr := range []*tar.Header{
		{Name: "go/bin/", Typeflag: tar.TypeDir, Mode: 0755},
		{Name: "go/VERSION", Typeflag: tar.TypeReg, Mode: 0644, Size: int64(len(body))},
	} {
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if hdr.Typeflag == tar.TypeReg {
			if _, err := tw.Write(body); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(downloads, name))
	if err != nil {
		t.Fatal(err)
	}
	h.Write(b)
	return godl.Release{Version: "go" + version, Stable: true, Files: []godl.File{
		{Filename: name, OS: runtime.GOOS, Arch: runtime.GOARCH, Kind: "archive", SHA256: hex.EncodeToString(h.Sum(nil))},
	}}
}

func TestApplyPointsLatestAtEachReleaseAndLeavesNoOldOne(t *testing.T) {
	t.Setenv("GOTM_TEST_HOME", t.TempDir())
	l, err := store.Layout()
	if err != nil {
		t.Fatal(err)
	}
	m := save(t, map[string]config.Entry{})

	for _, version := range []string{"1.27.1", "1.27.2"} {
		c := change{key: "go", pin: "latest", prev: m.Extracts["go"], target: release(t, l, version)}
		if err := apply(context.Background(), m, l, c); err != nil {
			t.Fatal(err)
		}
		tree, ok := pointer.Target(l.LatestPath("go"))
		if !ok || tree != l.VersionPath("go", version) {
			t.Fatalf("after %s latest names %s, %v", version, tree, ok)
		}
		if got, err := os.ReadFile(filepath.Join(tree, "VERSION")); err != nil || string(got) != "go"+version {
			t.Errorf("after %s the tree says %q, %v", version, got, err)
		}
		if m.Extracts["go"].Version != version {
			t.Errorf("the manifest says %s after %s", m.Extracts["go"].Version, version)
		}
	}
	entries, err := os.ReadDir(l.ExtractPath("go"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, ",") != "1.27.2,latest" {
		t.Errorf("the install holds %v", names)
	}
}
