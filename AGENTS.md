# AGENTS.md

gotm is ghpm cut down to a single package: go. The shape, the output and the
conventions are ghpm's on purpose, so read ghpm's AGENTS.md for the reasoning
behind anything here that looks unexplained.

## After every code change

Run all four — do not skip any:

```sh
go build -o gotm ./cmd/gotm
go test ./...
gofmt -w .
golangci-lint run ./...
```

Tests that access gotm state must set `GOTM_TEST_HOME` to a fresh temporary
directory (`withHome(t)` in `internal/config`). The store package deliberately
refuses to resolve the real user home from a Go test binary when it is absent.
Never weaken or bypass this guard: `sync` and `remove` delete toolchains.

`GOTM_TEST_HOME` also redirects the real binary, which is how to try a change
by hand without touching `~/.gotm`.

## Keeping this file current

Update this file when a package is added, removed or renamed, when the manifest
or disk layout changes, or when a flow changes. Update `README.md` when
user-visible behavior changes.

## Cross-compile

```sh
GOOS=darwin  GOARCH=arm64 go build -o gotm-darwin-arm64      ./cmd/gotm
GOOS=darwin  GOARCH=amd64 go build -o gotm-darwin-amd64      ./cmd/gotm
GOOS=linux   GOARCH=arm64 go build -o gotm-linux-arm64       ./cmd/gotm
GOOS=linux   GOARCH=amd64 go build -o gotm-linux-amd64       ./cmd/gotm
GOOS=windows GOARCH=arm64 go build -o gotm-windows-arm64.exe ./cmd/gotm
GOOS=windows GOARCH=amd64 go build -o gotm-windows-amd64.exe ./cmd/gotm
```

## Publishing

To cut a release: bump the version in `VERSION` and push to `main`. The CI
pipeline (`.github/workflows/pipeline.yaml`) is ghpm's: version check → validate
(fmt + lint + test) → a GPG-signed tag `v<VERSION>` and `goreleaser release`.
It requires the `GPG_PRIVATE_KEY` and `GPG_PASSPHRASE` repository secrets.
Never create the tag by hand.

## Project structure

- `cmd/gotm/` — entry point
- `internal/cli/` — subcommands. `sync.go` holds `add`, `sync` and the one
  mutating flow they share (`apply`); `state.go` holds pin and shim naming and
  what the commands ask of the disk; `upgrade.go` is gotm upgrading itself
- `internal/config/` — the manifest, and gotm's lock over `~/.gotm`
- `internal/godl/` — go.dev's release list: fetch, resolve a pin to a release,
  pick the archive for a platform. The only source of go's releases and
  checksums
- `internal/store/` — `~/.gotm/` path helpers

## golie

Everything gotm shares with ghpm lives in [golie](https://github.com/meop/golie),
a separate module: `ui` (output and prompts), `vers` (pins and comparison),
`fetch` (the checked download and extraction), `pinned` and `sheesh` (the
vendored kebab, kept at its pin by content), `shim`, `upgrade`, `lock` and
`home`. A change to any of those is made there, released there (bump its
`VERSION`), and taken here with `go get github.com/meop/golie@v<VERSION>`. What
is only gotm's — go.dev, the manifest, the commands — stays here.

## Manifest and disk layout

`~/.gotm/manifest.json` records the toolchains that were added, keyed by pin —
`latest`, a line (`1.26`) or one release (`1.26.4`). Each entry holds `version`
(the release the pin resolved to and is unpacked, in go.dev's spelling,
`go1.27.2`) and `bin` (the shims stamped for it).

- `~/.gotm/bin/` — gotm and the shims; the one directory that goes on `PATH`.
  The toolchain following `latest` owns the plain names (`go`, `gofmt`); every
  other pin's shims carry the pin (`go1.26`, `gofmt1.26.4`), so toolchains
  never replace each other's
- `~/.gotm/toolchain/<release>/` — a go tree, its GOROOT. Shared by every pin
  that resolves to that release, and removed when the last of them leaves it
- `~/.gotm/vendor/sheesh/` — `kebab` and its templates; never on `PATH`
- `~/.gotm/download/` — an archive, only while it is fetched and unpacked

## Key flows

**Add** — per argument (none means `latest`): already added says so → resolve
the pin against go.dev → **gate table** (name/version/pin) + confirm → `apply`.

**Sync** — updates what was added and nothing else: with nothing added it says
so and installs nothing. Per added pin (all, or the ones named): resolve → a
newer release, or one the disk no longer bears out, is a change → **gate
table** (name/version/update/pin) + confirm → `apply`.

**apply** (one toolchain) — sync the vendored sheesh once per command →
download the archive unless the tree is already there, check its sha256,
unpack it beside its destination and rename it in → stamp a shim for every
command in the tree's `bin` → write the manifest → remove shims the new
release no longer has, and the old tree if no pin is on it.

The manifest is the commit point. A failure before it puts the shims back on
the tree the manifest still names and drops what the run unpacked. The old tree
is deleted only after the commit, because that restore needs it.

"Healthy" means the tree and every recorded shim are really there. A toolchain
that is not is redone by sync rather than believed.

**Remove** — gate → a toolchain's shims, then its tree unless another pin is on
it, then its manifest entry. Nothing here can be rolled back, so the order is
the safety: a shim that will not delete leaves the toolchain fully described
for the retry.

**Upgrade** — gotm and nothing else. The newest release is read from where
`releases/latest` redirects, the archive is checked against that release's
`SHA256SUMS`, and the running binary is replaced (moved aside first on
Windows, which will not overwrite a running executable).

**Vendored sheesh** — golie pins it: a version and the sha256 of each
platform's archive and of every file kept from it. Before anything is stamped,
the vendored files are checked against that pin by content, not by the version
`kebab` reports, so a copy from another gotm, a damaged one and a swapped one
all fail the same check and are replaced from a plain download of the pinned
release. A wrong copy of this very version is said (`‼ sheesh: kebab does not
match its pin, replacing`). Taking a newer golie is the only way it moves; it
is not part of `upgrade`.

## Conventions

- go's releases and checksums come from go.dev only. No GitHub API call
  anywhere: sheesh and gotm's own upgrade are plain release downloads
- A shim is replaced or removed only if `shim.IsShim` says gotm stamped it;
  anything else in `~/.gotm/bin` needs `--force`
- Names mode prints names and nothing else: `--long-names` / `--short-names`
  on `list` and `outdated` are output for a script, so every status line is
  silent under them and an empty result is empty output — never `no go added`,
  which a script would have to recognise by its wording. The names are pins,
  since a pin is what every other command takes
- Output follows ghpm: `<subject> <verb> [<name>]` info lines, a gate before any
  change, `✓` and `✗` only on summaries and errors, and a closing line on a dry run
