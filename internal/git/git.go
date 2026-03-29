package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// IsGitRepository checks if the current directory is a git repository
func IsGitRepository() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	err := cmd.Run()
	return err == nil
}

// IsClaspProject checks if .clasp.json exists
func IsClaspProject() bool {
	_, err := os.Stat(".clasp.json")
	return err == nil
}

// IsWranglerProject checks if wrangler.toml exists
func IsWranglerProject() bool {
	_, err := os.Stat("wrangler.toml")
	return err == nil
}

// lockFiles lists filenames whose contents add noise to AI-generated diffs.
// These files are still committed normally — they are only excluded from the
// diff sent to the AI and the --export output.
var lockFiles = []string{
	"package-lock.json",
	"yarn.lock",
	"pnpm-lock.yaml",
	"go.sum",
	"Gemfile.lock",
	"poetry.lock",
	"Cargo.lock",
	"composer.lock",
}

type diffSection struct {
	label   string
	content string
}

// GetFullDiff gets the complete diff including tracked and untracked files
func GetFullDiff() (string, error) {
	var sections []diffSection

	// Tracked modifications (exclude deletions)
	excludeArgs := []string{"diff", "HEAD", "--diff-filter=d", "--"}
	for _, lf := range lockFiles {
		excludeArgs = append(excludeArgs, ":!"+lf)
	}
	trackedCmd := exec.Command("git", excludeArgs...)
	trackedOutput, err := trackedCmd.Output()
	if err != nil {
		// No HEAD yet - try --cached
		cachedArgs := []string{"diff", "--cached", "--diff-filter=d", "--"}
		for _, lf := range lockFiles {
			cachedArgs = append(cachedArgs, ":!"+lf)
		}
		trackedCmd = exec.Command("git", cachedArgs...)
		trackedOutput, _ = trackedCmd.Output()
	}

	// Split tracked output into per-file sections
	if len(trackedOutput) > 0 {
		for _, chunk := range splitDiffByFile(string(trackedOutput)) {
			sections = append(sections, diffSection{
				label:   "MODIFIED",
				content: chunk,
			})
		}
	}

	// Deleted files - names only, no content
	deletedCmd := exec.Command("git", "diff", "HEAD", "--diff-filter=D", "--name-only")
	deletedOutput, err := deletedCmd.Output()
	if err != nil {
		deletedCmd = exec.Command("git", "diff", "--cached", "--diff-filter=D", "--name-only")
		deletedOutput, _ = deletedCmd.Output()
	}
	if len(deletedOutput) > 0 {
		for _, name := range strings.Split(strings.TrimSpace(string(deletedOutput)), "\n") {
			if name != "" {
				sections = append(sections, diffSection{
					label:   "DELETED",
					content: fmt.Sprintf("--- Deleted file: %s ---\n", name),
				})
			}
		}
	}

	// Untracked (new) files
	untrackedCmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	untrackedOutput, err := untrackedCmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get untracked files: %w", err)
	}

	if len(untrackedOutput) > 0 {
		untrackedFiles := strings.Split(strings.TrimSpace(string(untrackedOutput)), "\n")

		for _, file := range untrackedFiles {
			if file == "" || isLockFile(file) {
				continue
			}

			var buf strings.Builder
			buf.WriteString(fmt.Sprintf("--- New file: %s ---\n", file))
			content, err := os.ReadFile(file)
			if err != nil {
				buf.WriteString(fmt.Sprintf("[Could not read file: %v]\n", err))
			} else {
				for _, line := range strings.Split(string(content), "\n") {
					buf.WriteString("+" + line + "\n")
				}
			}
			sections = append(sections, diffSection{
				label:   "NEW",
				content: buf.String(),
			})
		}
	}

	// Sort smallest first so truncation preserves the broadest picture
	sort.Slice(sections, func(i, j int) bool {
		return len(sections[i].content) < len(sections[j].content)
	})

	var fullDiff strings.Builder
	for _, s := range sections {
		fullDiff.WriteString(s.content)
		fullDiff.WriteString("\n")
	}

	return fullDiff.String(), nil
}

