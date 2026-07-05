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

## Build from source

```sh
go build -o itapu .
```

Pure standard library — no external dependencies. Releases are
cross-compiled and published by [GoReleaser](https://goreleaser.com) via
GitHub Actions on every `v*` tag.

## Usage

```sh
itapu login                # device-code login, stores a 6-day account token
itapu init [--env=<slug>]  # grant this repo an environment (default: dev),
                           # stores an 8-hour secrets token and writes .itapu.json
itapu run -- pnpm dev      # fetch secrets and run the command with them injected
```

The API origin defaults to `https://app.itapu.dev`; override with
`itapu login --base-url=<origin>` or the `ITAPU_BASE_URL` environment
variable (useful for local development against `http://localhost:3000`).

## Where things are stored

| File | Contents | Commit? |
| --- | --- | --- |
| `~/.config/itapu/config.json` (mode `0600`) | account + secrets tokens, base URL | never |
| `.itapu.json` in the repo | org/project/environment ids only, no tokens | safe |

Secret values are never written to disk — `itapu run` fetches them per
invocation and injects them into the child process environment (secrets win
over inherited variables).

Notes from the API contract:

- Claiming a new secrets token (`itapu init`) revokes all your previous
  ones — other repos on other machines will need `itapu init` again.
- `itapu run` selects the project from `.itapu.json` automatically; when
  several are configured, it matches the current directory name or takes
  `--project=<name|id>`.
- On `token_expired` / `token_revoked`, the CLI tells you which command to
  rerun (`itapu login` for the account token, `itapu init` for the secrets
  token).
