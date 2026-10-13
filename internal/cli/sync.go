package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/meop/golie/pinned"
	"github.com/meop/golie/sheesh"

	"github.com/meop/golie/fetch"
	"github.com/meop/golie/shim"
	"github.com/meop/gotm/internal/config"
	"github.com/meop/gotm/internal/godl"
	"github.com/meop/gotm/internal/store"
)

var force bool

// change is one toolchain a command is about to move: the pin it is keyed by,
// what the manifest says of it now (the zero value when it is being added),
// and the release it is going to.
type change struct {
	pin    string
	prev   config.Toolchain
	target godl.Release
}

func (c change) name() string { return shimName(pkgGo, c.pin) }

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [version...]",
		Short: "Add a go toolchain: latest, a line (1.26) or one release (1.26.4)",
		Long: "Add a go toolchain, and keep it beside any already added.\n\n" +
			"  gotm add          follow go's newest stable release, as go and gofmt\n" +
			"  gotm add 1.26     follow the newest 1.26.x, as go1.26 and gofmt1.26\n" +
			"  gotm add 1.26.4   stay on exactly 1.26.4, as go1.26.4 and gofmt1.26.4",
		RunE: runAdd,
	}
	cmd.Flags().BoolVar(&force, "force", false, "Replace a file in the bin directory that is not a gotm shim")
	return cmd
}

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sync [version...]",
		Aliases: []string{"up"},
		Short:   "Update the added go toolchains to the newest release each pin allows",
		RunE:    runSync,
	}
	cmd.Flags().BoolVar(&force, "force", false, "Replace a file in the bin directory that is not a gotm shim")
	return cmd
}

func runAdd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	unlock, err := config.AcquireLock()
	if err != nil {
		return fail("%v", err)
	}
	defer unlock()
	m, err := config.LoadManifest()
	if err != nil {
		return fail("could not load manifest: %v", err)
	}
	if len(args) == 0 {
		args = []string{config.PinLatest}
	}

	var r resolver
	var changes []change
	failed := 0
	for _, arg := range args {
		pin, err := normalizePin(arg)
		if err != nil {
			printFail("%v", err)
			failed++
			continue
		}
		if slices.ContainsFunc(changes, func(c change) bool { return c.pin == pin }) {
			continue
		}
		if _, ok := m.Toolchains[pin]; ok {
			print("%s already added", shimName(pkgGo, pin))
			continue
		}
		target, err := r.resolve(ctx, pin)
		if err != nil {
			printFail("%s: %v", shimName(pkgGo, pin), err)
			failed++
			continue
		}
		changes = append(changes, change{pin: pin, target: target})
	}
	if len(changes) > 0 {
		var rows [][]string
		for _, c := range changes {
			rows = append(rows, []string{c.name(), shown(c.target.Version), c.pin})
		}
		if gate([]string{"name", "version", "pin"}, rows, []func(string) string{colorfn("info")}, "add go?") {
			done, bad := applyAll(ctx, m, changes)
			failed += bad
			if done > 0 {
				printPass("added %d toolchain(s)", done)
				warnPath()
			}
		}
	}
	return closing(failed)
}

// runSync updates what was added and nothing else: with no toolchain added it
// installs none, since which go to follow is a choice add is there to record.
func runSync(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	unlock, err := config.AcquireLock()
	if err != nil {
		return fail("%v", err)
	}
	defer unlock()
	m, err := config.LoadManifest()
	if err != nil {
		return fail("could not load manifest: %v", err)
	}
	if len(m.Toolchains) == 0 {
		print(msgNoneAdded)
		return nil
	}

	pins := m.Pins()
	failed := 0
	if len(args) > 0 {
		pins = pins[:0]
		for _, arg := range args {
			pin, err := normalizePin(arg)
			if err != nil {
				printFail("%v", err)
				failed++
				continue
			}
			if _, ok := m.Toolchains[pin]; !ok {
				print("%s not added", shimName(pkgGo, pin))
				continue
			}
			if !slices.Contains(pins, pin) {
				pins = append(pins, pin)
			}
		}
	}

	var r resolver
	var changes []change
	for _, pin := range pins {
		prev := m.Toolchains[pin]
		target, err := r.resolve(ctx, pin)
		if err != nil {
			printFail("%s: %v", shimName(pkgGo, pin), err)
			failed++
			continue
		}
		// the same release again is a toolchain the disk no longer bears out: a
		// tree or a shim is missing, and sync puts it back
		if target.Version != prev.Version || !healthy(prev) {
			changes = append(changes, change{pin: pin, prev: prev, target: target})
		}
	}
	if len(changes) == 0 {
		if failed == 0 {
			print("go is up to date")
		}
		return closing(failed)
	}
	var rows [][]string
	for _, c := range changes {
		rows = append(rows, []string{c.name(), shown(c.prev.Version), shown(c.target.Version), c.pin})
	}
	if gate([]string{"name", "version", "update", "pin"}, rows, []func(string) string{colorfn("info")}, "update go?") {
		done, bad := applyAll(ctx, m, changes)
		failed += bad
		if done > 0 {
			printPass("updated %d toolchain(s)", done)
		}
	}
	return closing(failed)
}

// closing ends a command that reported its failures as it met them.
func closing(failed int) error {
	if failed > 0 {
		printFail("%d failed", failed)
		return errSilent
	}
	return nil
}

