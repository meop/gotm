package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/meop/golie/shim"
	"github.com/meop/gotm/internal/config"
	"github.com/meop/gotm/internal/godl"
	"github.com/meop/gotm/internal/store"
)

// normalizePin turns what was typed into the pin a toolchain is keyed by:
// "latest" for nothing or latest, and otherwise the version without go.dev's
// "go" prefix, so "go1.26" and "1.26" are the same toolchain.
func normalizePin(arg string) (string, error) {
	pin := strings.ToLower(strings.TrimSpace(arg))
	if pin == "" || pin == config.PinLatest {
		return config.PinLatest, nil
	}
	pin = godl.Display(pin)
	if pin == "" || pin[0] < '0' || pin[0] > '9' {
		return "", fmt.Errorf("invalid version %q: expected latest, a line like 1.26, or a release like 1.26.4", arg)
	}
	return pin, nil
}

// shimName is what a toolchain's command is called on PATH: its own name for
// the toolchain following latest, and its name with the pin for any other, so
// "go" sits beside "go1.26" and "go1.26.4" and none replaces another.
func shimName(bin, pin string) string {
	if pin == config.PinLatest {
		return bin
	}
	return bin + pin
}

// stamper is where gotm's shims go and what they may point at: a shim that
// runs something under the toolchain dir is one gotm stamped.
func stamper() (shim.Stamper, error) {
	binDir, err := store.BinDir()
	if err != nil {
		return shim.Stamper{}, err
	}
	kebabDir, err := store.ShimDir()
	if err != nil {
		return shim.Stamper{}, err
	}
	owned, err := store.ToolchainsDir()
	if err != nil {
		return shim.Stamper{}, err
	}
	return shim.Stamper{Tool: "gotm", BinDir: binDir, KebabDir: kebabDir, Owned: owned}, nil
}

// treeBins lists the commands a go tree ships in its bin dir, without the
// host's executable suffix.
func treeBins(tree string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(tree, "bin"))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), filepath.Ext(shim.ExeName("x"))))
	}
	slices.Sort(names)
	return names, nil
}

// treePresent reports whether release is unpacked: its go command is there.
func treePresent(release string) bool {
	if release == "" {
		return false
	}
	tree, err := store.ToolchainDir(release)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(tree, "bin", shim.ExeName(pkgGo)))
	return err == nil
}

// healthy reports whether what the manifest says of a toolchain is really on
// disk: the tree and every shim recorded for it. One that is not is redone by
// sync rather than believed.
func healthy(tc config.Toolchain) bool {
	if !treePresent(tc.Version) || len(tc.Bin) == 0 {
		return false
	}
	s, err := stamper()
	if err != nil {
		return false
	}
	for _, name := range tc.Bin {
		if !s.IsShim(s.Path(shim.ExeName(name))) {
			return false
		}
	}
	return true
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

// shown is a release as tables print it, with a dash for none.
func shown(release string) string {
	if release == "" {
		return "-"
	}
	return godl.Display(release)
}
