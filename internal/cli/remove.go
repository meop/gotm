package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/meop/golie/tidy"

	"github.com/meop/gotm/internal/config"
	"github.com/meop/gotm/internal/store"
)

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <version> [version...]",
		Aliases: []string{"r", "rm", "rem", "un", "unin", "uninstall"},
		Short:   "Remove installed toolchains",
		Args:    cobra.MinimumNArgs(1),
		RunE:    runRemove,
	}
}

// runRemove deletes a toolchain's shims, then its extract dir, and only then
// forgets it. Nothing here can be rolled back, so that order is the safety: a
// shim that will not delete leaves the manifest describing a toolchain that
// is still fully there, for the retry to finish.
func runRemove(cmd *cobra.Command, args []string) error {
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

	var keys []string
	var failed []failedItem
	for _, arg := range args {
		pin, err := parsePin(arg)
		if err != nil {
			printFail("%s: %v", arg, err)
			failed = append(failed, failedItem{name: arg, reason: err.Error()})
			continue
		}
		key := keyOf(pin)
		if _, ok := m.Extracts[key]; !ok {
			print("%s: not installed", key)
			continue
		}
		if !slices.Contains(keys, key) {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return closing(failed)
	}
	var rows [][]string
	for _, key := range keys {
		e := m.Extracts[key]
		rows = append(rows, []string{key, e.Version, e.Pin, m.Repos[pkgGo], strings.Join(e.Assets, ", ")})
	}
	colors := []func(string) string{nil, colorfn("info"), nil, nil, nil}
	if !gate([]string{"name", "version", "pin", "uri", "assets"}, rows, colors, fmt.Sprintf("uninstall %d toolchain(s)", len(keys))) {
		return closing(failed)
	}

	s, err := stamper()
	if err != nil {
		return fail("%v", err)
	}
	done := 0
	for _, key := range keys {
		e := m.Extracts[key]
		reason := ""
		for name := range e.Bin {
			if err := s.Remove(name); err != nil {
				printFail("%s: %s: could not remove shim: %v", key, name, err)
				if reason == "" {
					reason = fmt.Sprintf("%s: could not remove shim: %v", name, err)
				}
			}
		}
		if reason == "" {
			if err := tidy.PruneExtract(l.ExtractPath(key, e.Version), l.ExtractsDir()); err != nil && !os.IsNotExist(err) {
				reason = fmt.Sprintf("could not remove extract dir: %v", err)
				printFail("%s: %s", key, reason)
			}
		}
		if reason != "" {
			failed = append(failed, failedItem{name: key, reason: reason})
			continue
		}
		m.RemoveExtract(key)
		if err := config.SaveManifest(m); err != nil {
			return fail("could not save manifest: %v", err)
		}
		done++
	}
	if done > 0 {
		printPass("uninstalled %d toolchain(s)", done)
	}
	return closing(failed)
}
