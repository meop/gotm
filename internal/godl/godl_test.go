package godl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func rel(version string, stable bool) Release {
	return Release{Version: version, Stable: stable, Files: []File{
		{Filename: version + ".linux-amd64.tar.gz", OS: "linux", Arch: "amd64", Kind: "archive", SHA256: "aa"},
		{Filename: version + ".windows-amd64.zip", OS: "windows", Arch: "amd64", Kind: "archive", SHA256: "bb"},
		{Filename: version + ".windows-amd64.msi", OS: "windows", Arch: "amd64", Kind: "installer", SHA256: "cc"},
		{Filename: version + ".src.tar.gz", Kind: "source", SHA256: "dd"},
	}}
}

var releases = []Release{
	rel("go1.28rc1", false),
	rel("go1.27.2", true),
	rel("go1.27.10", true),
	rel("go1.27.9", true),
	rel("go1.26.9", true),
	rel("go1.26.10", true),
	rel("go1.20", true),
}

func TestResolve(t *testing.T) {
	for _, tc := range []struct{ pin, want string }{
		{"", "go1.27.10"},
		{"latest", "go1.27.10"},
		{"1", "go1.27.10"},
		{"1.26", "go1.26.10"},
		{"1.27", "go1.27.10"},
		{"1.27.2", "go1.27.2"},
		{"go1.27.2", "go1.27.2"},
		{"1.20", "go1.20"},
		// a release candidate is never what a line resolves to, only what is named
		{"1.28rc1", "go1.28rc1"},
	} {
		got, err := Resolve(releases, tc.pin)
		if err != nil {
			t.Errorf("Resolve(%q): %v", tc.pin, err)
			continue
		}
		if got.Version != tc.want {
			t.Errorf("Resolve(%q) = %s, want %s", tc.pin, got.Version, tc.want)
		}
	}
	for _, pin := range []string{"1.28", "1.99.0", "2", "nonsense"} {
		if got, err := Resolve(releases, pin); err == nil {
			t.Errorf("Resolve(%q) = %s, want an error", pin, got.Version)
		}
	}
}

func TestArchiveFor(t *testing.T) {
	r := rel("go1.27.2", true)
	f, err := r.ArchiveFor("windows", "amd64")
	if err != nil || f.Filename != "go1.27.2.windows-amd64.zip" {
		t.Fatalf("windows/amd64 = %q, %v: the installer is not the archive", f.Filename, err)
	}
	if _, err := r.ArchiveFor("plan9", "amd64"); err == nil {
		t.Fatal("plan9/amd64 has no archive here, and no error said so")
	}
}

func TestList(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		_, _ = w.Write([]byte(`[{"version":"go1.27.2","stable":true,"files":[{"filename":"go1.27.2.linux-amd64.tar.gz","os":"linux","arch":"amd64","sha256":"aa","size":3,"kind":"archive"}]}]`))
	}))
	defer srv.Close()
	old := BaseURL
	BaseURL = srv.URL + "/"
	defer func() { BaseURL = old }()

	got, err := List(context.Background(), false)
	if err != nil || len(got) != 1 || got[0].Files[0].SHA256 != "aa" {
		t.Fatalf("List = %+v, %v", got, err)
	}
	if query != "mode=json" {
		t.Errorf("short list asked %q", query)
	}
	if _, err := List(context.Background(), true); err != nil || query != "mode=json&include=all" {
		t.Errorf("full list asked %q, %v", query, err)
	}
	if got[0].Files[0].URL() != srv.URL+"/go1.27.2.linux-amd64.tar.gz" {
		t.Errorf("file url = %s", got[0].Files[0].URL())
	}
}
