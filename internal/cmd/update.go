package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/charmbracelet/huh/spinner"

	"github.com/itapulab/itapu-cli/internal/ui"
	"github.com/itapulab/itapu-cli/internal/update"
)

// Update implements `itapu update`: replace this binary with the latest
// GitHub release (resolved, downloaded and checksum-verified by the update
// package).
func Update(current string, args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	tag, err := update.LatestTag(nil)
	if err != nil {
		return err
	}

	newer, comparable := update.Newer(tag, current)
	switch {
	case comparable && !newer:
		info(ui.Success(fmt.Sprintf("itapu %s is already the latest version.", update.DisplayVersion(current))))
		return nil
	case !comparable:
		// e.g. a `make install` dev build; the user explicitly asked for the
		// latest release, so proceed but say what is being replaced.
		info(ui.Warn(fmt.Sprintf("You are on a development build (%s); installing %s.", current, tag)))
	}

	apply := func() error { return update.Apply(tag) }
	if ui.Interactive() {
		var applyErr error
		title := fmt.Sprintf("Downloading itapu %s...", tag)
		if err := spinner.New().Title(title).Output(os.Stderr).Action(func() {
			applyErr = apply()
		}).Run(); err != nil {
			return err
		}
		err = applyErr
	} else {
		info("Downloading itapu %s...", tag)
		err = apply()
	}
	if err != nil {
		return err
	}

	if comparable {
		info(ui.Success(fmt.Sprintf("Updated itapu %s → %s.", update.DisplayVersion(current), tag)))
	} else {
		info(ui.Success(fmt.Sprintf("Installed itapu %s.", tag)))
	}
	return nil
}