// applyAll makes each change in turn, after making sure kebab is there to
// stamp with. One that fails is said and does not stop the rest.
func applyAll(ctx context.Context, m *config.Manifest, changes []change) (done, failed int) {
	if err := syncSheesh(ctx); err != nil {
		printFail("sheesh: %v", err)
		return 0, len(changes)
	}
	for _, c := range changes {
		if err := apply(ctx, m, c); err != nil {
			printFail("%s: %v", c.name(), err)
			failed++
			continue
		}
		done++
	}
	return done, failed
}

// syncSheesh makes sure the vendored kebab and its templates are the ones
// this build pins, by content. A copy of this very version that does not
// match is replaced like any other, and said: it was damaged or changed.
func syncSheesh(ctx context.Context) error {
	dir, err := store.ShimDir()
	if err != nil {
		return err
	}
	return sheesh.Sync(ctx, dir, pinned.Options{
		Fetching:  func(asset string) { print("sheesh: asset fetching [%s]...", asset) },
		Replacing: func(file string) { printWarn("sheesh: %s does not match its pin, replacing", file) },
	})
}

// apply brings one toolchain to its target release.
//
// The manifest is the commit point: it is written only once the tree is
// unpacked and every shim points into it, and a failure before that puts the
// shims back on the tree the manifest still names. A tree is shared by every
// pin that resolves to it, so the old one is deleted only after the commit —
// that restore needs it — and only when no other pin is still on it.
func apply(ctx context.Context, m *config.Manifest, c change) error {
	tree, err := store.ToolchainDir(c.target.Version)
	if err != nil {
		return err
	}
	unpacked := false
	if !treePresent(c.target.Version) {
		if err := fetchTree(ctx, c.name(), c.target, tree); err != nil {
			return err
		}
		unpacked = true
	}
	bins, err := treeBins(tree)
	if err != nil {
		return err
	}
	names, err := stamp(tree, bins, c.pin)
	if err != nil {
		// put back what the manifest still describes, and drop what this run
		// unpacked; a restore that fails is said rather than hidden
		if c.prev.Version != "" && c.prev.Version != c.target.Version && treePresent(c.prev.Version) {
			if restoreErr := restore(c); restoreErr != nil {
				printWarn("%s: could not restore the shims for %s: %v", c.name(), shown(c.prev.Version), restoreErr)
			}
		}
		if unpacked && !m.Uses(c.target.Version, c.pin) {
			_ = os.RemoveAll(tree)
		}
		return err
	}

	m.Toolchains[c.pin] = config.Toolchain{Version: c.target.Version, Bin: names}
	if err := config.SaveManifest(m); err != nil {
		return fmt.Errorf("could not save manifest: %w", err)
	}

	// after the commit: a shim the new release no longer ships, and the tree
	// nothing points into any more
	for _, name := range c.prev.Bin {
		if !slices.Contains(names, name) {
			if err := removeShims([]string{name}); err != nil {
				printWarn("%s: could not remove the shim %s: %v", c.name(), name, err)
			}
		}
	}
	if c.prev.Version != "" && c.prev.Version != c.target.Version && !m.Uses(c.prev.Version, "") {
		removeTree(c.name(), c.prev.Version)
	}
	return nil
}

// restore points a toolchain's shims back at the release it was on.
func restore(c change) error {
	old, err := store.ToolchainDir(c.prev.Version)
	if err != nil {
		return err
	}
	bins, err := treeBins(old)
	if err != nil {
		return err
	}
	_, err = stamp(old, bins, c.pin)
	return err
}

func removeTree(name, release string) {
	tree, err := store.ToolchainDir(release)
	if err != nil {
		return
	}
	if err := os.RemoveAll(tree); err != nil {
		printWarn("%s: could not remove %s: %v", name, tree, err)
	}
}

// fetchTree downloads a release's archive for this machine, checks it against
// the checksum go.dev states, and unpacks it as tree. It is unpacked beside
// tree and renamed in, so tree either is a whole go or is not there.
func fetchTree(ctx context.Context, name string, release godl.Release, tree string) error {
	file, err := release.Archive()
	if err != nil {
		return err
	}
	downloads, err := store.DownloadDir()
	if err != nil {
		return err
	}
	archive := filepath.Join(downloads, file.Filename)
	print("%s: asset fetching [%s]...", name, file.Filename)
	if err := fetch.Download(ctx, file.URL(), archive, file.SHA256); err != nil {
		return err
	}
	defer func() { _ = os.Remove(archive) }()

	staging := tree + ".tmp"
	if err := os.RemoveAll(staging); err != nil {
		return err
	}
	// the archive holds one top level "go" directory
	if err := fetch.Extract(archive, staging, 1); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	// a tree left without its go command is a broken unpack this replaces
	if err := os.RemoveAll(tree); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	if err := os.Rename(staging, tree); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	return nil
}

// stamp points a shim for each of a tree's commands at it, named for the
// pin, and returns the shim names.
func stamp(tree string, bins []string, pin string) ([]string, error) {
	s, err := stamper()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(bins))
	for _, bin := range bins {
		name := shimName(bin, pin)
		if err := s.Create(shim.ExeName(name), filepath.Join(tree, "bin", shim.ExeName(bin)), force); err != nil {
			return nil, fmt.Errorf("shim %s: %w", name, err)
		}
		names = append(names, name)
	}
	return names, nil
}

// warnPath says so when the shims were stamped somewhere PATH does not reach.
func warnPath() {
	binDir, err := store.BinDir()
	if err != nil {
		return
	}
	// windows spells the same directory in more than one case
	if slices.ContainsFunc(filepath.SplitList(os.Getenv("PATH")), func(dir string) bool {
		return strings.EqualFold(filepath.Clean(dir), binDir)
	}) {
		return
	}
	printWarn("%s is not on PATH", binDir)
}
