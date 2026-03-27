package git_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SCHW-AI/aicommit/internal/git"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
	return string(output)
}

func withRepoCWD(t *testing.T, dir string) {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
}

func createRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.name", "AICommit Test")
	runGit(t, dir, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("hello\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")
	return dir
}

func TestGetFullDiffIncludesTrackedAndUntracked(t *testing.T) {
	dir := createRepo(t)
	withRepoCWD(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("hello\nworld\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("fresh\nfile\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	diff, err := git.GetFullDiff()
	if err != nil {
		t.Fatalf("GetFullDiff failed: %v", err)
	}
	if !strings.Contains(diff, "=== MODIFIED FILES ===") {
		t.Fatal("expected modified files section")
	}
	if !strings.Contains(diff, "--- New file: new.txt ---") {
		t.Fatal("expected new file section")
	}
}

func TestCommitUsesMultilineTempFile(t *testing.T) {
	dir := createRepo(t)
	withRepoCWD(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("updated\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := git.StageAll(); err != nil {
		t.Fatalf("StageAll failed: %v", err)
	}
	if err := git.Commit("Header line\n\nBody line"); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	message := runGit(t, dir, "log", "-1", "--format=%B")
	if !strings.Contains(message, "Header line") || !strings.Contains(message, "Body line") {
		t.Fatalf("unexpected git message: %q", message)
	}
}
