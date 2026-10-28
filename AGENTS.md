# AGENTS.md

gotm is a stripped down ghpm for a single package, go, with a behavior taken
from uv or rustup only where a toolchain manager has to differ from a package
manager. The commands, the pinning, the manifest, the disk layout, the output
and the conventions are ghpm's on purpose, and most of the code behind them is
shared with it (see **golie**), so read ghpm's AGENTS.md for the reasoning
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
Never weaken or bypass this guard: `sync`, `remove` and `tidy` delete toolchains.

`GOTM_TEST_HOME` also redirects the real binary, which is how to try a change
by hand without touching `~/.gotm`.

## Comments

No comment explains why code does what it does where a unit test can show it: the test is the explanation, and it
fails when the reasoning stops holding. A comment is only for what no test can show.

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
  mutating flow they share (`apply`); `find.go` lists go.dev's releases;
  `list.go` holds `list` and `outdated`; `remove.go` and `tidy.go` are ghpm's
  over `~/.gotm`; `state.go` holds what an argument is, and what the commands
  ask of the disk; `table.go` holds the messages, the gate and the closing
  failure table; `upgrade.go` is gotm upgrading itself
- `internal/config/` — gotm's names for golie's manifest, the one package
  (`go`) and its source (`go.dev/dl`), and gotm's lock over `~/.gotm`
- `internal/godl/` — go.dev's release list: fetch, resolve a pin to a release,
  order releases, pick the ones a term names, pick the archive for a platform.
  The only source of go's releases and checksums, and the only part of gotm
  that is not ghpm's shape
- `internal/store/` — where `~/.gotm` is; the layout inside it is golie's

## golie

