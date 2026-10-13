package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/meop/golie/ui"

	"github.com/meop/gotm/internal/config"
)

func TestNormalizePin(t *testing.T) {
	for arg, want := range map[string]string{
		"": "latest", "latest": "latest", "LATEST": "latest",
		"1.26": "1.26", "go1.26": "1.26", "1.26.4": "1.26.4", "go1.28rc1": "1.28rc1",
	} {
		if got, err := normalizePin(arg); err != nil || got != want {
			t.Errorf("normalizePin(%q) = %q, %v; want %q", arg, got, err, want)
		}
	}
	for _, arg := range []string{"nonsense", "go", "v1.26"} {
		if got, err := normalizePin(arg); err == nil {
			t.Errorf("normalizePin(%q) = %q, want an error", arg, got)
		}
	}
}

// the toolchain following latest owns the plain names, and every other pin
// carries its own, so no toolchain's shims replace another's
func TestShimName(t *testing.T) {
	for _, tc := range []struct{ bin, pin, want string }{
		{"go", "latest", "go"},
		{"gofmt", "latest", "gofmt"},
		{"go", "1.26", "go1.26"},
		{"gofmt", "1.26.4", "gofmt1.26.4"},
	} {
		if got := shimName(tc.bin, tc.pin); got != tc.want {
			t.Errorf("shimName(%q, %q) = %q, want %q", tc.bin, tc.pin, got, tc.want)
		}
	}
}

// names mode is for a script: it prints the pins, which are what every other
// command takes, and with nothing added it prints nothing at all rather than
// a sentence the script would have to know
func TestListNamesMode(t *testing.T) {
	t.Setenv("GOTM_TEST_HOME", t.TempDir())
	var names, prose bytes.Buffer
	stdout = &names
	ui.SetOutput(&prose)
	t.Cleanup(func() { stdout = os.Stdout; ui.SetOutput(os.Stdout) })
	list := newListCmd()

	run := func(flag *bool) string {
		names.Reset()
		prose.Reset()
		if flag != nil {
			*flag = true
			defer func() { *flag = false }()
		}
		if err := list.RunE(list, nil); err != nil {
			t.Fatal(err)
		}
		return names.String() + prose.String()
	}

	if got := run(nil); got != msgNoneAdded+"\n" {
		t.Errorf("without the flag the empty list is said, got %q", got)
	}
	for _, flag := range []*bool{&longNames, &shortNames} {
		if got := run(flag); got != "" {
			t.Errorf("names mode printed %q with nothing added", got)
		}
	}

	m := &config.Manifest{Toolchains: map[string]config.Toolchain{
		"1.26":   {Version: "go1.26.9", Bin: []string{"go1.26", "gofmt1.26"}},
		"latest": {Version: "go1.27.2", Bin: []string{"go", "gofmt"}},
	}}
	if err := config.SaveManifest(m); err != nil {
		t.Fatal(err)
	}
	if got := run(&longNames); got != "latest\n1.26\n" {
		t.Errorf("--long-names printed %q", got)
	}
	if got := run(&shortNames); got != "latest 1.26\n" {
		t.Errorf("--short-names printed %q", got)
	}
}
