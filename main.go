// itapu is the command-line interface for the Itapu secrets platform.
package main

import (
	"fmt"
	"os"

	"github.com/itapulab/itapu-cli/internal/cmd"
)

var version = "0.1.0"

const usage = `itapu — secrets for your development workflow

Usage:
  itapu login                Authenticate with the Itapu web app
  itapu init [--env=<slug>]  Grant this repo access to an environment (default: dev)
  itapu run -- <command>     Run a command with secrets injected into its env
  itapu version              Print the CLI version

Flags:
  login: --base-url=<origin>   Itapu web app origin (or $ITAPU_BASE_URL)
  run:   --project=<name|id>   Select a project when several are configured
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
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
	case "version", "--version", "-v":
		fmt.Println("itapu", version)
	case "help", "--help", "-h":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "itapu: unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "itapu:", err)
		os.Exit(1)
	}
}
