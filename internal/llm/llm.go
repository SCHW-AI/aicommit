package llm

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SCHW-AI/aicommit/internal/provider"
)

// CommitMessage represents a structured commit message
type CommitMessage struct {
	Header      string `json:"header"`
	Description string `json:"description"`
}

// Format returns the formatted commit message
func (c *CommitMessage) Format() string {
	if c.Description == "" {
		return c.Header
	}
	return fmt.Sprintf("%s\n\n%s", c.Header, c.Description)
}

// Client interface for LLM providers
type Client interface {
	GenerateCommitMessage(diff string) (*CommitMessage, error)
}

// NewClient creates a new LLM client for the selected provider and model.
func NewClient(providerValue provider.Provider, model, apiKey string) (Client, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required for provider %q", providerValue)
	}
	if model == "" {
		model = provider.DefaultModel(providerValue)
	}
	if err := provider.ValidateModel(providerValue, model); err != nil {
		return nil, err
	}

	switch providerValue {
	case provider.Anthropic:
		return NewAnthropicClient(apiKey, model)
	case provider.Gemini:
		return NewGeminiClient(apiKey, model)
	case provider.OpenAI:
		return NewOpenAIClient(apiKey, model)
	default:
		return nil, fmt.Errorf("unsupported provider %q", providerValue)
	}
}

// Common prompt for all providers
const commitPrompt = `Analyze this git diff and suggest a commit message.

Requirements:
- Header must be 50 characters or less
- Use imperative mood (Add, Fix, Update - NOT Added, Fixed, Updated)
- Description should explain what changed and why
- Do not use markdown, bullets, or special formatting

Diff:

%s`

// ParseResponse parses the LLM response into a CommitMessage
func ParseResponse(response string) (*CommitMessage, error) {
	lines := strings.Split(response, "\n")

	var header, description string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "HEADER:") {
			header = strings.TrimSpace(strings.TrimPrefix(line, "HEADER:"))
		} else if strings.HasPrefix(line, "DESCRIPTION:") {
			description = strings.TrimSpace(strings.TrimPrefix(line, "DESCRIPTION:"))
		}
	}

	if header == "" {
		return nil, fmt.Errorf("no header found in response")
	}

	return &CommitMessage{
		Header:      header,
		Description: description,
	}, nil
}

// parseJSONCommitMessage unmarshals a JSON string returned by providers
// that use schema-enforced structured output (Gemini, OpenAI).
func parseJSONCommitMessage(jsonText string) (*CommitMessage, error) {
	var msg CommitMessage
	if err := json.Unmarshal([]byte(jsonText), &msg); err != nil {
		return nil, fmt.Errorf("failed to parse structured response: %w", err)
	}
	if msg.Header == "" {
		return nil, fmt.Errorf("structured response missing header field")
	}
	return &msg, nil
}

// TruncateDiff truncates the diff if it's too long
func TruncateDiff(diff string, maxLength int) string {
	if len(diff) <= maxLength {
		return diff
	}
	return diff[:maxLength] + "\n... (diff truncated)"
}

// GenerateWithRetry calls GenerateCommitMessage and retries if the header exceeds 50 characters.
func GenerateWithRetry(client Client, diff string, maxRetries int) (*CommitMessage, error) {
	msg, err := client.GenerateCommitMessage(diff)
	if err != nil {
		return nil, err
	}

	for attempt := 0; attempt < maxRetries && len(msg.Header) > 50; attempt++ {
		retryDiff := fmt.Sprintf(
			"RETRY: Your previous header was %d characters: %q. It MUST be 50 characters or fewer. Shorten it.\n\n%s",
			len(msg.Header), msg.Header, diff,
		)
		msg, err = client.GenerateCommitMessage(retryDiff)
		if err != nil {
			return nil, err
		}
	}

	if len(msg.Header) > 50 {
		msg.Header = msg.Header[:50]
	}

	return msg, nil
}
