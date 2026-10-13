# gotm

Go Toolchain Manager. Go toolchains side by side, each kept at the release you
pin.

It needs no go to start with, and nothing but itself: the release list and every
checksum come from [go.dev](https://go.dev/dl/), not GitHub or git.

## Install

macOS and Linux:

```sh
curl --fail-with-body --location --url https://raw.githubusercontent.com/meop/gotm/main/install.sh | sh
```

Windows:

```powershell
irm https://raw.githubusercontent.com/meop/gotm/main/install.ps1 | iex
```

Then put `~/.gotm/bin` on your `PATH`. It holds `gotm` and the `go` and `gofmt`
it installs.

## Use

```sh
gotm add             # follow go's newest stable release, as go and gofmt
gotm add 1.26        # follow the newest 1.26.x, as go1.26 and gofmt1.26
gotm add 1.26.4      # stay on exactly 1.26.4, as go1.26.4 and gofmt1.26.4
gotm sync            # update every added toolchain to the newest release its pin allows
gotm sync 1.26       # update only that one
gotm list            # the added toolchains, their pins, and their commands
gotm outdated        # the ones a newer release is out for
gotm remove 1.26     # remove one
gotm upgrade         # upgrade gotm itself
```

| Flag | What it does |
|------|--------------|
| `--dry-run`, `-n` | Show what would change, and change nothing |
| `--yes`, `-y` | Answer yes to the confirmation |
| `--quiet`, `-q` | Print only tables and errors |
| `--non-interactive` | Never prompt; for scripted use |
| `--force` | On `add` and `sync`: replace a file in `~/.gotm/bin` that gotm did not put there |

## What it does

- Nothing is installed until you add it, and `sync` only updates what was added.
- Added toolchains sit beside each other. The one following `latest` is `go` and
  `gofmt`; any other carries its pin in its name, so `go1.26` never replaces
  `go`. There is no switching between them and no environment to set: you run
  the one you mean.
- Updating a toolchain unpacks the new release beside the old, points its
  commands at it, and only then removes the old one.
- Every archive is checked against the sha256 go.dev states before it is unpacked.
- The commands in `~/.gotm/bin` are small [sheesh](https://github.com/meop/sheesh)
  shims that run the real ones where they are unpacked, so there are no
  symlinks or junctions.
- A release candidate is installed only when you name it (`gotm add 1.28rc1`).

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
