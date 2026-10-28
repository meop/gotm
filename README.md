# gotm

Go Toolchain Manager. Go toolchains side by side, each kept at the release you
pin.

It needs no go to start with, and nothing but itself: the release list and every
checksum come from [go.dev](https://go.dev/dl/), not GitHub or git.

## Install

macOS and Linux:

```sh
curl --fail-with-body --location --no-progress-meter --url https://raw.githubusercontent.com/meop/gotm/main/install.sh | sh
```

Windows:

```powershell
irm -ErrorAction Stop -Uri https://raw.githubusercontent.com/meop/gotm/main/install.ps1 | iex
```

Then put `~/.gotm/bin` on your `PATH`. It holds `gotm` and the `go` and `gofmt`
it installs.

## Usage

```sh
gotm add             # follow go's newest stable release, as go and gofmt
gotm add latest      # the same, said aloud
gotm add 1.26        # follow the newest 1.26.x, as go@1.26 and gofmt@1.26
gotm add 1.26.4      # stay on exactly 1.26.4, as go@1.26.4 and gofmt@1.26.4
gotm add latest 1.26 # add several at once
gotm add --reinstall 1.26 # reinstall even if already installed

gotm find            # the newest release of each line go has for this machine
gotm find 1.26       # every 1.26 release, candidates and betas among them
gotm list            # installed toolchains, their pins, and their commands
gotm list 1.26       # only the ones a term starts the pin or release of
gotm list -l         # just the names, one per line, for a script
gotm outdated        # the ones a newer release is out for

gotm sync            # update every toolchain that follows latest or a line
gotm sync 1.26       # update only that one
gotm sync --reinstall # reinstall even if already at latest version

gotm remove 1.26     # remove one
gotm tidy            # remove unused cached archives and orphaned items
gotm tidy --all      # remove all cached archives

gotm upgrade         # upgrade gotm itself
```

gotm is [ghpm](https://github.com/meop/ghpm) for a single package. The
package is always `go`, so a command is given only the pin: `gotm add 1.26` is
what `ghpm add go@1.26` would be. The key ghpm would use (`go`, `go@1.26`) is
what `list` prints, and is taken wherever a pin is.

### Global flags

| Flag | Description |
|---|---|
| `--dry-run`, `-n` | Print what would be done without executing |
| `--yes`, `-y` | Skip confirmation prompts |
| `--verbose`, `-v` | Also print the checks and syncs of gotm's vendored tools, and more detail on their errors |
| `--non-interactive` | Never prompt; for scripted use |

### Per-command flags

| Flag | Commands | Description |
|---|---|---|
| `--reinstall`, `-r` | `add`, `sync` | Reinstall even if already installed / already at latest version |
| `--force`, `-f` | `add`, `sync` | Replace a file in `~/.gotm/bin` that gotm did not put there |
| `--all` | `tidy` | Remove all cached archives regardless of installation status |
| `--long-names`, `-l` | `list`, `find`, `outdated` | Print names only, one per line, and nothing when there are none |
| `--short-names`, `-s` | `list`, `find`, `outdated` | Print names only, space-separated on one line, and nothing when there are none |

### Version pinning

| Syntax | Name | Meaning | Updates |
|---|---|---|---|
| `gotm add` | `go` | Latest version | Yes — `gotm sync` fetches the newest stable release |
| `gotm add 1` | `go@1` | Latest 1.x | Yes — within major only |
| `gotm add 1.26` | `go@1.26` | Latest 1.26.x | Yes — within major.minor only |
| `gotm add 1.26.4` | `go@1.26.4` | Exact version | Never — static pin |

A pinned toolchain keeps the pin as its name (`go@1.26`, not `go@1.26.9`);
`gotm list` shows the version installed. Its commands carry the pin too
(`go@1.26`, `gofmt@1.26`), so toolchains sit beside each other and none
replaces another. There is no switching between them and no environment to
set: you run the one you mean.

## Behavior

- Nothing is installed until you add it, and `sync` only updates what was added.
- `find` lists only releases with an archive for this machine, which are the
  ones `add` can install here.
- Updating a toolchain unpacks the new release beside the old, points its
  commands at it, and only then removes the old one. If any step fails, the
  toolchain is put back as it was.
- Every archive is checked against the sha256 go.dev states before it is
  unpacked. It stays in `~/.gotm/download` while a toolchain is on that
  release, so a second pin on the same release is not downloaded again;
  `gotm tidy` removes the ones nothing is on.
- The commands in `~/.gotm/bin` are small [wedge](https://github.com/meop/wedge)
  shims that run the real ones where they are unpacked, so there are no
  symlinks or junctions.
- A release candidate is installed only when you name it (`gotm add 1.28rc1`).
- `gotm upgrade` replaces only the gotm the install script put in
  `~/.gotm/bin`. One installed another way (ghpm, `go install`) works the
  same and keeps the same `~/.gotm`, but is upgraded by what installed it.

A project whose `go.mod` asks for a newer go than the one you run is still
built by a toolchain go downloads for itself; that is go's doing, not gotm's.

## Verifying releases

Every release archive contains `LICENSE`. Verify the signed checksum manifest
with the public release key before checking downloaded artifacts:

```sh
gpg --verify SHA256SUMS.sig SHA256SUMS
sha256sum --ignore-missing --check SHA256SUMS
```

## License

[MIT](LICENSE)
