package cli

import "testing"

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
