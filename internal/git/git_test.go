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
	if !strings.Contains(diff, "diff --git a/tracked.txt b/tracked.txt") {
		t.Fatal("expected tracked file diff to be included")
	}
	if !strings.Contains(diff, "--- New file: new.txt ---") {
		t.Fatal("expected new file section")
	}
}

func TestGetFullDiffShowsDeletedFilesByNameAndSortsSmallestFirst(t *testing.T) {
	dir := createRepo(t)
	withRepoCWD(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "delete-me.txt"), []byte("to be deleted\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	runGit(t, dir, "add", "delete-me.txt")
	runGit(t, dir, "commit", "-m", "add delete target")

	if err := os.Remove(filepath.Join(dir, "delete-me.txt")); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("hello\nworld\nwith more lines\nfor a larger diff\nand even more context\nplus another line\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("fresh\nfile\nwith\nsome\ncontent\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	diff, err := git.GetFullDiff()
	if err != nil {
		t.Fatalf("GetFullDiff failed: %v", err)
	}

	deletedEntry := "--- Deleted file: delete-me.txt ---"
	newEntry := "--- New file: new.txt ---"
	modifiedEntry := "diff --git a/tracked.txt b/tracked.txt"

	if !strings.Contains(diff, deletedEntry) {
		t.Fatal("expected deleted file entry to be included")
	}
	if strings.Contains(diff, "deleted file mode") {
		t.Fatal("expected deleted files to be represented without full diff content")
	}
	if strings.Contains(diff, "-to be deleted") {
		t.Fatal("expected deleted file contents to be excluded")
	}

	deletedIndex := strings.Index(diff, deletedEntry)
	newIndex := strings.Index(diff, newEntry)
	modifiedIndex := strings.Index(diff, modifiedEntry)
	if deletedIndex == -1 || newIndex == -1 || modifiedIndex == -1 {
		t.Fatal("expected deleted, new, and modified sections to all be present")
	}
	if !(deletedIndex < newIndex && newIndex < modifiedIndex) {
		t.Fatal("expected sections to be ordered from smallest to largest")
	}
}

func TestGetFullDiffSkipsLockFiles(t *testing.T) {
	dir := createRepo(t)
	withRepoCWD(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("hello\nworld\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), []byte("module checksum\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("fresh\nfile\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package-lock.json"), []byte("{\"lock\":true}\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	diff, err := git.GetFullDiff()
	if err != nil {
		t.Fatalf("GetFullDiff failed: %v", err)
	}

	if !strings.Contains(diff, "tracked.txt") {
		t.Fatal("expected tracked non-lock file diff to be included")
	}
	if !strings.Contains(diff, "--- New file: new.txt ---") {
		t.Fatal("expected untracked non-lock file to be included")
	}
	if strings.Contains(diff, "go.sum") {
		t.Fatal("expected tracked lock file diff to be excluded")
	}
	if strings.Contains(diff, "package-lock.json") {
		t.Fatal("expected untracked lock file content to be excluded")
	}
	if strings.Contains(diff, "module checksum") {
		t.Fatal("expected tracked lock file content to be excluded")
	}
	if strings.Contains(diff, "{\"lock\":true}") {
		t.Fatal("expected untracked lock file content to be excluded")
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
