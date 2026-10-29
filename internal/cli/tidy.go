package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/meop/golie/shim"
	"github.com/meop/golie/tidy"

	"github.com/meop/gotm/internal/config"
	"github.com/meop/gotm/internal/store"
)

// sourceDepth is how many path elements go's source has under download/:
// go.dev/dl.
const sourceDepth = 2

func newTidyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tidy",
		Aliases: []string{"t", "ti", "cl", "clean"},
		Short:   "Clean unused downloads and orphaned items",
		Args:    cobra.NoArgs,
		RunE:    runTidy,
	}
	cmd.Flags().Bool("all", false, "Remove all cached assets regardless of installation status")
	return cmd
}

// runTidy is ghpm's tidy over ~/.gotm: what is broken or orphaned is found
// and removed by golie's tidy, and each kind is shown and confirmed on its
// own here.
func runTidy(cmd *cobra.Command, args []string) error {
	unlock, err := config.AcquireLock()
	if err != nil {
		return fail("%v", err)
	}
	defer unlock()
	m, err := config.LoadManifest()
	if err != nil {
		return fail("could not load manifest: %v", err)
	}
	l, err := store.Layout()
	if err != nil {
		return fail("%v", err)
	}
	dirs, err := tidy.DirsOf(l)
	if err != nil {
		return fail("%v", err)
	}

	if all, _ := cmd.Flags().GetBool("all"); all {
		if dryRun {
			print("remove all cached assets in %s", dirs.Downloads)
			return nil
		}
		if !promptConfirm("remove all download(s)") {
			return nil
		}
		if err := os.RemoveAll(dirs.Downloads); err != nil {
			return fail("%v", err)
		}
		printPass("removed all cached assets")
		return nil
	}

	b1 := cleanBrokenInstalls(m, dirs)
	b2 := cleanOrphanedBinShims(m, dirs)
	b3 := cleanOrphanedExtracts(m, dirs)
	b4 := cleanOrphanedDownloads(m, dirs)
	if !b1 && !b2 && !b3 && !b4 {
		print("nothing to tidy")
	}
	return nil
}

// dryRunBail prints the shared dry-run closing message and returns true (the
// "there was something to report" signal every clean* function returns).
func dryRunBail() bool {
	sep()
	print(msgDryRun)
	return true
}

// cleanBrokenInstalls removes manifest entries whose extract dir is gone, and
// forgets the shims that are missing from one that is still there.
func cleanBrokenInstalls(m *config.Manifest, dirs tidy.Dirs) bool {
	items := tidy.FindBroken(m, dirs)
	if len(items) == 0 {
		return false
	}
	rows := make([][]string, len(items))
	for i, it := range items {
		rows[i] = []string{it.Key, it.Issue}
	}
	printTable([]string{"name", "issue"}, rows, nil)
	if dryRun {
		return dryRunBail()
	}
	if !promptConfirm(fmt.Sprintf("remove %d broken install(s)", len(items))) {
		return true
	}
	touched := false
	for _, it := range items {
		changed, stuck := tidy.RemoveBroken(m, it, dirs)
		for _, st := range stuck {
			printFail("%s: could not remove shim: %v", filepath.Base(st.Path), st.Err)
		}
		touched = touched || changed
	}
	if touched {
		if err := config.SaveManifest(m); err != nil {
			printFail("could not save manifest: %v", err)
			return true
		}
	}
	printPass("removed %d broken install(s)", len(items))
	return true
}

// cleanOrphanedBinShims removes the shims gotm stamped that no toolchain
// owns any more. A file it did not stamp is not its to clean up.
func cleanOrphanedBinShims(m *config.Manifest, dirs tidy.Dirs) bool {
	paths := tidy.FindOrphanShims(m, dirs.Bin, []string{shim.ExeName("gotm")}, isShim)
	if len(paths) == 0 {
		return false
	}
	rows := make([][]string, len(paths))
	for i, p := range paths {
		rows[i] = []string{filepath.Base(p)}
	}
	printTable([]string{"name"}, rows, nil)
	if dryRun {
		return dryRunBail()
	}
	if !promptConfirm(fmt.Sprintf("remove %d orphaned bin(s)", len(paths))) {
		return true
	}
	removed := 0
	for _, p := range paths {
		if err := os.Remove(p); err != nil {
			printFail("%s: %v", p, err)
			continue
		}
		removed++
	}
	if removed > 0 {
		printPass("removed %d orphaned bin(s)", removed)
	}
	return true
}

// cleanOrphanedExtracts removes extract dirs no manifest entry is on, and what
// an op left staged or aside.
func cleanOrphanedExtracts(m *config.Manifest, dirs tidy.Dirs) bool {
	orphans := tidy.FindOrphanExtracts(m, dirs.Extracts)
	if len(orphans) == 0 {
		return false
	}
	rows := make([][]string, len(orphans))
	for i, o := range orphans {
		state := "not installed"
		if o.Leftover {
			state = "left over"
		}
		rows[i] = []string{o.Key, state}
	}
	printTable([]string{"name", "state"}, rows, nil)
	if dryRun {
		return dryRunBail()
	}
	if !promptConfirm(fmt.Sprintf("remove %d orphaned extract(s)", len(orphans))) {
		return true
	}
	removed := 0
	for _, o := range orphans {
		if err := tidy.PruneExtract(o.Path, dirs.Extracts); err != nil {
			printFail("%s: %v", o.Path, err)
			continue
		}
		removed++
	}
	if removed > 0 {
		printPass("removed %d orphaned extract(s)", removed)
	}
	return true
}

// cleanOrphanedDownloads removes the cached archives of releases no
// toolchain is on.
func cleanOrphanedDownloads(m *config.Manifest, dirs tidy.Dirs) bool {
	orphans := tidy.FindOrphanDownloads(m, dirs.Downloads, sourceDepth)
	if len(orphans) == 0 {
		return false
	}
	rows := make([][]string, len(orphans))
	for i, o := range orphans {
		rows[i] = []string{pkgGo, o.Version, o.File}
	}
	printTable([]string{"package", "version", "file"}, rows, nil)
	if dryRun {
		return dryRunBail()
	}
	if !promptConfirm(fmt.Sprintf("remove %d orphaned download(s)", len(orphans))) {
		return true
	}
	removed := 0
	for _, o := range orphans {
		if err := os.Remove(o.Path); err != nil && !os.IsNotExist(err) {
			printFail("%s: %v", o.Path, err)
			continue
		}
		removed++
	}
	tidy.PruneEmptyDirs(dirs.Downloads)
	if removed > 0 {
		printPass("removed %d orphaned download(s)", removed)
	}
	return true
}
