package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/meop/golie/fetch"
	"github.com/meop/golie/journal"
	"github.com/meop/golie/layout"
	"github.com/meop/golie/manifest"
	"github.com/meop/golie/pinned"
	"github.com/meop/golie/sheesh"
	"github.com/meop/golie/tidy"
	"github.com/meop/golie/vers"

	"github.com/meop/gotm/internal/config"
	"github.com/meop/gotm/internal/godl"
	"github.com/meop/gotm/internal/store"
)

// reinstall and force are the two flags add and sync share, and they are
// separate because they are different things to ask for: --reinstall redoes
// gotm's own work and touches nothing that is not gotm's, while --force is the
// only way a shim replaces a file gotm did not stamp. Neither implies the
// other.
var reinstall, force bool

func addInstallFlags(cmd *cobra.Command, reinstallUsage string) {
	cmd.Flags().BoolVarP(&reinstall, "reinstall", "r", false, reinstallUsage)
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Replace a file in the bin directory that is not a gotm shim")
}

// change is one toolchain a command is about to move: its manifest key, what
// it is pinned to, what the manifest says of it now (the zero value when it
// is being added), and the release it is going to.
type change struct {
	key    string
	pin    string
	prev   config.Entry
	target godl.Release
}

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "add [version...]",
		Aliases: []string{"a", "ad", "in", "install"},
		Short:   "Add go toolchains: latest, a line (1.26) or one release (1.26.4)",
		Long: "Add go toolchains, each kept beside any already added.\n\n" +
			"  gotm add          follow go's newest stable release, as go and gofmt\n" +
			"  gotm add latest   the same, said aloud\n" +
			"  gotm add 1.26     follow the newest 1.26.x, as go@1.26 and gofmt@1.26\n" +
			"  gotm add 1.26.4   stay on exactly 1.26.4, as go@1.26.4 and gofmt@1.26.4",
		RunE: runAdd,
	}
	addInstallFlags(cmd, "Reinstall even if already installed")
	return cmd
}

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sync [version...]",
		Aliases: []string{"s", "sy", "up", "update"},
		Short:   "Sync toolchains to the newest release each pin allows",
		RunE:    runSync,
	}
	addInstallFlags(cmd, "Reinstall even if already at latest version")
	return cmd
}

// runAdd is ghpm's add with the package left off: no argument is a bare
// "go", the toolchain that follows go's newest release.
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
	var failed []failedItem
	for _, arg := range args {
		pin, err := parsePin(arg)
		if err != nil {
			printFail("%s: %v", arg, err)
			failed = append(failed, failedItem{name: arg, reason: err.Error()})
			continue
		}
		key := keyOf(pin)
		if slices.ContainsFunc(changes, func(c change) bool { return c.key == key }) {
			continue
		}
		prev, exists := m.Extracts[key]
		if exists && !reinstall {
			print("%s: already added → %s", key, prev.Version)
			continue
		}
		target, err := r.resolve(ctx, pin)
		if err != nil {
			printFail("%s: %v", key, err)
			failed = append(failed, failedItem{name: key, reason: err.Error()})
			continue
		}
		changes = append(changes, change{key: key, pin: pin, prev: prev, target: target})
	}
	if len(changes) == 0 {
		return closing(failed)
	}
	var rows [][]string
	for _, c := range changes {
		rows = append(rows, []string{c.key, godl.Display(c.target.Version), manifest.Pin(c.pin), config.Source})
	}
	colors := []func(string) string{nil, colorfn("new"), nil, nil}
	if gate([]string{"name", "version", "pin", "uri"}, rows, colors, fmt.Sprintf("add %d toolchain(s)", len(changes))) {
		done, bad := applyAll(ctx, m, changes)
		failed = append(failed, bad...)
		if done > 0 {
			printPass("installed %d toolchain(s)", done)
			warnPath()
		}
	}
	return closing(failed)
}

