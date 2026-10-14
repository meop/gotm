package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/meop/golie/ui"
)

var errSilent = errors.New("")

const msgNoneInstalled = "no toolchains installed"

// msgNoMatch is shown when a version filter was given but matched nothing
// installed, to tell it from an empty install set ("no toolchains installed").
const msgNoMatch = "no toolchains matched"

const msgAllUpToDate = "all toolchains are up to date"

const msgGotmUpToDate = "gotm is up to date"

// msgCannotUpgradeItself is upgrade's whole answer to a gotm its installer
// did not place: the path it runs from is what says who does upgrade it.
const msgCannotUpgradeItself = "gotm: cannot upgrade itself, running from %s"

// msgDryRun is printed at every dry-run bail point: a bare preview table
// followed by silence is indistinguishable from a hang.
const msgDryRun = "dry run, no changes made"

// sep requests a deferred blank line before the next output. See internal/ui.
func sep() { ui.Break() }

var longNames, shortNames bool

// stdout is where names mode writes; a test points it at a buffer.
var stdout io.Writer = os.Stdout

func addNameFormatFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&longNames, "long-names", "l", false, "Print pins only, one per line")
	cmd.Flags().BoolVarP(&shortNames, "short-names", "s", false, "Print pins only, space-separated on one line")
}

// namesOnly reports whether the command was asked for names alone
// (--long-names, --short-names). That output is for a script to read, so
// nothing but names may be in it — not even the line saying there are none,
// which a script would otherwise have to recognise by its wording. An empty
// result is empty output.
func namesOnly() bool { return longNames || shortNames }

// printNameList prints names in the format asked for, and nothing at all when
// there are none. It returns true if either flag is set.
func printNameList(names []string) bool {
	if !namesOnly() {
		return false
	}
	if len(names) == 0 {
		return true
	}
	if longNames {
		for _, n := range names {
			_, _ = fmt.Fprintln(stdout, n)
		}
		return true
	}
	_, _ = fmt.Fprintln(stdout, strings.Join(names, " "))
	return true
}

// -q ("quiet") suppresses status chatter (print, printWarn, printPass) but
// never the substantive content: printFail (errors), and printTable (which
// backs both gate previews and the actual result of read-only commands).
func print(format string, args ...any) {
	if quiet || namesOnly() {
		return
	}
	ui.Out(format, args...)
}

func printWarn(format string, args ...any) {
	if quiet || namesOnly() {
		return
	}
	ui.Warn(format, args...)
}

func printFail(format string, args ...any) { ui.Fail(format, args...) }

func printPass(format string, args ...any) {
	if quiet || namesOnly() {
		return
	}
	ui.Pass(format, args...)
}

func printTable(headers []string, rows [][]string, colColors []func(string) string) {
	ui.Table(headers, rows, colColors)
}

func promptConfirm(msg string) bool {
	if yes {
		return true
	}
	return ui.Confirm(msg)
}

// gate renders the "here's what I'm going to do" preview table and then,
// unless this is a dry run, asks confirmMsg. It returns true only when the
// user opts in; both a dry run and a declined prompt return false, signaling
// the caller to stop.
func gate(headers []string, rows [][]string, colColors []func(string) string, confirmMsg string) bool {
	printTable(headers, rows, colColors)
	if dryRun {
		sep()
		print(msgDryRun)
		return false
	}
	return promptConfirm(confirmMsg)
}

// failedItem is one item that did not make it into a mutating command's
// success count, paired with why, so the reason is still on screen when the
// run ends rather than wherever the inline line scrolled to.
type failedItem struct {
	name   string
	reason string
}

// printFailedTable closes a mutating command with what failed and why. noun
// is singular ("toolchain", "component").
func printFailedTable(noun string, items []failedItem) {
	if len(items) == 0 {
		return
	}
	sorted := slices.Clone(items)
	slices.SortFunc(sorted, func(a, b failedItem) int { return strings.Compare(a.name, b.name) })
	sep()
	printFail("%d %s(s) failed", len(sorted), noun)
	rows := make([][]string, len(sorted))
	for i, it := range sorted {
		rows[i] = []string{it.name, it.reason}
	}
	printTable([]string{"name", "reason"}, rows, nil)
	sep()
}

// closing ends a command that reported its failures as it met them.
func closing(failed []failedItem) error {
	printFailedTable("toolchain", failed)
	if len(failed) > 0 {
		return errSilent
	}
	return nil
}

// fail reports err and returns the error a command ends with once it has
// said what went wrong itself.
func fail(format string, args ...any) error {
	printFail(format, args...)
	return errSilent
}
