// Package prompt provides minimal interactive terminal prompts.
package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var stdin = bufio.NewReader(os.Stdin)

func readLine() (string, error) {
	line, err := stdin.ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("input closed")
	}
	return strings.TrimSpace(line), nil
}

// Select prints a numbered list and returns the chosen index.
func Select(label string, options []string) (int, error) {
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

// MultiSelect prints a numbered list and returns the chosen indexes.
// Accepts comma/space separated numbers or "all".
func MultiSelect(label string, options []string) ([]int, error) {
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
