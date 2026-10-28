package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/meop/golie/upgrade"

	"github.com/meop/gotm/internal/store"
)

// self is gotm as its own release page publishes it. Everything upgrade asks
// of GitHub is a plain page or release download; see golie's upgrade package.
var self = upgrade.Self{Name: "gotm", ReleasesURL: "https://github.com/meop/gotm/releases/"}

// installedSelf is self held to the one place gotm upgrades itself from:
// ~/.gotm/bin, where its installer puts it. A gotm that runs from anywhere
// else — ghpm's, go install's, a build — is upgraded by what put it there.
func installedSelf() (upgrade.Self, error) {
	l, err := store.Layout()
	if err != nil {
		return upgrade.Self{}, err
	}
	binDir, err := l.BinDir()
	if err != nil {
		return upgrade.Self{}, err
	}
	s := self
	s.Dir = binDir
	return s, nil
}

func newUpgradeCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "upgrade",
		Aliases: []string{"ug", "upg"},
		Short:   "Upgrade gotm to its latest release",
		Args:    cobra.NoArgs,
		RunE:    runUpgrade,
	}
}

// runUpgrade upgrades gotm and nothing else: the go toolchains are sync's,
// and the vendored wedge is pinned to whatever the new gotm was built to run.
func runUpgrade(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	failed := func(err error) error {
		printFail("gotm: %v", err)
		printFailedTable("component", []failedItem{{name: "gotm", reason: err.Error()}})
		return errSilent
	}

	// asked first, and of nothing but the disk: a gotm that will not be
	// replaced has no use for which release is newest
	self, err := installedSelf()
	if err != nil {
		return failed(err)
	}
	if err := self.Check(); err != nil {
		if elsewhere, ok := errors.AsType[*upgrade.NotSelfInstalledError](err); ok {
			return fail(msgCannotUpgradeItself, elsewhere.Running)
		}
		return failed(err)
	}

	tag, err := self.LatestTag(ctx)
	if err != nil {
		return failed(err)
	}
	latest := strings.TrimPrefix(tag, "v")
	if latest == strings.TrimPrefix(version, "v") {
		print(msgGotmUpToDate)
		return nil
	}
	if !gate(
		[]string{"name", "version", "update"},
		[][]string{{"gotm", version, latest}},
		[]func(string) string{nil, colorfn("old"), colorfn("new")},
		fmt.Sprintf("upgrade gotm to %s", latest),
	) {
		return nil
	}
	if err := self.Install(ctx, tag, func(asset string) { print("gotm: asset fetching [%s]...", asset) }); err != nil {
		return failed(err)
	}
	printPass("upgraded gotm to %s", latest)
	return nil
}
