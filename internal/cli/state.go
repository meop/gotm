package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/meop/golie/manifest"
	"github.com/meop/golie/shim"

	"github.com/meop/gotm/internal/config"
	"github.com/meop/gotm/internal/godl"
	"github.com/meop/gotm/internal/store"
)

// parsePin turns an argument into what a toolchain is pinned to: nothing for
// the one that follows go's newest release, a line ("1.26") or one release
// ("1.26.4"). gotm's one package is go, so the argument is ghpm's with the
// name left off — "1.26" for ghpm's "go@1.26", "latest" for a bare "go" — and
// the key itself is taken too, since it is what list prints.
func parsePin(arg string) (string, error) {
	typed := strings.ToLower(strings.TrimSpace(arg))
	pin := strings.TrimPrefix(strings.TrimPrefix(typed, pkgGo), "@")
	if typed == pkgGo || pin == config.PinLatest {
		return "", nil
	}
	if pin == "" || pin[0] < '0' || pin[0] > '9' {
		return "", errors.New("invalid version: expected latest, a line like 1.26, or a release like 1.26.4")
	}
	return pin, nil
}

// keyOf is the manifest key of the toolchain pinned to pin: "go", "go@1.26".
func keyOf(pin string) string { return manifest.Key(pkgGo, pin) }

// stamper is where gotm's shims go and what they may point at: a shim that
// runs something under the extract dir is one gotm stamped.
func stamper() (shim.Stamper, error) {
	l, err := store.Layout()
	if err != nil {
		return shim.Stamper{}, err
	}
	binDir, err := l.BinDir()
	if err != nil {
		return shim.Stamper{}, err
	}
	malletDir, err := l.ShimDir()
	if err != nil {
		return shim.Stamper{}, err
	}
	return shim.Stamper{Tool: "gotm", BinDir: binDir, MalletDir: malletDir, Owned: l.ExtractsDir()}, nil
}

// isShim is tidy's question of a file in the bin dir.
func isShim(path string) bool {
	s, err := stamper()
	return err == nil && s.IsShim(path)
}

// treeBins lists the commands a go tree ships, as bin keys: their paths
// inside the tree, named as the host names them ("bin/go", "bin/go.exe").
func treeBins(tree string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(tree, "bin"))
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, e := range entries {
		if !e.IsDir() {
			keys = append(keys, "bin/"+e.Name())
		}
	}
	slices.Sort(keys)
	return keys, nil
}

// cached reports whether path is already the file sha256 names, so a release
// another pin downloaded is not downloaded again and a damaged copy is.
func cached(path, sha string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false
	}
	return strings.EqualFold(hex.EncodeToString(h.Sum(nil)), sha)
}

// findKeys is the reading commands' filter, for list and outdated: an
// installed toolchain is kept when some term starts what it is pinned to or
// the release it is on (godl.StartsWith), so "1.27" lists go@1.27.2 and the
// go that follows latest while it is on 1.27.x. With no terms it is every
// key. Reading is wider than acting on purpose: sync and remove take a pin
// whole.
func findKeys(m *config.Manifest, terms []string) []string {
	keys := m.Keys()
	if len(terms) == 0 {
		return keys
	}
	return slices.DeleteFunc(keys, func(key string) bool {
		_, pin, _ := manifest.ParseKey(key)
		return !slices.ContainsFunc(terms, func(term string) bool {
			want, err := parsePin(term)
			if err != nil {
				return false
			}
			if want == "" {
				return pin == ""
			}
			return godl.StartsWith(pin, want) || godl.StartsWith(m.Extracts[key].Version, want)
		})
	})
}

// entryRows is one row per shim of an install, behind the leading columns
// prefix gives: ghpm's list rows, where the type is always bin.
func entryRows(rows [][]string, prefix []string, e config.Entry) [][]string {
	names := make([]string, 0, len(e.Bin))
	for name := range e.Bin {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		rows = append(rows, append(slices.Clone(prefix), "bin", name))
	}
	return rows
}

// resolver asks go.dev which release a pin names, fetching each of the two
// release lists at most once however many pins a command resolves.
type resolver struct {
	short, all []godl.Release
}

func (r *resolver) resolve(ctx context.Context, pin string) (godl.Release, error) {
	list := &r.short
	if godl.NeedsAll(pin) {
		list = &r.all
	}
	if *list == nil {
		releases, err := godl.List(ctx, godl.NeedsAll(pin))
		if err != nil {
			return godl.Release{}, err
		}
		*list = releases
	}
	return godl.Resolve(*list, pin)
}