Everything gotm shares with ghpm lives in [golie](https://github.com/meop/golie),
a separate module, and that is most of it: `manifest` (the manifest's format,
keys, pin levels and shim names), `layout` (the directories under `~/.gotm`),
`journal` (commit and rollback), `tidy` (what is broken or orphaned, and its
removal), `ui` (output and prompts), `vers` (pins and comparison), `fetch`
(the checked download and extraction), `pinned` and `wedge` (the vendored
mallet, kept at its pin by content), `shim`, `upgrade`, `lock` and `home`.
golie knows nothing of go: where ghpm passes a package name, gotm passes
`go`. A change to any of those is made there, released there (bump its
`VERSION`), and taken here with `go get github.com/meop/golie@v<VERSION>`.
What is only gotm's — go.dev, what an argument is, the commands and what they
print — stays here.

## Where gotm differs from ghpm

Each of these is deliberate, and the list is meant to stay this short. Before
adding a command, a flag or a message, find ghpm's and use its shape and its
words, with `toolchain` for `package`.

- **No package argument.** The package is always `go`, so a command takes
  only the pin: `gotm add 1.26` is `ghpm add go@1.26`, and the key is
  `go@1.26` either way. The key itself is accepted too, since `list` prints it
- **A bare `add` adds `go`.** ghpm's `add` needs a name; gotm's has the only
  one there is, so with no argument it installs the toolchain that follows the
  newest release, as a bare `uv python install` does. `latest` says the same
  thing aloud
- **`find` searches releases, not packages.** ghpm's registry is a list of
  packages; gotm's one package has a list of releases, which is what there is
  to find. No argument lists the newest release of each line, as
  `uv python list` does
- **Nothing is chosen during an install.** A go release has one archive per
  platform and a fixed `bin`, so there is no asset prompt, no bin prompt, no
  second gate, and nothing recorded as declined
- **No GitHub.** Releases and checksums are go.dev's, so there is no `gh`, no
  `info`, `download`, `refresh` or `doctor`, and no `--skip-hash-check`

## Manifest and disk layout

Both are ghpm's, through golie. `~/.gotm/manifest.json`:

```json
{
  "repo": { "go": "go.dev/dl" },
  "extract": {
    "go":      { "pin": "latest", "version": "1.27.2", "assets": ["go1.27.2.linux-amd64.tar.gz"], "bin": { "go": "bin/go", "gofmt": "bin/gofmt" } },
    "go@1.26": { "pin": "minor",  "version": "1.26.9", "assets": ["go1.26.9.linux-amd64.tar.gz"], "bin": { "go@1.26": "bin/go", "gofmt@1.26": "bin/gofmt" } }
  }
}
```

A key is `go`, or `go@<pin>`; `pin` is the level the pin holds it to (`latest`,
`major`, `minor`, `fixed`); `version` is the release it is on, without
go.dev's `go` prefix; `bin` maps each shim, named as it sits on disk, to the
command's path inside the extract dir.

- `~/.gotm/bin/` — gotm and the shims; the one directory that goes on `PATH`
- `~/.gotm/extract/<key>/<version>/` — a go tree, its GOROOT. One per install:
  two pins on the same release are two trees, as two ghpm installs are
- `~/.gotm/download/go.dev/dl/<version>/` — the archive, kept while an install
  is on that release and removed by `tidy` once none is
- `~/.gotm/vendor/wedge/` — `mallet` and its templates; never on `PATH`

## Key flows

The flow model is ghpm's: resolve what was asked → a **gate table** + confirm
→ progress lines and a `✓ N` line, with a `✗ N toolchain(s) failed`
name/reason table whenever anything failed.

**Arguments** — a command that **acts** (`add`, `sync`, `remove`) takes a pin
whole (`parsePin`): `latest`, a line, or one release, spelled bare (`1.26`)
or as its key (`go@1.26`; `go` is `latest`). A command that **reads** (`find`,
`list`, `outdated`) takes the start of a version (`godl.StartsWith`: `1.2`
starts `1.2.1` and `1.2rc1`, never `1.26`) — `findKeys` puts it to an
install's pin and to the release it is on, `godl.Found` to go.dev's releases.
Do not add a third comparison.

**Add** — per argument (none is `latest`): already added says so unless
`--reinstall` → resolve the pin against go.dev → **gate table**
(name/version/pin/uri) + confirm → `apply`.

**Sync** — updates what was added and nothing else: with nothing installed it
says so. Every install that is not `fixed` (all, or the ones named; a named
fixed one is said and skipped): resolve → a newer release, or any with
`--reinstall` → **gate table** (name/version/update/pin/uri) + confirm → `apply`.

**apply** (one toolchain) — sync the vendored wedge once per command → fetch
the archive into the download cache unless a copy with the right sha256 is
there → unpack it beside its extract dir and rename it in → stamp a shim for
every command in the tree's `bin` → write the manifest entry → remove shims
the new release no longer has, and the old extract dir.

The manifest is the commit point, and the steps before it are journalled
(`golie/journal`): a failure rolls them back to the release the manifest still
names. The old extract dir is deleted only after the commit, because that
rollback restores from it.

**Find** — go.dev's full release list, kept to the releases with an archive
for this machine (what `add` can install here). No term: the newest stable
release of each line. A term: every release it starts, newest first,
candidates and betas included, since naming one is how it is added. A `name`
column shows the installs on a release, and is dropped when there are none.
Names mode prints the versions.

**List / Outdated** — ghpm's tables: a row per shim
(name/version/pin/uri/type/target, with `update` for outdated), optionally
filtered by terms. `uri` is always `go.dev/dl`; it is kept so the tables are
the ones ghpm prints. `outdated` leaves fixed installs out.

**Remove** — gate → a toolchain's shims, then its extract dir, then its
manifest entry. Nothing here can be rolled back, so the order is the safety: a
shim that will not delete leaves the toolchain fully described for the retry.
Its cached archive is left for `tidy`.

**Tidy** — ghpm's, by `golie/tidy`: broken installs, orphaned shims, orphaned
extract dirs (a crashed unpack's `<version>.tmp` among them) and orphaned
downloads, each shown and confirmed on its own; `--all` empties the download
cache.

**Upgrade** — gotm and nothing else. The newest release is read from where
`releases/latest` redirects, the archive is checked against that release's
`SHA256SUMS`, and the running binary is replaced (moved aside first on
Windows, which will not overwrite a running executable). Only the gotm in
`~/.gotm/bin`, where the install script puts it, is replaced, reached directly
or through a symlink. One that runs from anywhere else — ghpm's, `go
install`'s, a build — was put there by something else, which is what upgrades
it: it is refused in one line (`✗ gotm: cannot upgrade itself, running from
<path>`, off `golie/upgrade`'s `Check`) before the release page is asked
anything. Everything else such a gotm does is unchanged: its
state is still `~/.gotm` and its shims still go in `~/.gotm/bin`, it is only
not in there itself.

**Vendored wedge** — golie pins it: a version and the sha256 of each
platform's archive and of every file kept from it. Before anything is stamped,
the vendored files are checked against that pin by content, not by the version
`mallet` reports, so a copy from another gotm, a damaged one and a swapped one
all fail the same check and are replaced from a plain download of the pinned
release. The sync is **silent** — on a match, a fetch and a replace alike —
since wedge is internals, and a line about it would only confuse someone who
asked for a toolchain. Only a failure is shown. Never wire `pinned.Options.Fetching` or
`Replacing` to a print. Taking a newer golie is the only way it moves; it is
not part of `upgrade`.

## Conventions

- go's releases and checksums come from go.dev only. No GitHub API call
  anywhere: wedge and gotm's own upgrade are plain release downloads
- `--reinstall` and `--force` are two flags because they are two things to
  ask for, and neither implies the other. `--reinstall` redoes gotm's own
  work — an install already there, a toolchain already at the newest release —
  and touches nothing that is not gotm's. `--force` is the only way a shim
  replaces a file in `~/.gotm/bin` that `shim.IsShim` does not say gotm
  stamped, which is the one destructive thing either command can do. Both
  mean the same on `add` and on `sync`, and the same in ghpm
- A flag is one thing to ask for, means the same on every command that has
  it, and is ghpm's when ghpm has it. Do not copy a flag from uv, rustup or
  anything else because it is there: take what is cleanest, and change both
  tools together
- Names mode prints names and nothing else: `--long-names` / `--short-names`
  on `list`, `find` and `outdated` are output for a script, so every status
  line is silent under them and an empty result is empty output — never
  `no toolchains installed`, which a script would have to recognise by its
  wording. `list` and `outdated` print keys and `find` prints versions, since
  those are what every other command takes
- Output is ghpm's, word for word where the thing said is the same:
  `no toolchains installed`, `no toolchains matched`, `all toolchains are up
  to date`, `<name>: not installed`, `<name>: already added → <version>`,
  `<name>: fixed at <version>, skipping`, `<name>: asset fetching [<asset>]...`,
  `<name>: bin found [<bin>]`, confirms that count (`add 2 toolchain(s)`,
  `update …`, `uninstall …`, `upgrade gotm to <version>`), and
  `✓ installed|updated|uninstalled N toolchain(s)`
- Table colours are ghpm's roles: `new` on the version an add or update goes
  to, `old` on the one it leaves, `info` on the version in `list` and `remove`
