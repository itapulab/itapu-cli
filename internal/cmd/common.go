// Package cmd implements the itapu subcommands.
package cmd

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/itapulab/itapu-cli/internal/api"
	"github.com/itapulab/itapu-cli/internal/browser"
	"github.com/itapulab/itapu-cli/internal/config"
)

func info(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}

// openBrowser prints the URL (always, for SSH/headless use) and tries to
// open it in the default browser.
func openBrowser(url string) {
	info("\nOpen this URL in your browser:\n\n    %s\n", url)
	if err := browser.Open(url); err == nil {
		info("(a browser window should have opened automatically)")
	}
}

// waitApproval polls fn every interval until the deadline, returning the
// first non-pending status.
func waitApproval(interval time.Duration, deadline time.Time, fn func() (string, error)) (string, error) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	info("Waiting for approval (expires %s)...", deadline.Local().Format(time.Kitchen))
	for {
		if time.Now().After(deadline) {
			return "expired", nil
		}
		status, err := fn()
		if err != nil {
			return "", err
		}
		if status != "pending" {
			return status, nil
		}
		time.Sleep(interval)
	}
}

// requireAccountToken returns the stored account token or an actionable error.
func requireAccountToken(cfg *config.UserConfig) (string, error) {
	if cfg.AccountToken == "" {
		return "", errors.New("you are not logged in — run `itapu login` first")
	}
	if !cfg.AccountTokenExpiresAt.IsZero() && time.Now().After(cfg.AccountTokenExpiresAt) {
		return "", errors.New("your login has expired — run `itapu login` again")
	}
	return cfg.AccountToken, nil
}

// asAPIError extracts an *api.Error if err is one.
func asAPIError(err error) *api.Error {
	var apiErr *api.Error
	if errors.As(err, &apiErr) {
		return apiErr
	}
	return nil
}

// mapAuthError converts the 401 family into actionable messages. loginHint
// is the command to suggest ("itapu login" or "itapu init").
func mapAuthError(err error, loginHint string) error {
	if apiErr := asAPIError(err); apiErr != nil {
		switch apiErr.Code {
		case "invalid_token", "token_revoked", "token_expired":
			return fmt.Errorf("%s — run `%s` again", apiErr.Message, loginHint)
		}
	}
	return err
}
