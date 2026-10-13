// Package cli holds gotm's subcommands.
package cli

import (
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/meop/golie/ui"
)

// pkgGo is the one package gotm manages, as its tables name it.
const pkgGo = "go"

var (
	version        = "dev"
	dryRun         bool
	quiet          bool
	yes            bool
	nonInteractive bool
)

func SetVersion(v string) { version = v }

var roleColors = map[string]color.Attribute{
	"fail": color.FgRed,
	"info": color.FgBlue,
	"pass": color.FgGreen,
	"warn": color.FgYellow,
}

// colorfn returns a sprint function for the named role, or nil when there is
// none. fatih/color itself stays plain when NO_COLOR is set or the output is
// not a terminal.
func colorfn(role string) func(string) string {
	attr, ok := roleColors[role]
	if !ok {
		return nil
	}
	fn := color.New(attr).SprintFunc()
	return func(s string) string { return fn(s) }
}

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "gotm [flags] [command]",
		Short:         "Go Toolchain Manager — go toolchains, each kept at the release you pin",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cmd.Help()
			return nil
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			ui.SetNonInteractive(nonInteractive)
			ui.SetColorResolver(colorfn)
			return nil
		},
	}

	root.Version = version
	root.SetVersionTemplate("{{.Version}}\n")
	root.Flags().Bool("version", false, "Print gotm version")

	root.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "Print execution only without running")
	root.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-error output")
	root.PersistentFlags().BoolVarP(&yes, "yes", "y", false, "Yes for confirmation prompts")
	root.PersistentFlags().BoolVar(&nonInteractive, "non-interactive", false, "Never prompt; for scripted use")

	root.SetHelpCommand(&cobra.Command{Hidden: true})

	root.AddCommand(
		newAddCmd(),
		newListCmd(),
		newOutdatedCmd(),
		newRemoveCmd(),
		newSyncCmd(),
		newUpgradeCmd(),
	)

	return root
}
