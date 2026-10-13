package cli

import (
	"errors"

	"github.com/meop/golie/ui"
)

var errSilent = errors.New("")

// msgDryRun is printed at every dry-run bail point: a bare preview table
// followed by silence is indistinguishable from a hang.
const msgDryRun = "dry run, no changes made"

// sep requests a deferred blank line before the next output. See internal/ui.
func sep() { ui.Break() }

// -q ("quiet") suppresses status chatter (print, printWarn, printPass) but
// never the substantive content: printFail (errors), and printTable (which
// backs both gate previews and the actual result of read-only commands).
func print(format string, args ...any) {
	if quiet {
		return
	}
	ui.Out(format, args...)
}

func printWarn(format string, args ...any) {
	if quiet {
		return
	}
	ui.Warn(format, args...)
}

func printFail(format string, args ...any) { ui.Fail(format, args...) }

func printPass(format string, args ...any) {
	if quiet {
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

// fail reports err and returns the error a command ends with once it has
// said what went wrong itself.
func fail(format string, args ...any) error {
	printFail(format, args...)
	return errSilent
}
