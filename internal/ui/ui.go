// Package ui centralizes the CLI's visual style (Lip Gloss) so every
// command renders consistently in light and dark terminals.
package ui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

var (
	accent  = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"}
	green   = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"}
	yellow  = lipgloss.AdaptiveColor{Light: "#A16207", Dark: "#FACC15"}
	red     = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}
	subtle  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	strongC = lipgloss.AdaptiveColor{Light: "#111827", Dark: "#F9FAFB"}

	successStyle = lipgloss.NewStyle().Foreground(green).Bold(true)
	warnStyle    = lipgloss.NewStyle().Foreground(yellow)
	errorStyle   = lipgloss.NewStyle().Foreground(red).Bold(true)
	faintStyle   = lipgloss.NewStyle().Foreground(subtle)
	urlStyle     = lipgloss.NewStyle().Foreground(accent).Underline(true)
	strongStyle  = lipgloss.NewStyle().Foreground(strongC).Bold(true)
	codeBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accent).
			Foreground(accent).
			Bold(true).
			Padding(0, 3).
			MarginLeft(2)
)

// Interactive reports whether we can drive interactive prompts/spinners.
func Interactive() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd()))
}

func Success(s string) string { return successStyle.Render("✔ " + s) }
func Warn(s string) string    { return warnStyle.Render("⚠ " + s) }
func Error(s string) string   { return errorStyle.Render("✖ " + s) }
func Faint(s string) string   { return faintStyle.Render(s) }
func URL(s string) string     { return urlStyle.Render(s) }
func Strong(s string) string  { return strongStyle.Render(s) }

// CodeBox renders the login verification code in a bordered box.
func CodeBox(code string) string { return codeBoxStyle.Render(code) }

// Grant renders one "project → environment" summary line.
func Grant(project, env string) string {
	return "  " + strongStyle.Render(project) + faintStyle.Render(" → ") + env
}
