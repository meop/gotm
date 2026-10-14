// Package godl reads go's own release list from go.dev. It is the one source
// gotm asks what releases exist, which is newest, and what each archive's
// checksum is — no GitHub, no git, and no go installed yet.
package godl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/meop/golie/vers"

	"github.com/meop/gotm/internal/config"
)

// BaseURL is go's download site. A test points it at its own server.
var BaseURL = "https://go.dev/dl/"

// File is one downloadable file of a release.
type File struct {
	Filename string `json:"filename"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	Kind     string `json:"kind"`
}

// Release is one go release, named as go.dev names it ("go1.27.2").
type Release struct {
	Version string `json:"version"`
	Stable  bool   `json:"stable"`
	Files   []File `json:"files"`
}

// URL is where a file of a release is downloaded from.
func (f File) URL() string { return BaseURL + f.Filename }

var client = &http.Client{Timeout: 60 * time.Second}

// List fetches the release list. Without all it is the short list go.dev
// leads with — the stable releases still supported, newest first — which is
// all that following latest needs; with all it is every release ever made,
// for resolving a pin to an older line.
func List(ctx context.Context, all bool) ([]Release, error) {
	url := BaseURL + "?mode=json"
	if all {
		url += "&include=all"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	return releases, nil
}

// Resolve picks the release a pin names out of releases: the newest stable
// one for "latest", the newest stable one of a line for a minor pin ("1.26"),
// and exactly the named one for a full version ("1.26.4"), which may be a
// release candidate since naming it is asking for it.
func Resolve(releases []Release, pin string) (Release, error) {
	if pin == "" || pin == config.PinLatest {
		return newest(releases, func(r Release) bool { return r.Stable })
	}
	// a full release name is taken literally: "go1.28rc1" has no numeric form
	for _, r := range releases {
		if r.Version == pin || r.Version == "go"+pin {
			return r, nil
		}
	}
	c, err := vers.ParseConstraint(pin)
	if err != nil {
		return Release{}, err
	}
	if c.Level == vers.PinExact {
		return Release{}, fmt.Errorf("go has no release %s", pin)
	}
	return newest(releases, func(r Release) bool { return r.Stable && c.Matches(r.Version) })
}

func newest(releases []Release, keep func(Release) bool) (Release, error) {
	var kept []Release
	for _, r := range releases {
		if keep(r) {
			kept = append(kept, r)
		}
	}
	if len(kept) == 0 {
		return Release{}, fmt.Errorf("go has no matching release")
	}
	slices.SortStableFunc(kept, byNewest)
	return kept[0], nil
}

// order is what a release sorts by: its numbers, and whatever follows them
// ("rc1"), which is nothing for a stable release. vers compares numbers only,
// and reads "1.28rc1" as far as its 1.
func order(version string) (nums [3]int, pre string) {
	rest := Display(version)
	for i := range nums {
		end := 0
		for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
			end++
		}
		nums[i], _ = strconv.Atoi(rest[:end])
		rest = rest[end:]
		if !strings.HasPrefix(rest, ".") {
			break
		}
		rest = rest[1:]
	}
	return nums, rest
}

// byNewest sorts releases newest first, a release above its own candidates
// and betas, and a later candidate above an earlier one.
func byNewest(a, b Release) int {
	an, ap := order(a.Version)
	bn, bp := order(b.Version)
	if c := slices.Compare(bn[:], an[:]); c != 0 || ap == bp {
		return c
	}
	if ap == "" || bp == "" {
		return len(ap) - len(bp)
	}
	word := func(pre string) (string, int) {
		letters := strings.TrimRight(pre, "0123456789")
		n, _ := strconv.Atoi(pre[len(letters):])
		return letters, n
	}
	aw, ai := word(ap)
	bw, bi := word(bp)
	if c := strings.Compare(bw, aw); c != 0 {
		return c
	}
	return bi - ai
}

// Line is the line a release is of: its first two numbers, "1.26" for
// go1.26.4 and for go1.26rc1.
func Line(version string) string {
	nums, _ := order(version)
	return fmt.Sprintf("%d.%d", nums[0], nums[1])
}

// StartsWith reports whether term starts version at the end of a number:
// "1.2" starts 1.2, 1.2.1 and 1.2rc1, and never 1.26. Both are spelled
// without go.dev's "go".
func StartsWith(version, term string) bool {
	if term == "" || !strings.HasPrefix(version, term) {
		return false
	}
	return len(version) == len(term) || version[len(term)] < '0' || version[len(term)] > '9'
}

// Lines is the newest stable release of each line, newest line first: what
// there is to follow.
func Lines(releases []Release) []Release {
	sorted := slices.Clone(releases)
	slices.SortStableFunc(sorted, byNewest)
	var lines []Release
	seen := map[string]bool{}
	for _, r := range sorted {
		if line := Line(r.Version); r.Stable && !seen[line] {
			seen[line] = true
			lines = append(lines, r)
		}
	}
	return lines
}

// Found is every release a term names, newest first: the one "latest"
// resolves to, or all that the term starts (StartsWith), candidates and betas
// among them, since naming one is how it is added.
func Found(releases []Release, term string) []Release {
	if term == config.PinLatest {
		r, err := Resolve(releases, term)
		if err != nil {
			return nil
		}
		return []Release{r}
	}
	var found []Release
	for _, r := range releases {
		if StartsWith(Display(r.Version), term) {
			found = append(found, r)
		}
	}
	slices.SortStableFunc(found, byNewest)
	return found
}

// NeedsAll reports whether resolving pin takes the full release list rather
// than the short one.
func NeedsAll(pin string) bool {
	return pin != "" && pin != config.PinLatest
}

// Archive is the file of a release to unpack on this machine.
func (r Release) Archive() (File, error) {
	return r.ArchiveFor(runtime.GOOS, runtime.GOARCH)
}

// ArchiveFor is the file of a release to unpack on goos/goarch.
func (r Release) ArchiveFor(goos, goarch string) (File, error) {
	for _, f := range r.Files {
		if f.Kind == "archive" && f.OS == goos && f.Arch == goarch {
			return f, nil
		}
	}
	return File{}, fmt.Errorf("%s has no archive for %s/%s", r.Version, goos, goarch)
}

// Display is a release without go.dev's "go" prefix, as a user types it.
func Display(release string) string { return strings.TrimPrefix(release, "go") }