// splitDiffByFile splits a unified diff into per-file chunks on "diff --git" boundaries.
func splitDiffByFile(raw string) []string {
	const marker = "diff --git "
	var chunks []string
	parts := strings.Split(raw, marker)
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		chunks = append(chunks, marker+p+"\n")
	}
	return chunks
}

func isLockFile(path string) bool {
	base := filepath.Base(path)
	for _, lf := range lockFiles {
		if base == lf {
			return true
		}
	}
	return false
}

// StageAll stages all changes
func StageAll() error {
	cmd := exec.Command("git", "add", ".")
	return cmd.Run()
}

// Commit creates a commit with the given message
func Commit(message string) error {
	tmpFile, err := os.CreateTemp("", "aicommit-message-*.txt")
	if err != nil {
		return fmt.Errorf("failed to create temp commit message file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if err := os.WriteFile(tmpFile.Name(), []byte(message), 0600); err != nil {
		return fmt.Errorf("failed to write temp commit message file: %w", err)
	}

	cmd := exec.Command("git", "commit", "-F", tmpFile.Name())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git commit failed: %v - %s", err, stderr.String())
	}
	return nil
}

// Push pushes to the remote repository
func Push() error {
	cmd := exec.Command("git", "push")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git push failed: %v - %s", err, stderr.String())
	}
	return nil
}

// ClaspPush pushes to clasp
func ClaspPush() error {
	// Check if clasp is installed
	if _, err := exec.LookPath("clasp"); err != nil {
		return fmt.Errorf("clasp is not installed")
	}

	cmd := exec.Command("clasp", "push")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("clasp push failed: %v - %s", err, stderr.String())
	}
	return nil
}

// WranglerDeploy deploys to Cloudflare Workers
func WranglerDeploy() error {
	// Check if wrangler is installed
	if _, err := exec.LookPath("wrangler"); err != nil {
		return fmt.Errorf("wrangler is not installed")
	}

	cmd := exec.Command("wrangler", "deploy")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wrangler deploy failed: %v - %s", err, stderr.String())
	}
	return nil
}

// GetLastCommit returns the last commit message
func GetLastCommit() (string, error) {
	cmd := exec.Command("git", "log", "-1", "--oneline")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetCurrentBranch returns the current branch name
func GetCurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// HasRemote checks if a remote is configured
func HasRemote() bool {
	cmd := exec.Command("git", "remote")
	output, _ := cmd.Output()
	return len(output) > 0
}

// GetRepoRoot returns the root directory of the git repository
func GetRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// IsClean checks if the working directory is clean
func IsClean() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	output, _ := cmd.Output()
	return len(output) == 0
}

// GetStatus returns the git status output
func GetStatus() (string, error) {
	cmd := exec.Command("git", "status", "--short")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// GetStagedFiles returns a list of staged files
func GetStagedFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--cached", "--name-only")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	if len(output) == 0 {
		return []string{}, nil
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	return files, nil
}

// GetModifiedFiles returns a list of modified files
func GetModifiedFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	if len(output) == 0 {
		return []string{}, nil
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")
	return files, nil
}

// GetUntrackedFiles returns a list of untracked files
func GetUntrackedFiles() ([]string, error) {
	cmd := exec.Command("git", "ls-files", "--others", "--exclude-standard")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	if len(output) == 0 {
		return []string{}, nil
	}

	files := strings.Split(strings.TrimSpace(string(output)), "\n")

	// Filter out empty strings
	var result []string
	for _, file := range files {
		if file != "" {
			result = append(result, file)
		}
	}

	return result, nil
}

// GetFileExtensions returns unique file extensions from changed files
func GetFileExtensions() ([]string, error) {
	extensions := make(map[string]bool)

	// Get all changed files
	modified, _ := GetModifiedFiles()
	untracked, _ := GetUntrackedFiles()

	allFiles := append(modified, untracked...)

	for _, file := range allFiles {
		ext := filepath.Ext(file)
		if ext != "" {
			extensions[ext] = true
		}
	}

	// Convert map to slice
	var result []string
	for ext := range extensions {
		result = append(result, ext)
	}

	return result, nil
}
