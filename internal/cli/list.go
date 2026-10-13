package cli

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/meop/gotm/internal/config"
)

const msgNoneAdded = "no go added"

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "Show the added go toolchains",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := config.LoadManifest()
			if err != nil {
				return fail("could not load manifest: %v", err)
			}
			if len(m.Toolchains) == 0 {
				print(msgNoneAdded)
				return nil
			}
			var rows [][]string
			for _, pin := range m.Pins() {
				tc := m.Toolchains[pin]
				rows = append(rows, []string{shimName(pkgGo, pin), shown(tc.Version), pin, strings.Join(tc.Bin, " ")})
			}
			printTable([]string{"name", "version", "pin", "bin"}, rows, []func(string) string{colorfn("info")})
			return nil
		},
	}
}

func newOutdatedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "outdated",
		Short: "Show the added go toolchains a newer release is out for",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := config.LoadManifest()
			if err != nil {
				return fail("could not load manifest: %v", err)
			}
			if len(m.Toolchains) == 0 {
				print(msgNoneAdded)
				return nil
			}
			var r resolver
			var rows [][]string
			for _, pin := range m.Pins() {
				tc := m.Toolchains[pin]
				target, err := r.resolve(context.Background(), pin)
				if err != nil {
					return fail("%s: %v", shimName(pkgGo, pin), err)
				}
				if target.Version != tc.Version {
					rows = append(rows, []string{shimName(pkgGo, pin), shown(tc.Version), shown(target.Version), pin})
				}
			}
			if len(rows) == 0 {
				print("go is up to date")
				return nil
			}
			printTable([]string{"name", "version", "update", "pin"}, rows, []func(string) string{colorfn("info")})
			return nil
		},
	}
}