// runSync updates what was added and nothing else: with no toolchain added it
// installs none, since which go to follow is a choice add is there to record.
// A toolchain held to one release has nowhere to move, and is left alone.
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
	if len(m.Extracts) == 0 {
		print(msgNoneInstalled)
		return nil
	}

	var keys []string
	var failed []failedItem
	if len(args) == 0 {
		for _, key := range m.Keys() {
			if m.Extracts[key].Pin != manifest.PinFixed {
				keys = append(keys, key)
			}
		}
	}
	for _, arg := range args {
		pin, err := parsePin(arg)
		if err != nil {
			printFail("%s: %v", arg, err)
			failed = append(failed, failedItem{name: arg, reason: err.Error()})
			continue
		}
		key := keyOf(pin)
		e, ok := m.Extracts[key]
		if !ok {
			print("%s: not installed", key)
			continue
		}
		if e.Pin == manifest.PinFixed {
			print("%s: fixed at %s, skipping", key, e.Version)
			continue
		}
		if !slices.Contains(keys, key) {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return closing(failed)
	}

	var r resolver
	var changes []change
	for _, key := range keys {
		_, pin, _ := manifest.ParseKey(key)
		prev := m.Extracts[key]
		target, err := r.resolve(ctx, pin)
		if err != nil {
			printFail("%s: %v", key, err)
			failed = append(failed, failedItem{name: key, reason: err.Error()})
			continue
		}
		if vers.CompareVersions(target.Version, prev.Version) > 0 || reinstall {
			changes = append(changes, change{key: key, pin: pin, prev: prev, target: target})
		}
	}
	if len(changes) == 0 {
		if len(failed) == 0 {
			print(msgAllUpToDate)
		}
		return closing(failed)
	}
	var rows [][]string
	for _, c := range changes {
		rows = append(rows, []string{c.key, c.prev.Version, godl.Display(c.target.Version), c.prev.Pin, m.Repos[pkgGo]})
	}
	colors := []func(string) string{nil, colorfn("old"), colorfn("new"), nil, nil}
	if gate([]string{"name", "version", "update", "pin", "uri"}, rows, colors, fmt.Sprintf("update %d toolchain(s)", len(changes))) {
		done, bad := applyAll(ctx, m, changes)
		failed = append(failed, bad...)
		if done > 0 {
			printPass("updated %d toolchain(s)", done)
		}
	}
	return closing(failed)
}

// applyAll makes each change in turn, after making sure kebab is there to
// stamp with. One that fails is said and does not stop the rest.
func applyAll(ctx context.Context, m *config.Manifest, changes []change) (done int, failed []failedItem) {
	l, err := store.Layout()
	if err == nil {
		err = syncSheesh(ctx, l)
	}
	if err != nil {
		printFail("sheesh: %v", err)
		for _, c := range changes {
			failed = append(failed, failedItem{name: c.key, reason: fmt.Sprintf("sheesh: %v", err)})
		}
		return 0, failed
	}
	for _, c := range changes {
		if err := apply(ctx, m, l, c); err != nil {
			printFail("%s: %v", c.key, err)
			failed = append(failed, failedItem{name: c.key, reason: err.Error()})
			continue
		}
		done++
	}
	return done, failed
}

// syncSheesh makes sure the vendored kebab and its templates are the ones
// this build pins, by content. A copy of this very version that does not
// match is replaced like any other, and said: it was damaged or changed.
func syncSheesh(ctx context.Context, l layout.Layout) error {
	dir, err := l.ShimDir()
	if err != nil {
		return err
	}
	return sheesh.Sync(ctx, dir, pinned.Options{
		Fetching:  func(asset string) { print("sheesh: asset fetching [%s]...", asset) },
		Replacing: func(file string) { printWarn("sheesh: %s does not match its pin, replacing", file) },
	})
}

