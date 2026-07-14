// itapu is the command-line interface for the Itapu secrets platform.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/itapulab/itapu-cli/internal/cmd"
	"github.com/itapulab/itapu-cli/internal/prompt"
	"github.com/itapulab/itapu-cli/internal/ui"
	"github.com/itapulab/itapu-cli/internal/update"
)

// version is stamped at build time (goreleaser on releases, the Makefile on
// local builds); "dev" disables the update check and `itapu update` warns.
var version = "dev"

const usage = `itapu — secrets for your development workflow

Usage:
  itapu <command> [flags]

Commands:
  login      Authenticate with the Itapu web app (device-code flow in the
             browser; stores an account token in ~/.config/itapu)
  init       Grant the current repo access to a project environment and
             write .itapu.json (no secrets in it; gitignored automatically)
  run        Run a command with your secrets injected into its environment,
             fetched fresh on every invocation — nothing written to disk
  update     Update itapu to the latest release
  version    Print the CLI version
  help       Show this help

Flags:
  login   --base-url=<origin>   Itapu web app origin (or $ITAPU_BASE_URL)
  init    --env=<slug>          Environment to grant (default: dev)
  run     --project=<name|id>   Select a project when several are configured

Examples:
  itapu login
  itapu init --env=staging
  itapu run -- pnpm dev
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	command := os.Args[1]
	var err error
	switch command {
	case "login":
		err = cmd.Login(os.Args[2:])
	case "init":
		err = cmd.Init(os.Args[2:])
	case "run":
		var code int
		code, err = cmd.Run(os.Args[2:])
		if err == nil {
			os.Exit(code)
		}
	case "update":
		err = cmd.Update(version, os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("itapu", version)
	case "help", "--help", "-h":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "itapu: unknown command %q\n\n%s", command, usage)
		os.Exit(2)
	}

	if err != nil {
		if errors.Is(err, prompt.ErrCancelled) {
			fmt.Fprintln(os.Stderr, ui.Faint("Cancelled."))
			os.Exit(130)
		}
		fmt.Fprintln(os.Stderr, ui.Error(err.Error()))
		os.Exit(1)
	}

	// Passive update hint, throttled to once a day. Never for `run` (its
	// stderr belongs to the child process — and its success path exits
	// above anyway), `update` itself, or non-interactive use.
	if command != "update" && ui.Interactive() {
		if notice := update.Notice(version); notice != "" {
			fmt.Fprintln(os.Stderr, ui.Faint(notice))
		}
	}
}
