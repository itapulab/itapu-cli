package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/itapulab/itapu-cli/internal/config"
)

// ensureGitignored makes sure dir's .gitignore covers the project config
// (.itapu.json is per-developer state: every `itapu init` rewrites it with
// that developer's project selection, so it must not be committed).
// It reports whether an entry was added; outside a git repository or when
// the file is already ignored it is a no-op.
func ensureGitignored(dir string) (bool, error) {
	if !insideGitRepo(dir) {
		return false, nil
	}
	if isGitIgnored(dir, config.ProjectConfigName) {
		return false, nil
	}
	if err := appendGitignoreEntry(filepath.Join(dir, ".gitignore"), config.ProjectConfigName); err != nil {
		return false, err
	}
	return true, nil
}

// insideGitRepo walks up from dir looking for a .git entry — a directory in
// normal checkouts, a file in worktrees and submodules.
func insideGitRepo(dir string) bool {
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

// isGitIgnored prefers `git check-ignore`, which understands negations,
// parent .gitignores and global excludes; when git is unavailable or says
// "not ignored", an exact line match in dir's own .gitignore still counts
// (cheap, and avoids duplicate entries if check-ignore failed for an
// unrelated reason).
func isGitIgnored(dir, name string) bool {
	if _, err := exec.LookPath("git"); err == nil {
		if exec.Command("git", "-C", dir, "check-ignore", "-q", "--", name).Run() == nil {
			return true
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == name {
			return true
		}
	}
	return false
}

// appendGitignoreEntry appends name as its own line, creating the file if
// needed and inserting a newline first when the file doesn't end with one.
func appendGitignoreEntry(path, name string) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	entry := name + "\n"
	if st, err := f.Stat(); err == nil && st.Size() > 0 {
		last := make([]byte, 1)
		if _, err := f.ReadAt(last, st.Size()-1); err == nil && last[0] != '\n' {
			entry = "\n" + entry
		}
	}
	if _, err := f.Write([]byte(entry)); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
