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

// Init implements `itapu init [--env=<slug>]`: links this folder to one
// project and a single environment. The browser approval mints an 8-hour
// secrets token; a still-valid previous token is sent along so the server
// carries its grants into the new one (and revokes it), meaning initing
// project B never breaks a folder linked to project A. When the current
// token already covers the requested grant, no approval is needed and only
// .itapu.json is (re)written.
//
// When the folder already has a .itapu.json, its org/project pin is reused
// and the selection prompts are skipped (e.g. re-initing after the token
// expired); the prompts return if the pin no longer matches what the user
// can access, or if the file lists several projects.
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

	// A .itapu.json already in this folder pins the org/project; a corrupt
	// file only costs the shortcut, since init rewrites it anyway.
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	pinned, err := config.LoadProjectDir(cwd)
	if err != nil {
		info(ui.Warn(fmt.Sprintf("Ignoring existing %s: %v", config.ProjectConfigName, err)))
		pinned = nil
	}

	// Pick the organization: the pinned one when still accessible, otherwise
	// prompt (skipping the prompt when there is only one).
	var org *api.Organization
	if pinned != nil {
		for i := range ws.Organizations {
			if ws.Organizations[i].ID == pinned.OrgID {
				org = &ws.Organizations[i]
				break
			}
		}
		if org == nil {
			info(ui.Warn(fmt.Sprintf("The organization pinned in %s is not one you belong to — pick again.",
				config.ProjectConfigName)))
		} else {
			info("Organization: %s %s", ui.Strong(org.Name),
				ui.Faint("("+org.Role+", from "+config.ProjectConfigName+")"))
		}
	}
	if org == nil && len(ws.Organizations) == 1 {
		org = &ws.Organizations[0]
		info("Organization: %s %s", ui.Strong(org.Name), ui.Faint("("+org.Role+")"))
	}
	if org == nil {
		labels := make([]string, len(ws.Organizations))
		for i, o := range ws.Organizations {
			labels[i] = fmt.Sprintf("%s (%s)", o.Name, o.Role)
		}
		idx, err := prompt.Select("Select an organization:", labels)
		if err != nil {
			return err
		}
		org = &ws.Organizations[idx]
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

	// Pick one project: the pinned one when it still qualifies, otherwise
	// prompt (skipping the prompt when there is only one). A multi-project
	// .itapu.json never auto-picks — init links a single project, and
	// silently narrowing the file down to one would break `itapu run` for
	// the others.
	pick := -1
	if pinned != nil && pinned.OrgID == org.ID && len(pinned.Projects) == 1 {
		for i := range qualifying {
			if qualifying[i].ID == pinned.Projects[0].ProjectID {
				pick = i
				break
			}
		}
		if pick >= 0 {
			info("Project: %s %s", ui.Strong(qualifying[pick].Name),
				ui.Faint("(from "+config.ProjectConfigName+")"))
		} else {
			info(ui.Warn(fmt.Sprintf("The project pinned in %s has no %q environment you can read — pick again.",
				config.ProjectConfigName, *envSlug)))
		}
	}
	if pick < 0 && len(qualifying) == 1 {
		pick = 0
		info("Project: %s", ui.Strong(qualifying[0].Name))
	}
	if pick < 0 {
		labels := make([]string, len(qualifying))
		for i, p := range qualifying {
			labels[i] = p.Name
		}
		pick, err = prompt.Select(fmt.Sprintf("Select a project (environment: %s):", *envSlug), labels)
		if err != nil {
			return err
		}
	}
	project := &qualifying[pick]

	// Already granted? Then the browser step would approve nothing new:
	// just link this folder to the existing token.
	if hasValidSecretsToken(cfg) {
		for _, g := range cfg.SecretsTokenGrants {
			if g.OrgID == org.ID && g.ProjectID == project.ID && g.EnvironmentSlug == *envSlug {
				info("\n" + ui.Success("Your current secrets token already covers this project — no approval needed."))
				if _, err := linkProject(org.ID, *envSlug, g); err != nil {
					return err
				}
				info("\n" + ui.Faint(fmt.Sprintf("Secrets token valid until %s.",
					cfg.SecretsTokenExpiresAt.Local().Format("Mon, 02 Jan 2006 15:04"))))
				info("Run " + ui.Strong("itapu run -- <command>") + " to start your app with secrets injected.")
				return nil
			}
		}
	}

	// A still-valid token is sent along so its grants carry over into the
	// new token; an expired one is not worth extending.
	extendToken := ""
	if hasValidSecretsToken(cfg) {
		extendToken = cfg.SecretsToken
	}

	req, err := client.CreateAuthRequest(org.ID, project.ID, *envSlug, extendToken)
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
		// Token and its full grant list go to the user-level store only.
		grants := make([]config.TokenGrant, len(approved.Grants))
		for i, g := range approved.Grants {
			orgID := g.OrgID
			if orgID == "" {
				orgID = org.ID
			}
			grants[i] = config.TokenGrant{
				OrgID:           orgID,
				ProjectID:       g.ProjectID,
				ProjectName:     g.ProjectName,
				EnvironmentID:   g.EnvironmentID,
				EnvironmentSlug: g.EnvironmentSlug,
				EnvironmentName: g.EnvironmentName,
			}
		}
		cfg.SecretsToken = approved.SecretsToken
		cfg.SecretsTokenExpiresAt = approved.ExpiresAt
		cfg.SecretsTokenGrants = grants
		if err := config.SaveUser(cfg); err != nil {
			return fmt.Errorf("authorized, but failed to save the secrets token: %w", err)
		}

		var linked *config.TokenGrant
		for i := range grants {
			if grants[i].ProjectID == project.ID && grants[i].EnvironmentSlug == *envSlug {
				linked = &grants[i]
				break
			}
		}
		if linked == nil {
			return fmt.Errorf("the approved token does not cover %s (%q) — run `itapu init` again", project.Name, *envSlug)
		}

		info("\n" + ui.Success("Authorized."))
		if len(grants) > 1 {
			info("Your secrets token now covers:")
			for _, g := range grants {
				info(ui.Grant(g.ProjectName, fmt.Sprintf("%s (%s)", g.EnvironmentName, g.EnvironmentSlug)))
			}
		}
		if _, err := linkProject(org.ID, *envSlug, *linked); err != nil {
			return err
		}
		expiry := approved.ExpiresAt.Local().Format("Mon, 02 Jan 2006 15:04")
		if extendToken != "" {
			info("\n" + ui.Faint(fmt.Sprintf("Secrets token valid until %s. It replaced your previous token,", expiry)))
			info(ui.Faint("carrying its grants over — folders linked to them keep working."))
		} else {
			info("\n" + ui.Faint(fmt.Sprintf("Secrets token valid until %s. Note: this revoked any previous", expiry)))
			info(ui.Faint("secrets token of yours."))
		}
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

// hasValidSecretsToken reports whether the stored secrets token exists and
// has not expired.
func hasValidSecretsToken(cfg *config.UserConfig) bool {
	return cfg.SecretsToken != "" &&
		(cfg.SecretsTokenExpiresAt.IsZero() || time.Now().Before(cfg.SecretsTokenExpiresAt))
}

// linkProject writes .itapu.json in the current directory pointing at the
// grant (ids/names only, no tokens; per-developer, hence gitignored) and
// reports both to the user.
func linkProject(orgID, envSlug string, g config.TokenGrant) (string, error) {
	proj := &config.ProjectConfig{
		OrgID:           orgID,
		EnvironmentSlug: envSlug,
		Projects: []config.ProjectGrant{{
			ProjectID:       g.ProjectID,
			ProjectName:     g.ProjectName,
			EnvironmentID:   g.EnvironmentID,
			EnvironmentName: g.EnvironmentName,
		}},
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	path, err := config.SaveProject(cwd, proj)
	if err != nil {
		return "", err
	}
	info(ui.Success("Wrote " + path))
	info(ui.Grant(g.ProjectName, fmt.Sprintf("%s (%s)", g.EnvironmentName, g.EnvironmentSlug)))
	switch added, err := ensureGitignored(cwd); {
	case err != nil:
		info(ui.Warn(fmt.Sprintf("Couldn't update .gitignore (%v) — add %s to it yourself; the file is per-developer.",
			err, config.ProjectConfigName)))
	case added:
		info("Added " + ui.Strong(config.ProjectConfigName) + " to .gitignore " + ui.Faint("(per-developer file)"))
	}
	return path, nil
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