// apply brings one toolchain to its target release, the way ghpm installs a
// package: the archive into the download cache, unpacked into the install's
// own extract dir, a shim per command, and then the manifest entry.
//
// The manifest is the commit point. Everything before it is journalled, and a
// failure rolls the journal back to the release the manifest still names. The
// old extract dir is removed only after the commit, because that rollback
// restores from it.
func apply(ctx context.Context, m *config.Manifest, l layout.Layout, c change) error {
	version := godl.Display(c.target.Version)
	file, err := c.target.Archive()
	if err != nil {
		return err
	}
	downloads, err := l.DownloadDir(config.Source, version)
	if err != nil {
		return err
	}
	archive := filepath.Join(downloads, file.Filename)
	// an archive already in the cache is not fetched again, and says nothing
	if !cached(archive, file.SHA256) {
		print("%s: asset fetching [%s]...", c.key, file.Filename)
		if err := fetch.Download(ctx, file.URL(), archive, file.SHA256); err != nil {
			return err
		}
	}
	s, err := stamper()
	if err != nil {
		return err
	}

	tree := l.ExtractPath(c.key, version)
	oldTree := l.ExtractPath(c.key, c.prev.Version)
	jrn := &journal.Journal{}
	rollback := func(err error) error {
		for _, undo := range jrn.Rollback() {
			printWarn("%s: could not roll back: %v", c.key, undo)
		}
		return err
	}

	jrn.Did("extract", journal.ExtractUndo(tree, c.prev.Version, version))
	if err := unpack(archive, tree); err != nil {
		return rollback(err)
	}
	keys, err := treeBins(tree)
	if err != nil {
		return rollback(err)
	}
	bins := make(map[string]string, len(keys))
	for _, binKey := range keys {
		name := manifest.ShimName(c.key, filepath.Base(binKey))
		print("%s: bin found [%s]", c.key, binKey)
		oldSource := ""
		if old := c.prev.Bin[name]; old != "" {
			oldSource = filepath.Join(oldTree, filepath.FromSlash(old))
		}
		jrn.Did("shim "+name, journal.ShimUndo(s, name, oldSource))
		if err := s.Create(name, filepath.Join(tree, filepath.FromSlash(binKey)), force); err != nil {
			return rollback(fmt.Errorf("%s: could not create shim: %w", name, err))
		}
		bins[name] = binKey
	}

	m.Repos[pkgGo] = config.Source
	m.Extracts[c.key] = config.Entry{Pin: manifest.Pin(c.pin), Version: version, Assets: []string{file.Filename}, Bin: bins}
	if err := config.SaveManifest(m); err != nil {
		m.Extracts[c.key] = c.prev
		if c.prev.Version == "" {
			m.RemoveExtract(c.key)
		}
		return rollback(fmt.Errorf("could not save manifest: %w", err))
	}

	// after the commit: a shim the new release no longer ships, and the
	// release this one replaced
	for name := range c.prev.Bin {
		if _, ok := bins[name]; !ok {
			if err := s.Remove(name); err != nil {
				printWarn("%s: could not remove stale shim %s: %v", c.key, name, err)
			}
		}
	}
	if c.prev.Version != "" && c.prev.Version != version {
		if err := tidy.PruneExtract(oldTree, l.ExtractsDir()); err != nil {
			printWarn("%s: could not remove %s: %v", c.key, oldTree, err)
		}
	}
	return nil
}

// unpack makes tree the go the archive holds. It is unpacked beside tree and
// renamed in, so tree either is a whole go or is not there.
func unpack(archive, tree string) error {
	if err := os.MkdirAll(filepath.Dir(tree), 0755); err != nil {
		return err
	}
	staging := tree + ".tmp"
	if err := os.RemoveAll(staging); err != nil {
		return err
	}
	// the archive holds one top level "go" directory
	if err := fetch.Extract(archive, staging, 1); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
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

// warnPath says so when the shims were stamped somewhere PATH does not reach.
func warnPath() {
	l, err := store.Layout()
	if err != nil {
		return
	}
	binDir, err := l.BinDir()
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
