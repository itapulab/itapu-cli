package cmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/itapulab/itapu-cli/internal/api"
	"github.com/itapulab/itapu-cli/internal/config"
	"github.com/itapulab/itapu-cli/internal/prompt"
	"github.com/itapulab/itapu-cli/internal/ui"
)

// Init implements `itapu init [--env=<slug>]`: scopes the CLI to one org,
// one or more projects and a single environment, and stores an 8-hour
// secrets token in the user-level config (the token never touches
// .itapu.json, which is safe to commit).
func Init(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	envSlug := fs.String("env", "dev", "environment slug (never prompted)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.LoadUser()
	if err != nil {
		return err
	}
	accountToken, err := requireAccountToken(cfg)
	if err != nil {
		return err
	}
	client := api.New(config.ResolveBaseURL("", cfg), accountToken)

	ws, err := client.Workspace()
	if err != nil {
		return mapAuthError(err, "itapu login")
	}
	if len(ws.Organizations) == 0 {
		return errors.New("you don't belong to any organization yet — create one in the Itapu dashboard")
	}

	// Pick the organization (skip the prompt when there is only one).
	org := &ws.Organizations[0]
	if len(ws.Organizations) > 1 {
		labels := make([]string, len(ws.Organizations))
		for i, o := range ws.Organizations {
			labels[i] = fmt.Sprintf("%s (%s)", o.Name, o.Role)
		}
		idx, err := prompt.Select("Select an organization:", labels)
		if err != nil {
			return err
		}
		org = &ws.Organizations[idx]
	} else {
		info("Organization: %s %s", ui.Strong(org.Name), ui.Faint("("+org.Role+")"))
	}

	// A project qualifies when it has the resolved environment slug (the
	// workspace tree only lists environments the user can read).
	var qualifying []api.Project
	var excluded []string
	for _, p := range org.Projects {
		hasEnv := false
		for _, e := range p.Environments {
			if e.Slug == *envSlug {
				hasEnv = true
				break
			}
		}
		if hasEnv {
			qualifying = append(qualifying, p)
		} else {
			excluded = append(excluded, p.Name)
		}
	}
	if len(excluded) > 0 {
		info("\n" + ui.Warn(fmt.Sprintf("Skipping projects without access to a %q environment: %s",
			*envSlug, strings.Join(excluded, ", "))))
	}
	if len(qualifying) == 0 {
		return fmt.Errorf("no project in %s has a %q environment you can read", org.Name, *envSlug)
	}

	var picks []int
	if len(qualifying) == 1 {
		info("Project: %s", ui.Strong(qualifying[0].Name))
		picks = []int{0}
	} else {
		labels := make([]string, len(qualifying))
		for i, p := range qualifying {
			labels[i] = p.Name
		}
		picks, err = prompt.MultiSelect(fmt.Sprintf("Select projects (environment: %s):", *envSlug), labels)
		if err != nil {
			return err
		}
	}
	projectIDs := make([]string, len(picks))
	for i, idx := range picks {
		projectIDs[i] = qualifying[idx].ID
	}

	req, err := client.CreateAuthRequest(org.ID, projectIDs, *envSlug)
	if err != nil {
		return describeAuthRequestError(err, *envSlug)
	}

	openBrowser(req.ApprovalUrl)

	interval := time.Duration(req.PollIntervalSeconds) * time.Second
	var approved *api.AuthRequestPollResponse
	status, err := waitApproval(interval, req.ExpiresAt, func() (string, error) {
		resp, err := client.AuthRequestPoll(req.RequestCode)
		if err != nil {
			if apiErr := asAPIError(err); apiErr != nil {
				switch apiErr.Code {
				case "invalid_request_code", "already_claimed":
					return "", errors.New("this authorization request is no longer valid — run `itapu init` again")
				}
			}
			return "", mapAuthError(err, "itapu login")
		}
		if resp.Status == "approved" {
			approved = resp
		}
		return resp.Status, nil
	})
	if err != nil {
		return err
	}

	switch status {
	case "approved":
		// Token goes to the user-level store only.
		cfg.SecretsToken = approved.SecretsToken
		cfg.SecretsTokenExpiresAt = approved.ExpiresAt
		if err := config.SaveUser(cfg); err != nil {
			return fmt.Errorf("authorized, but failed to save the secrets token: %w", err)
		}

		// .itapu.json holds only ids/names — safe to commit.
		proj := &config.ProjectConfig{OrgID: org.ID, EnvironmentSlug: *envSlug}
		for _, g := range approved.Grants {
			proj.Projects = append(proj.Projects, config.ProjectGrant{
				ProjectID:       g.ProjectID,
				ProjectName:     g.ProjectName,
				EnvironmentID:   g.EnvironmentID,
				EnvironmentName: g.EnvironmentName,
			})
		}
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		path, err := config.SaveProject(cwd, proj)
		if err != nil {
			return err
		}

		info("\n" + ui.Success("Authorized. Wrote "+path))
		for _, g := range approved.Grants {
			info(ui.Grant(g.ProjectName, fmt.Sprintf("%s (%s)", g.EnvironmentName, g.EnvironmentSlug)))
		}
		info("\n" + ui.Faint(fmt.Sprintf("Secrets token valid until %s. Note: this revoked any previous",
			approved.ExpiresAt.Local().Format("Mon, 02 Jan 2006 15:04"))))
		info(ui.Faint("secrets token of yours (other repos may need `itapu init` again)."))
		info("Run " + ui.Strong("itapu run -- <command>") + " to start your app with secrets injected.")
		return nil
	case "denied":
		return errors.New("authorization denied in the browser (or your access to a requested environment was revoked)")
	case "expired":
		return errors.New("authorization request expired — run `itapu init` again")
	default:
		return fmt.Errorf("unexpected status %q", status)
	}
}

// describeAuthRequestError turns auth-request conflicts into actionable
// messages, naming the offending projects from `details`.
func describeAuthRequestError(err error, envSlug string) error {
	apiErr := asAPIError(err)
	if apiErr == nil {
		return err
	}
	names := func() string {
		var d api.ProjectDetails
		if len(apiErr.Details) == 0 || json.Unmarshal(apiErr.Details, &d) != nil {
			return ""
		}
		var out []string
		for _, p := range d.Projects {
			out = append(out, p.Name)
		}
		return strings.Join(out, ", ")
	}
	switch apiErr.Code {
	case "organization_access_denied":
		return errors.New("you are no longer a member of this organization — run `itapu init` again")
	case "project_not_found":
		return errors.New("some selected projects no longer exist — run `itapu init` again")
	case "environment_not_found":
		return fmt.Errorf("no %q environment in: %s", envSlug, names())
	case "environment_access_denied":
		return fmt.Errorf("you don't have read access to the %q environment of: %s", envSlug, names())
	}
	return mapAuthError(err, "itapu login")
}
