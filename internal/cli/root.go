// Package cli holds gotm's subcommands.
package cli

import (
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/meop/golie/ui"

	"github.com/meop/gotm/internal/config"
)

// pkgGo is the one package gotm manages: the name ghpm would be given, and
// the one every gotm command leaves off.
const pkgGo = config.Package

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
	"new":  color.FgCyan,
	"old":  color.FgMagenta,
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

	root.SetUsageTemplate(`Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if and (not .Runnable) .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`)

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
		newFindCmd(),
		newListCmd(),
		newOutdatedCmd(),
		newRemoveCmd(),
		newSyncCmd(),
		newTidyCmd(),
		newUpgradeCmd(),
	)

	return root
}
