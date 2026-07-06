package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/itapulab/itapu-cli/internal/config"
)

func gitInit(t *testing.T, dir string) {
	t.Helper()
	if err := exec.Command("git", "init", "-q", dir).Run(); err != nil {
		t.Skipf("git unavailable: %v", err)
	}
}

func readGitignore(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}
	return string(data)
}

func TestEnsureGitignoredOutsideRepo(t *testing.T) {
	dir := t.TempDir()
	added, err := ensureGitignored(dir)
	if err != nil || added {
		t.Fatalf("want no-op outside a repo, got added=%v err=%v", added, err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); !os.IsNotExist(err) {
		t.Fatal("no .gitignore should be created outside a repo")
	}
}

func TestEnsureGitignoredCreatesAndAppends(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)

	added, err := ensureGitignored(dir)
	if err != nil || !added {
		t.Fatalf("want entry added in fresh repo, got added=%v err=%v", added, err)
	}
	if got, want := readGitignore(t, dir), config.ProjectConfigName+"\n"; got != want {
		t.Fatalf(".gitignore = %q, want %q", got, want)
	}

	// Second run must be idempotent.
	added, err = ensureGitignored(dir)
	if err != nil || added {
		t.Fatalf("want no-op on second run, got added=%v err=%v", added, err)
	}
}

func TestEnsureGitignoredAddsNewlineFirst(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules"), 0o644); err != nil {
		t.Fatal(err)
	}

	if added, err := ensureGitignored(dir); err != nil || !added {
		t.Fatalf("want entry added, got added=%v err=%v", added, err)
	}
	if got, want := readGitignore(t, dir), "node_modules\n"+config.ProjectConfigName+"\n"; got != want {
		t.Fatalf(".gitignore = %q, want %q", got, want)
	}
}

func TestEnsureGitignoredRespectsExistingPattern(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	// A glob that only `git check-ignore` (not the exact-line fallback)
	// recognizes as covering .itapu.json.
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(".itapu.*\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if added, err := ensureGitignored(dir); err != nil || added {
		t.Fatalf("want no-op when a pattern already ignores it, got added=%v err=%v", added, err)
	}
	if got, want := readGitignore(t, dir), ".itapu.*\n"; got != want {
		t.Fatalf(".gitignore = %q, want %q", got, want)
	}
}

func TestEnsureGitignoredInSubdirectory(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	sub := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	if added, err := ensureGitignored(sub); err != nil || !added {
		t.Fatalf("want entry added in subdir of repo, got added=%v err=%v", added, err)
	}
	if got, want := readGitignore(t, sub), config.ProjectConfigName+"\n"; got != want {
		t.Fatalf("apps/web/.gitignore = %q, want %q", got, want)
	}
}
