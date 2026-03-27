package prompt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/SCHW-AI/aicommit/internal/llm"
	"github.com/manifoldco/promptui"
)

// Confirm asks for confirmation
func Confirm(message string, defaultValue bool) (bool, error) {
	defaultText := "n"
	if defaultValue {
		defaultText = "y"
	}

	prompt := promptui.Prompt{
		Label:     message,
		IsConfirm: true,
		Default:   defaultText,
	}

	_, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrAbort {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// Select provides a selection menu
func Select(message string, options []string) (string, error) {
	prompt := promptui.Select{
		Label: message,
		Items: options,
		Size:  len(options),
	}

	_, result, err := prompt.Run()
	return result, err
}

// Input prompts for text input
func Input(message, defaultValue string) (string, error) {
	prompt := promptui.Prompt{
		Label:   message,
		Default: defaultValue,
	}

	return prompt.Run()
}

// EditCommitMessage opens an editor for editing the commit message
func EditCommitMessage(current *llm.CommitMessage) (*llm.CommitMessage, error) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "commit-*.txt")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write current message to temp file
	content := fmt.Sprintf("HEADER: %s\n\nDESCRIPTION: %s\n", current.Header, current.Description)
	content += "\n# Please edit the commit message above.\n"
	content += "# Lines starting with '#' will be ignored.\n"
	content += "# The first line should be the header (max 72 chars).\n"
	content += "# The description can be multiple lines.\n"

	if err := os.WriteFile(tmpFile.Name(), []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write temp file: %w", err)
	}

	// Open editor
	editor := getEditor()
	cmd := exec.Command(editor, tmpFile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to open editor: %w", err)
	}

	// Read edited content
	editedContent, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read edited file: %w", err)
	}

	// Parse edited content
	lines := strings.Split(string(editedContent), "\n")
	var header, description string
	var descLines []string
	inDescription := false

	for _, line := range lines {
		// Skip comments
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}

		if strings.HasPrefix(line, "HEADER:") {
			header = strings.TrimSpace(strings.TrimPrefix(line, "HEADER:"))
			continue
		}

		if strings.HasPrefix(line, "DESCRIPTION:") {
			description = strings.TrimSpace(strings.TrimPrefix(line, "DESCRIPTION:"))
			inDescription = true
			continue
		}

		// If we're in description section, collect lines
		if inDescription && strings.TrimSpace(line) != "" {
			descLines = append(descLines, line)
		}
	}

	// Join description lines if there are multiple
	if len(descLines) > 0 {
		if description != "" {
			description = description + "\n" + strings.Join(descLines, "\n")
		} else {
			description = strings.Join(descLines, "\n")
		}
	}

	// If no header found, use the original
	if header == "" {
		header = current.Header
	}

	return &llm.CommitMessage{
		Header:      header,
		Description: description,
	}, nil
}

// getEditor returns the default text editor
func getEditor() string {
	// Check environment variables
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}
	if editor := os.Getenv("VISUAL"); editor != "" {
		return editor
	}

	// Platform defaults
	switch runtime.GOOS {
	case "windows":
		// Try to find a suitable editor on Windows
		editors := []string{"code", "notepad++", "notepad"}
		for _, editor := range editors {
			if _, err := exec.LookPath(editor); err == nil {
				return editor
			}
		}
		return "notepad"
	case "darwin":
		// macOS
		if _, err := exec.LookPath("code"); err == nil {
			return "code"
		}
		return "nano"
	default:
		// Linux and others
		editors := []string{"vim", "nano", "vi"}
		for _, editor := range editors {
			if _, err := exec.LookPath(editor); err == nil {
				return editor
			}
		}
		return "vi"
	}
}

// ShowProgress shows a progress spinner
func ShowProgress(message string) func() {
	// Simple implementation - in production you might want to use a proper spinner library
	fmt.Printf("%s...", message)
	return func() {
		fmt.Println(" Done!")
	}
}

// MultiInput prompts for multiple lines of input
func MultiInput(message string) ([]string, error) {
	fmt.Println(message)
	fmt.Println("(Enter an empty line to finish)")

	var lines []string
	for {
		prompt := promptui.Prompt{
			Label: ">",
		}

		line, err := prompt.Run()
		if err != nil {
			return lines, err
		}

		if line == "" {
			break
		}

		lines = append(lines, line)
	}

	return lines, nil
}

// Password prompts for a password (hidden input)
func Password(message string) (string, error) {
	prompt := promptui.Prompt{
		Label: message,
		Mask:  '*',
	}

	return prompt.Run()
}

// GetTempDir returns a temporary directory for the application
func GetTempDir() (string, error) {
	dir := filepath.Join(os.TempDir(), "aicommit")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}
