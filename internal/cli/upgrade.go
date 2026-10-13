package cli

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/meop/golie/upgrade"
)

// self is gotm as its own release page publishes it. Everything upgrade asks
// of GitHub is a plain page or release download; see golie's upgrade package.
var self = upgrade.Self{Name: "gotm", ReleasesURL: "https://github.com/meop/gotm/releases/"}

func newUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade gotm itself",
		Args:  cobra.NoArgs,
		RunE:  runUpgrade,
	}
}

// runUpgrade upgrades gotm and nothing else: the go toolchains are sync's,
// and the vendored sheesh is pinned to whatever the new gotm was built to run.
func runUpgrade(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	tag, err := self.LatestTag(ctx)
	if err != nil {
		return fail("%v", err)
	}
	latest := strings.TrimPrefix(tag, "v")
	if latest == strings.TrimPrefix(version, "v") {
		print("gotm is up to date")
		return nil
	}
	if !gate(
		[]string{"name", "version", "update"},
		[][]string{{"gotm", version, latest}},
		[]func(string) string{colorfn("info")},
		"upgrade gotm?",
	) {
		return nil
	}
	if err := self.Install(ctx, tag, func(asset string) { print("gotm: asset fetching [%s]...", asset) }); err != nil {
		return fail("gotm: %v", err)
	}
	printPass("upgraded gotm to %s", latest)
	return nil
}
