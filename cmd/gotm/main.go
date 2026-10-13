package main

import (
	"os"

	"github.com/meop/golie/ui"
	"github.com/meop/gotm/internal/cli"
)

var version = "dev"

func main() {
	cli.SetVersion(version)
	root := cli.NewRootCmd()
	cmd, err := root.ExecuteC()
	if err != nil {
		if err.Error() != "" {
			// errSilent (empty message) means a subcommand already reported its
			// own failure via ui.Fail; anything with a real message here is a
			// cobra-level error (bad flag, wrong arg count) that nothing else
			// has shown the user yet.
			ui.Fail("%v", err)
			_ = cmd.Help()
		}
		os.Exit(1)
	}
}
