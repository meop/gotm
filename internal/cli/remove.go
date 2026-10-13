package cli

import (
	"slices"

	"github.com/spf13/cobra"

	"github.com/meop/golie/shim"

	"github.com/meop/gotm/internal/config"
)

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "remove <version...>",
		Aliases: []string{"rm"},
		Short:   "Remove added go toolchains",
		Args:    cobra.MinimumNArgs(1),
		RunE:    runRemove,
	}
}

// runRemove deletes a toolchain's shims, then its tree, and only then forgets
// it. Nothing here can be rolled back, so that order is the safety: a shim
// that will not delete leaves the manifest describing a toolchain that is
// still fully there, for the retry to finish. A tree another pin is also on
// stays.
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

	var pins []string
	failed := 0
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
	if len(pins) == 0 {
		return closing(failed)
	}
	var rows [][]string
	for _, pin := range pins {
		rows = append(rows, []string{shimName(pkgGo, pin), shown(m.Toolchains[pin].Version), pin})
	}
	if !gate([]string{"name", "version", "pin"}, rows, []func(string) string{colorfn("info")}, "remove go?") {
		return closing(failed)
	}

	done := 0
	for _, pin := range pins {
		name, tc := shimName(pkgGo, pin), m.Toolchains[pin]
		if err := removeShims(tc.Bin); err != nil {
			printFail("%s: %v", name, err)
			failed++
			continue
		}
		if !m.Uses(tc.Version, pin) {
			removeTree(name, tc.Version)
		}
		delete(m.Toolchains, pin)
		if err := config.SaveManifest(m); err != nil {
			return fail("could not save manifest: %v", err)
		}
		done++
	}
	if done > 0 {
		printPass("removed %d toolchain(s)", done)
	}
	return closing(failed)
}

func removeShims(names []string) error {
	s, err := stamper()
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := s.Remove(shim.ExeName(name)); err != nil {
			return err
		}
	}
	return nil
}
