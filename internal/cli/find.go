package cli

import (
	"context"
	"runtime"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/meop/gotm/internal/config"
	"github.com/meop/gotm/internal/godl"
)

func newFindCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "find [version...]",
		Aliases: []string{"f", "fi", "se", "search"},
		Short:   "List or search go's releases by version",
		RunE:    runFind,
	}
	addNameFormatFlags(cmd)
	return cmd
}

// runFind is where a version to add is looked up: go.dev's releases that have
// an archive for this machine, which is what add can install here. With no
// term it is the newest release of each line, the way uv lists pythons; a
// term lists every release it starts (godl.Found), so a line's candidates and
// betas are found by naming the line.
func runFind(cmd *cobra.Command, args []string) error {
	m, err := config.LoadManifest()
	if err != nil {
		return fail("could not load manifest: %v", err)
	}
	all, err := godl.List(context.Background(), true)
	if err != nil {
		return fail("%v", err)
	}
	releases := slices.DeleteFunc(all, func(r godl.Release) bool {
		_, err := r.Archive()
		return err != nil
	})
	if len(releases) == 0 {
		print("no releases for %s/%s", runtime.GOOS, runtime.GOARCH)
		return nil
	}

	if len(args) == 0 {
		printReleases(m, godl.Lines(releases))
		return nil
	}
	for _, term := range args {
		sep()
		if len(args) > 1 {
			print("find: %s", term)
		}
		var matches []godl.Release
		if pin, err := parsePin(term); err == nil {
			if pin == "" {
				pin = config.PinLatest
			}
			matches = godl.Found(releases, pin)
		}
		if len(matches) == 0 {
			print("no releases matching %q", term)
			continue
		}
		printReleases(m, matches)
	}
	return nil
}

// printReleases renders releases as versions add takes, beside the added
// toolchains that are on each. The name column is dropped when none is.
func printReleases(m *config.Manifest, releases []godl.Release) {
	versions := make([]string, len(releases))
	rows := make([][]string, len(releases))
	added := false
	for i, r := range releases {
		versions[i] = godl.Display(r.Version)
		var names []string
		for _, key := range m.Keys() {
			if m.Extracts[key].Version == versions[i] {
				names = append(names, key)
			}
		}
		added = added || len(names) > 0
		rows[i] = []string{versions[i], strings.Join(names, ", ")}
	}
	if printNameList(versions) {
		return
	}
	headers := []string{"version", "name"}
	if !added {
		headers = headers[:1]
		for i := range rows {
			rows[i] = rows[i][:1]
		}
	}
	printTable(headers, rows, nil)
}
