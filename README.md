# itapu CLI

Command-line interface for the Itapu secrets platform.

## Install

macOS / Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/itapulab/itapu-cli/main/install.sh | sh
```

This installs the latest release to `~/.local/bin/itapu` (set
`ITAPU_INSTALL_DIR` to change it). Prebuilt binaries for every platform,
Windows included, are on the [releases page](https://github.com/itapulab/itapu-cli/releases).

## Stay up to date

```sh
itapu update
```

`update` downloads the latest release for your platform, verifies it against
the published checksums, and swaps the binary in place atomically. If you are
already on the latest version it says so and does nothing.

The CLI also checks for new releases passively: at most once a day (cached in
`~/.config/itapu/update-check.json`, 2-second timeout, never blocks or breaks
a command) and, when a newer release exists, prints a one-line hint to stderr.
The check is skipped for non-interactive sessions, for `itapu run` (its output
belongs to your command), and for dev builds.

## Build from source

```sh
make build     # ./itapu in the repo (or: go build -o itapu .)
make install   # build straight into ~/.local/bin (override: ITAPU_INSTALL_DIR)
make test      # run the test suite
```

Requires Go 1.26+. Interactive prompts and styling come from the
[Charm](https://charm.sh) libraries (`huh`, `lipgloss`); everything else is
standard library. Releases are cross-compiled and published by
[GoReleaser](https://goreleaser.com) via GitHub Actions on every `v*` tag.

## Publish a release

```sh
make release-patch   # v0.3.0 → v0.3.1
make release-minor   # v0.3.0 → v0.4.0
make release-major   # v0.3.0 → v1.0.0
```

The tag is the version — there is no version file to bump. The command
(`scripts/release.sh`) checks you are on a clean `main` in sync with origin,
shows the commits since the last tag, asks for confirmation, then tags and
pushes; GitHub Actions runs GoReleaser and publishes the binaries. Add
`--dry-run` (e.g. `scripts/release.sh patch --dry-run`) to preview the
version it would tag. Users pick the release up via `itapu update` and the
daily update hint.

## Usage

```sh
itapu login                # device-code login, stores a 6-day account token
itapu init [--env=<slug>]  # link this repo to one project + environment (default: dev),
                           # stores an 8-hour secrets token and writes .itapu.json
itapu run -- pnpm dev      # fetch secrets and run the command with them injected
itapu update               # self-update to the latest release
itapu version              # print the CLI version
itapu help                 # full command and flag reference
```

The API origin defaults to `https://itapu.vercel.app`; override with
`itapu login --base-url=<origin>` or the `ITAPU_BASE_URL` environment
variable (useful for local development against `http://localhost:3000`).

## Where things are stored

| File                                        | Contents                                    | Commit?                    |
| ------------------------------------------- | ------------------------------------------- | -------------------------- |
| `~/.config/itapu/config.json` (mode `0600`) | account + secrets tokens, base URL          | never                      |
| `.itapu.json` in the repo                   | org/project/environment ids only, no tokens | no — per-developer, `itapu init` gitignores it |

`.itapu.json` contains no secrets, but every `itapu init` rewrites it with
that developer's own project/environment selection, so it is per-developer
state: `itapu init` appends it to your `.gitignore` automatically (when run
inside a git repository and not already ignored).

Secret values are never written to disk — `itapu run` fetches them per
invocation and injects them into the child process environment (secrets win
over inherited variables).

Notes from the API contract:

- There is only ever one valid secrets token per user. Claiming a new one
  (`itapu init`) revokes the previous one, but the still-valid previous
  token's grants are carried over into the new token (each expansion is
  approved in the browser), so other initialized folders on the *same
  machine* keep working. Other machines still need `itapu init` again.
- When the current token already covers the selected project/environment,
  `itapu init` skips the browser approval and only (re)writes `.itapu.json`.
- `itapu run` selects the project from `.itapu.json` automatically; when
  several are configured, it matches the current directory name or takes
  `--project=<name|id>`.
- On `token_expired` / `token_revoked`, the CLI tells you which command to
  rerun (`itapu login` for the account token, `itapu init` for the secrets
  token).
