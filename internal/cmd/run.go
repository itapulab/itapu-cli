package cmd

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/itapulab/itapu-cli/internal/api"
	"github.com/itapulab/itapu-cli/internal/config"
	"github.com/itapulab/itapu-cli/internal/ui"
)

// Run implements `itapu run [--project=<name|id>] -- <command>`: fetches the
// secrets of the granted environment and spawns the command with them
// injected (secrets win over inherited variables). Secrets are never written
// to disk. Returns the child's exit code.
func Run(args []string) (int, error) {
	// Split our flags from the child command at "--".
	ourArgs := args
	var childArgs []string
	for i, a := range args {
		if a == "--" {
			ourArgs = args[:i]
			childArgs = args[i+1:]
			break
		}
	}

	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	projectFlag := fs.String("project", "", "project name or id (when .itapu.json holds several)")
	if err := fs.Parse(ourArgs); err != nil {
		return 1, err
	}
	// Allow `itapu run pnpm dev` without the `--` separator too.
	if childArgs == nil {
		childArgs = fs.Args()
	}
	if len(childArgs) == 0 {
		return 1, errors.New("no command given — usage: itapu run -- <command> [args...]")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return 1, err
	}
	proj, cfgPath, err := config.FindProject(cwd)
	if err != nil {
		return 1, err
	}
	grant, err := selectGrant(proj, *projectFlag, cwd)
	if err != nil {
		return 1, err
	}

	cfg, err := config.LoadUser()
	if err != nil {
		return 1, err
	}
	if cfg.SecretsToken == "" {
		return 1, errors.New("no secrets token found — run `itapu init` first")
	}
	if !cfg.SecretsTokenExpiresAt.IsZero() && time.Now().After(cfg.SecretsTokenExpiresAt) {
		return 1, errors.New("your secrets token has expired — run `itapu init` again")
	}

	client := api.New(config.ResolveBaseURL("", cfg), cfg.SecretsToken)
	resp, err := client.Secrets(grant.EnvironmentID)
	if err != nil {
		if apiErr := asAPIError(err); apiErr != nil {
			switch apiErr.Code {
			case "invalid_token", "token_revoked", "token_expired":
				return 1, fmt.Errorf("%s — run `itapu init` again", apiErr.Message)
			case "environment_not_granted":
				return 1, fmt.Errorf("the environment in %s is not covered by your current token — run `itapu init` again", cfgPath)
			case "environment_access_denied":
				return 1, errors.New("your access to this environment was revoked — ask an admin, then run `itapu init` again")
			}
		}
		return 1, err
	}

	info(ui.Faint(fmt.Sprintf("itapu: injecting %d secrets from %s/%s (%s)",
		len(resp.Secrets), resp.Environment.ProjectName, resp.Environment.Slug, resp.Environment.Name)))

	return spawn(childArgs, resp.Secrets)
}

// selectGrant picks the grant to use: the only one, the --project match, or
// the one whose project name matches the current directory name.
func selectGrant(proj *config.ProjectConfig, projectFlag, cwd string) (*config.ProjectGrant, error) {
	if len(proj.Projects) == 0 {
		return nil, errors.New(".itapu.json has no projects — run `itapu init` again")
	}
	if len(proj.Projects) == 1 {
		return &proj.Projects[0], nil
	}
	if projectFlag != "" {
		for i := range proj.Projects {
			if proj.Projects[i].ProjectName == projectFlag || proj.Projects[i].ProjectID == projectFlag {
				return &proj.Projects[i], nil
			}
		}
		return nil, fmt.Errorf("no project %q in .itapu.json", projectFlag)
	}
	dirName := filepath.Base(cwd)
	for i := range proj.Projects {
		if proj.Projects[i].ProjectName == dirName {
			return &proj.Projects[i], nil
		}
	}
	var names []string
	for _, p := range proj.Projects {
		names = append(names, p.ProjectName)
	}
	return nil, fmt.Errorf("several projects configured (%s) — pick one with --project=<name>", strings.Join(names, ", "))
}

// spawn runs the command with os.Environ() merged with the secrets
// (secrets win), forwarding termination signals to the child.
func spawn(argv []string, secrets []api.Secret) (int, error) {
	env := map[string]string{}
	var order []string
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			if _, exists := env[k]; !exists {
				order = append(order, k)
			}
			env[k] = v
		}
	}
	for _, s := range secrets {
		if _, exists := env[s.Key]; !exists {
			order = append(order, s.Key)
		}
		env[s.Key] = s.Value
	}
	sort.Strings(order)
	merged := make([]string, 0, len(order))
	for _, k := range order {
		merged = append(merged, k+"="+env[k])
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = merged
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Ctrl-C reaches the child through the foreground process group; the
	// parent just waits. Forward direct SIGTERM/SIGHUP (e.g. from kill).
	signal.Ignore(os.Interrupt)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGHUP)

	if err := cmd.Start(); err != nil {
		return 1, fmt.Errorf("cannot start %q: %w", argv[0], err)
	}
	go func() {
		for s := range sigCh {
			_ = cmd.Process.Signal(s)
		}
	}()

	err := cmd.Wait()
	signal.Stop(sigCh)
	close(sigCh)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}
