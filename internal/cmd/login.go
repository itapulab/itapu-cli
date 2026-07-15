package cmd

import (
	"errors"
	"flag"
	"fmt"
	"time"

	"github.com/itapulab/itapu-cli/internal/api"
	"github.com/itapulab/itapu-cli/internal/config"
	"github.com/itapulab/itapu-cli/internal/ui"
)

// Login implements `itapu login`: device-code flow that stores a 6-day
// account token in the user-level config.
func Login(args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	baseURLFlag := fs.String("base-url", "", "Itapu web app origin (default: $ITAPU_BASE_URL or "+config.DefaultBaseURL+")")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.LoadUser()
	if err != nil {
		return err
	}
	// Login establishes which server the CLI is bound to, so the stored
	// baseUrl must not be a fallback here: omitting the flag means the
	// default (or $ITAPU_BASE_URL), not whatever the last login used.
	baseURL := config.ResolveBaseURL(*baseURLFlag, nil)

	client := api.New(baseURL, "")
	start, err := client.LoginStart()
	if err != nil {
		return err
	}

	// The user compares this code with the one shown in the browser before
	// approving — display it prominently.
	info("\nYour verification code is:\n")
	info(ui.CodeBox(start.UserCode))
	info("\n" + ui.Faint("Confirm it matches the code shown in the browser before approving."))
	openBrowser(start.VerificationUrl)

	interval := time.Duration(start.PollIntervalSeconds) * time.Second
	var approved *api.LoginPollResponse
	status, err := waitApproval(interval, start.ExpiresAt, func() (string, error) {
		resp, err := client.LoginPoll(start.DeviceCode)
		if err != nil {
			if apiErr := asAPIError(err); apiErr != nil && apiErr.Code == "invalid_device_code" {
				return "", errors.New("this login request is no longer valid — run `itapu login` again")
			}
			return "", err
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
		if oldURL := config.ResolveBaseURL("", cfg); oldURL != baseURL {
			// The secrets token was minted by the previous server and is
			// invalid on the new one; drop it so `run` asks for `init` again.
			cfg.SecretsToken = ""
			cfg.SecretsTokenExpiresAt = time.Time{}
			cfg.SecretsTokenGrants = nil
		}
		cfg.BaseURL = baseURL
		cfg.AccountToken = approved.AccountToken
		cfg.AccountTokenExpiresAt = approved.ExpiresAt
		if err := config.SaveUser(cfg); err != nil {
			return fmt.Errorf("logged in, but failed to save credentials: %w", err)
		}
		info("\n" + ui.Success(fmt.Sprintf("Logged in. Your session is valid until %s.",
			approved.ExpiresAt.Local().Format("Mon, 02 Jan 2006 15:04"))))
		return nil
	case "denied":
		return errors.New("login denied in the browser")
	case "expired":
		return errors.New("login request expired — run `itapu login` again")
	default:
		return fmt.Errorf("unexpected status %q", status)
	}
}
