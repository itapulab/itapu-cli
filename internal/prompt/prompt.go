// Package prompt provides the CLI's interactive prompts: arrow-key
// navigation via huh in a terminal, with a plain numbered-list fallback
// for non-interactive stdin (pipes, CI).
package prompt

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"

	"github.com/itapulab/itapu-cli/internal/ui"
)

// ErrCancelled is returned when the user aborts a prompt (Ctrl-C / Esc).
var ErrCancelled = errors.New("cancelled")

// Select shows an arrow-key list and returns the chosen index.
func Select(label string, options []string) (int, error) {
	if !ui.Interactive() {
		return selectFallback(label, options)
	}
	var idx int
	opts := make([]huh.Option[int], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o, i)
	}
	err := huh.NewSelect[int]().
		Title(label).
		Options(opts...).
		Value(&idx).
		Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return 0, ErrCancelled
		}
		return 0, err
	}
	return idx, nil
}

// MultiSelect shows a checklist (space toggles, enter confirms) and
// returns the chosen indexes. At least one selection is required.
func MultiSelect(label string, options []string) ([]int, error) {
	if !ui.Interactive() {
		return multiSelectFallback(label, options)
	}
	var picks []int
	opts := make([]huh.Option[int], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o, i)
	}
	err := huh.NewMultiSelect[int]().
		Title(label).
		Description("space to toggle, enter to confirm").
		Options(opts...).
		Validate(func(v []int) error {
			if len(v) == 0 {
				return errors.New("select at least one project")
			}
			return nil
		}).
		Value(&picks).
		Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return nil, ErrCancelled
		}
		return nil, err
	}
	return picks, nil
}

// ---- non-TTY fallback ----

var stdin = bufio.NewReader(os.Stdin)

func readLine() (string, error) {
	line, err := stdin.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("input closed")
	}
	return strings.TrimSpace(line), nil
}

func selectFallback(label string, options []string) (int, error) {
	fmt.Fprintf(os.Stderr, "\n%s\n", label)
	for i, opt := range options {
		fmt.Fprintf(os.Stderr, "  %2d) %s\n", i+1, opt)
	}
	for {
		fmt.Fprintf(os.Stderr, "Enter a number (1-%d): ", len(options))
		line, err := readLine()
		if err != nil {
			return 0, err
		}
		n, err := strconv.Atoi(line)
		if err == nil && n >= 1 && n <= len(options) {
			return n - 1, nil
		}
		fmt.Fprintln(os.Stderr, "Invalid choice.")
	}
}

func multiSelectFallback(label string, options []string) ([]int, error) {
	fmt.Fprintf(os.Stderr, "\n%s\n", label)
	for i, opt := range options {
		fmt.Fprintf(os.Stderr, "  %2d) %s\n", i+1, opt)
	}
	for {
		fmt.Fprintf(os.Stderr, "Enter numbers separated by commas, or 'all': ")
		line, err := readLine()
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(line, "all") || strings.EqualFold(line, "a") {
			all := make([]int, len(options))
			for i := range options {
				all[i] = i
			}
			return all, nil
		}
		fields := strings.FieldsFunc(line, func(r rune) bool { return r == ',' || r == ' ' })
		seen := map[int]bool{}
		var picks []int
		valid := len(fields) > 0
		for _, f := range fields {
			n, err := strconv.Atoi(f)
			if err != nil || n < 1 || n > len(options) {
				valid = false
				break
			}
			if !seen[n-1] {
				seen[n-1] = true
				picks = append(picks, n-1)
			}
		}
		if valid {
			return picks, nil
		}
		fmt.Fprintln(os.Stderr, "Invalid selection.")
	}
}
